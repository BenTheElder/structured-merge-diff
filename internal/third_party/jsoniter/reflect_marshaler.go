package jsoniter

import (
	"encoding"
	"encoding/json"
	"reflect"
	"unsafe"
)

var marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
var unmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func createDecoderOfMarshaler(ctx *ctx, typ reflect.Type) ValDecoder {
	ptrType := reflect.PointerTo(typ)
	if ptrType.Implements(unmarshalerType) {
		return &referenceDecoder{
			&unmarshalerDecoder{ptrType},
		}
	}
	if ptrType.Implements(textUnmarshalerType) {
		return &referenceDecoder{
			&textUnmarshalerDecoder{ptrType},
		}
	}
	return nil
}

func createEncoderOfMarshaler(ctx *ctx, typ reflect.Type) ValEncoder {
	if typ == marshalerType {
		checkIsEmpty := createCheckIsEmpty(ctx, typ)
		var encoder ValEncoder = &directMarshalerEncoder{
			checkIsEmpty: checkIsEmpty,
		}
		return encoder
	}
	if typ.Implements(marshalerType) {
		checkIsEmpty := createCheckIsEmpty(ctx, typ)
		var encoder ValEncoder = &marshalerEncoder{
			valType:      typ,
			checkIsEmpty: checkIsEmpty,
		}
		return encoder
	}
	ptrType := reflect.PointerTo(typ)
	if ctx.prefix != "" && ptrType.Implements(marshalerType) {
		checkIsEmpty := createCheckIsEmpty(ctx, ptrType)
		var encoder ValEncoder = &marshalerEncoder{
			valType:      ptrType,
			checkIsEmpty: checkIsEmpty,
		}
		return &referenceEncoder{encoder}
	}
	if typ == textMarshalerType {
		checkIsEmpty := createCheckIsEmpty(ctx, typ)
		var encoder ValEncoder = &directTextMarshalerEncoder{
			checkIsEmpty:  checkIsEmpty,
			stringEncoder: ctx.EncoderOf(reflect.TypeOf("")),
		}
		return encoder
	}
	if typ.Implements(textMarshalerType) {
		checkIsEmpty := createCheckIsEmpty(ctx, typ)
		var encoder ValEncoder = &textMarshalerEncoder{
			valType:       typ,
			stringEncoder: ctx.EncoderOf(reflect.TypeOf("")),
			checkIsEmpty:  checkIsEmpty,
		}
		return encoder
	}
	// if prefix is empty, the type is the root type
	if ctx.prefix != "" && ptrType.Implements(textMarshalerType) {
		checkIsEmpty := createCheckIsEmpty(ctx, ptrType)
		var encoder ValEncoder = &textMarshalerEncoder{
			valType:       ptrType,
			stringEncoder: ctx.EncoderOf(reflect.TypeOf("")),
			checkIsEmpty:  checkIsEmpty,
		}
		return &referenceEncoder{encoder}
	}
	return nil
}

type marshalerEncoder struct {
	checkIsEmpty checkIsEmpty
	valType      reflect.Type
}

func (encoder *marshalerEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	value := reflect.NewAt(encoder.valType, ptr)
	obj := reflect.Indirect(value).Interface()
	if value.IsNil() {
		stream.WriteNil()
		return
	}
	marshaler := obj.(json.Marshaler)
	bytes, err := marshaler.MarshalJSON()
	if err != nil {
		stream.Error = err
	} else {
		// html escape was already done by jsoniter
		// but the extra '\n' should be trimed
		l := len(bytes)
		if l > 0 && bytes[l-1] == '\n' {
			bytes = bytes[:l-1]
		}
		stream.Write(bytes)
	}
}

func (encoder *marshalerEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.checkIsEmpty.IsEmpty(ptr)
}

type directMarshalerEncoder struct {
	checkIsEmpty checkIsEmpty
}

func (encoder *directMarshalerEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	marshaler := *(*json.Marshaler)(ptr)
	if marshaler == nil {
		stream.WriteNil()
		return
	}
	bytes, err := marshaler.MarshalJSON()
	if err != nil {
		stream.Error = err
	} else {
		stream.Write(bytes)
	}
}

func (encoder *directMarshalerEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.checkIsEmpty.IsEmpty(ptr)
}

type textMarshalerEncoder struct {
	valType       reflect.Type
	stringEncoder ValEncoder
	checkIsEmpty  checkIsEmpty
}

func (encoder *textMarshalerEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	value := reflect.NewAt(encoder.valType, ptr)
	obj := reflect.Indirect(value).Interface()
	if value.IsNil() {
		stream.WriteNil()
		return
	}
	marshaler := (obj).(encoding.TextMarshaler)
	bytes, err := marshaler.MarshalText()
	if err != nil {
		stream.Error = err
	} else {
		str := string(bytes)
		encoder.stringEncoder.Encode(unsafe.Pointer(&str), stream)
	}
}

func (encoder *textMarshalerEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.checkIsEmpty.IsEmpty(ptr)
}

type directTextMarshalerEncoder struct {
	stringEncoder ValEncoder
	checkIsEmpty  checkIsEmpty
}

func (encoder *directTextMarshalerEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	marshaler := *(*encoding.TextMarshaler)(ptr)
	if marshaler == nil {
		stream.WriteNil()
		return
	}
	bytes, err := marshaler.MarshalText()
	if err != nil {
		stream.Error = err
	} else {
		str := string(bytes)
		encoder.stringEncoder.Encode(unsafe.Pointer(&str), stream)
	}
}

func (encoder *directTextMarshalerEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.checkIsEmpty.IsEmpty(ptr)
}

type unmarshalerDecoder struct {
	valType reflect.Type
}

func (decoder *unmarshalerDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	value := reflect.NewAt(decoder.valType, ptr)
	obj := reflect.Indirect(value).Interface()
	unmarshaler := obj.(json.Unmarshaler)
	iter.nextToken()
	iter.unreadByte() // skip spaces
	bytes := iter.SkipAndReturnBytes()
	err := unmarshaler.UnmarshalJSON(bytes)
	if err != nil {
		iter.ReportError("unmarshalerDecoder", err.Error())
	}
}

type textUnmarshalerDecoder struct {
	valType reflect.Type
}

func (decoder *textUnmarshalerDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	valType := decoder.valType
	value := reflect.NewAt(valType, ptr)
	obj := reflect.Indirect(value).Interface()
	if reflect.ValueOf(obj).IsNil() {
		elemType := valType.Elem()
		elem := reflect.New(elemType)
		value.Set(elem)
		obj = reflect.Indirect(value).Interface()
	}
	unmarshaler := (obj).(encoding.TextUnmarshaler)
	str := iter.ReadString()
	err := unmarshaler.UnmarshalText([]byte(str))
	if err != nil {
		iter.ReportError("textUnmarshalerDecoder", err.Error())
	}
}
