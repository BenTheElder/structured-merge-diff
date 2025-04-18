package jsoniter

import (
	"encoding/json"
	"reflect"
	"unsafe"
)

var jsonRawMessageType = reflect.TypeOf((*json.RawMessage)(nil)).Elem()
var jsoniterRawMessageType = reflect.TypeOf((*RawMessage)(nil)).Elem()

func createEncoderOfJsonRawMessage(ctx *ctx, typ reflect.Type) ValEncoder {
	if typ == jsonRawMessageType {
		return &jsonRawMessageCodec{}
	}
	if typ == jsoniterRawMessageType {
		return &jsoniterRawMessageCodec{}
	}
	return nil
}

func createDecoderOfJsonRawMessage(ctx *ctx, typ reflect.Type) ValDecoder {
	if typ == jsonRawMessageType {
		return &jsonRawMessageCodec{}
	}
	if typ == jsoniterRawMessageType {
		return &jsoniterRawMessageCodec{}
	}
	return nil
}

type jsonRawMessageCodec struct {
}

func (codec *jsonRawMessageCodec) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if iter.ReadNil() {
		*((*json.RawMessage)(ptr)) = nil
	} else {
		*((*json.RawMessage)(ptr)) = iter.SkipAndReturnBytes()
	}
}

func (codec *jsonRawMessageCodec) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *((*json.RawMessage)(ptr)) == nil {
		stream.WriteNil()
	} else {
		stream.WriteRaw(string(*((*json.RawMessage)(ptr))))
	}
}

func (codec *jsonRawMessageCodec) IsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*json.RawMessage)(ptr))) == 0
}

type jsoniterRawMessageCodec struct {
}

func (codec *jsoniterRawMessageCodec) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if iter.ReadNil() {
		*((*RawMessage)(ptr)) = nil
	} else {
		*((*RawMessage)(ptr)) = iter.SkipAndReturnBytes()
	}
}

func (codec *jsoniterRawMessageCodec) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *((*RawMessage)(ptr)) == nil {
		stream.WriteNil()
	} else {
		stream.WriteRaw(string(*((*RawMessage)(ptr))))
	}
}

func (codec *jsoniterRawMessageCodec) IsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*RawMessage)(ptr))) == 0
}
