package jsoniter

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"unsafe"

	"github.com/modern-go/reflect2"
)

func decoderOfMap(ctx *ctx, typ reflect.Type) ValDecoder {
	keyType := typ.Key()
	elemTyp := typ.Elem()
	keyDecoder := decoderOfMapKey(ctx.append("[mapKey]"), keyType)
	elemDecoder := decoderOfType(ctx.append("[mapElem]"), elemTyp)
	return &mapDecoder{
		mapType:     typ,
		keyType:     keyType,
		elemType:    elemTyp,
		keyDecoder:  keyDecoder,
		elemDecoder: elemDecoder,
	}
}

func encoderOfMap(ctx *ctx, typ reflect.Type) ValEncoder {
	if ctx.sortMapKeys {
		return &sortKeysMapEncoder{
			mapType:     typ,
			keyEncoder:  encoderOfMapKey(ctx.append("[mapKey]"), typ.Key()),
			elemEncoder: encoderOfType(ctx.append("[mapElem]"), typ.Elem()),
		}
	}
	return &mapEncoder{
		mapType:     typ,
		keyEncoder:  encoderOfMapKey(ctx.append("[mapKey]"), typ.Key()),
		elemEncoder: encoderOfType(ctx.append("[mapElem]"), typ.Elem()),
	}
}

func decoderOfMapKey(ctx *ctx, typ reflect.Type) ValDecoder {
	decoder := ctx.decoderExtension.CreateMapKeyDecoder(typ)
	if decoder != nil {
		return decoder
	}
	for _, extension := range ctx.extraExtensions {
		decoder := extension.CreateMapKeyDecoder(typ)
		if decoder != nil {
			return decoder
		}
	}

	ptrType := reflect.PointerTo(typ)
	if ptrType.Implements(unmarshalerType) {
		return &referenceDecoder{
			&unmarshalerDecoder{
				valType: ptrType,
			},
		}
	}
	if typ.Implements(unmarshalerType) {
		return &unmarshalerDecoder{
			valType: typ,
		}
	}
	if ptrType.Implements(textUnmarshalerType) {
		return &referenceDecoder{
			&textUnmarshalerDecoder{
				valType: ptrType,
			},
		}
	}
	if typ.Implements(textUnmarshalerType) {
		return &textUnmarshalerDecoder{
			valType: typ,
		}
	}

	switch typ.Kind() {
	case reflect.String:
		return decoderOfType(ctx, defaultTypeOfKind(reflect.String))
	case reflect.Bool,
		reflect.Uint8, reflect.Int8,
		reflect.Uint16, reflect.Int16,
		reflect.Uint32, reflect.Int32,
		reflect.Uint64, reflect.Int64,
		reflect.Uint, reflect.Int,
		reflect.Float32, reflect.Float64,
		reflect.Uintptr:
		typ = defaultTypeOfKind(typ.Kind())
		return &numericMapKeyDecoder{decoderOfType(ctx, typ)}
	default:
		return &lazyErrorDecoder{err: fmt.Errorf("unsupported map key type: %v", typ)}
	}
}

func encoderOfMapKey(ctx *ctx, typ reflect.Type) ValEncoder {
	encoder := ctx.encoderExtension.CreateMapKeyEncoder(typ)
	if encoder != nil {
		return encoder
	}
	for _, extension := range ctx.extraExtensions {
		encoder := extension.CreateMapKeyEncoder(typ)
		if encoder != nil {
			return encoder
		}
	}

	if typ.Kind() != reflect.String {
		if typ == textMarshalerType {
			return &directTextMarshalerEncoder{
				stringEncoder: ctx.EncoderOf(reflect.TypeOf("")),
			}
		}
		if typ.Implements(textMarshalerType) {
			return &textMarshalerEncoder{
				valType:       typ,
				stringEncoder: ctx.EncoderOf(reflect.TypeOf("")),
			}
		}
	}

	switch typ.Kind() {
	case reflect.String:
		return encoderOfType(ctx, defaultTypeOfKind(reflect.String))
	case reflect.Bool,
		reflect.Uint8, reflect.Int8,
		reflect.Uint16, reflect.Int16,
		reflect.Uint32, reflect.Int32,
		reflect.Uint64, reflect.Int64,
		reflect.Uint, reflect.Int,
		reflect.Float32, reflect.Float64,
		reflect.Uintptr:
		typ = defaultTypeOfKind(typ.Kind())
		return &numericMapKeyEncoder{encoderOfType(ctx, typ)}
	default:
		if typ.Kind() == reflect.Interface {
			return &dynamicMapKeyEncoder{ctx, typ}
		}
		return &lazyErrorEncoder{err: fmt.Errorf("unsupported map key type: %v", typ)}
	}
}

// TODO: ported from reflect2
// defaultTypeOfKind return the non aliased default type for the kind
func defaultTypeOfKind(kind reflect.Kind) reflect.Type {
	return kindTypes[kind]
}

var kindTypes = map[reflect.Kind]reflect.Type{
	reflect.Bool:          reflect.TypeOf(true),
	reflect.Uint8:         reflect.TypeOf(uint8(0)),
	reflect.Int8:          reflect.TypeOf(int8(0)),
	reflect.Uint16:        reflect.TypeOf(uint16(0)),
	reflect.Int16:         reflect.TypeOf(int16(0)),
	reflect.Uint32:        reflect.TypeOf(uint32(0)),
	reflect.Int32:         reflect.TypeOf(int32(0)),
	reflect.Uint64:        reflect.TypeOf(uint64(0)),
	reflect.Int64:         reflect.TypeOf(int64(0)),
	reflect.Uint:          reflect.TypeOf(uint(0)),
	reflect.Int:           reflect.TypeOf(int(0)),
	reflect.Float32:       reflect.TypeOf(float32(0)),
	reflect.Float64:       reflect.TypeOf(float64(0)),
	reflect.Uintptr:       reflect.TypeOf(uintptr(0)),
	reflect.String:        reflect.TypeOf(""),
	reflect.UnsafePointer: reflect.TypeOf(unsafe.Pointer(nil)),
}

type mapDecoder struct {
	mapType     reflect.Type
	keyType     reflect.Type
	elemType    reflect.Type
	keyDecoder  ValDecoder
	elemDecoder ValDecoder
}

func (decoder *mapDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	mapType := decoder.mapType
	c := iter.nextToken()
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		*(*unsafe.Pointer)(ptr) = nil
		//value := reflect.NewAt(mapType, ptr)
		//m := reflect.MakeMap(mapType)
		//fmt.Printf("value: %v %#v\nm: %v, %#v", value, value, m, m)
		//value.Set(reflect.MakeMapWithSize(mapType, 0))
		return
	}
	value := reflect.Indirect(reflect.NewAt(mapType, ptr))
	//fmt.Printf("%#v\n%v\n%#v\n%v\n", decoder.mapType, decoder.mapType, value, value)
	// TODO: is this correct?
	if value.IsNil() {
		value.Set(reflect.MakeMapWithSize(mapType, 0))
	}
	if c != '{' {
		iter.ReportError("ReadMapCB", `expect { or n, but found `+string([]byte{c}))
		return
	}
	c = iter.nextToken()
	if c == '}' {
		return
	}
	iter.unreadByte()
	fmt.Printf("keyType: %v\nelemType: %v\n", decoder.keyType, decoder.elemType)
	key := reflect.New(decoder.keyType)
	decoder.keyDecoder.Decode(key.UnsafePointer(), iter)
	c = iter.nextToken()
	if c != ':' {
		iter.ReportError("ReadMapCB", "expect : after object field, but found "+string([]byte{c}))
		return
	}
	elem := reflect.New(decoder.elemType)
	fmt.Printf("key: %#v, key.Elem(): %#v, elem: %#v, elem.Elem(): %v\n", key, key.Elem(), elem, elem.Elem())
	decoder.elemDecoder.Decode(elem.UnsafePointer(), iter)
	value.SetMapIndex(key.Elem(), elem.Elem())
	for c = iter.nextToken(); c == ','; c = iter.nextToken() {
		key = reflect.New(decoder.keyType)
		decoder.keyDecoder.Decode(key.UnsafePointer(), iter)
		c = iter.nextToken()
		if c != ':' {
			iter.ReportError("ReadMapCB", "expect : after object field, but found "+string([]byte{c}))
			return
		}
		elem = reflect.New(decoder.elemType)
		fmt.Printf("key: %#v, key.Elem(): %#v, elem: %#v, elem.Elem(): %v\n", key, key.Elem(), elem, elem.Elem())
		decoder.elemDecoder.Decode(elem.UnsafePointer(), iter)
		value.SetMapIndex(key.Elem(), elem.Elem())
	}
	if c != '}' {
		iter.ReportError("ReadMapCB", `expect }, but found `+string([]byte{c}))
	}
}

type numericMapKeyDecoder struct {
	decoder ValDecoder
}

func (decoder *numericMapKeyDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	c := iter.nextToken()
	if c != '"' {
		iter.ReportError("ReadMapCB", `expect ", but found `+string([]byte{c}))
		return
	}
	decoder.decoder.Decode(ptr, iter)
	c = iter.nextToken()
	if c != '"' {
		iter.ReportError("ReadMapCB", `expect ", but found `+string([]byte{c}))
		return
	}
}

type numericMapKeyEncoder struct {
	encoder ValEncoder
}

func (encoder *numericMapKeyEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	stream.writeByte('"')
	encoder.encoder.Encode(ptr, stream)
	stream.writeByte('"')
}

func (encoder *numericMapKeyEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return false
}

type dynamicMapKeyEncoder struct {
	ctx     *ctx
	valType reflect.Type
}

func (encoder *dynamicMapKeyEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	obj := reflect.Indirect(reflect.NewAt(encoder.valType, ptr))
	encoderOfMapKey(encoder.ctx, reflect.TypeOf(obj)).Encode(obj.UnsafePointer(), stream)
}

func (encoder *dynamicMapKeyEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	obj := reflect.Indirect(reflect.NewAt(encoder.valType, ptr))
	return encoderOfMapKey(encoder.ctx, reflect.TypeOf(obj)).IsEmpty(reflect2.PtrOf(obj))
}

type mapEncoder struct {
	mapType     reflect.Type
	keyEncoder  ValEncoder
	elemEncoder ValEncoder
}

func (encoder *mapEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *(*unsafe.Pointer)(ptr) == nil {
		stream.WriteNil()
		return
	}
	stream.WriteObjectStart()
	value := reflect.NewAt(encoder.mapType, ptr)
	fmt.Printf("%#v\n%v\n%#v\n%v\n", encoder.mapType, encoder.mapType, value, value)
	// TODO: nil check?
	iter := value.Elem().MapRange()
	i := 0
	for iter.Next() {
		if i != 0 {
			stream.WriteMore()
		}
		key, elem := iter.Key(), iter.Value()
		encoder.keyEncoder.Encode(unsafe.Pointer(key.UnsafeAddr()), stream)
		if stream.indention > 0 {
			stream.writeTwoBytes(byte(':'), byte(' '))
		} else {
			stream.writeByte(':')
		}
		encoder.elemEncoder.Encode(unsafe.Pointer(elem.UnsafeAddr()), stream)
		i++
	}
	stream.WriteObjectEnd()
}

func (encoder *mapEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	iter := reflect.NewAt(encoder.mapType, ptr).MapRange()
	return !iter.Next()
}

type sortKeysMapEncoder struct {
	mapType     reflect.Type
	keyEncoder  ValEncoder
	elemEncoder ValEncoder
}

func (encoder *sortKeysMapEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *(*unsafe.Pointer)(ptr) == nil {
		stream.WriteNil()
		return
	}
	stream.WriteObjectStart()
	mapIter := reflect.NewAt(encoder.mapType, ptr).Elem().MapRange()
	subStream := stream.cfg.BorrowStream(nil)
	subStream.Attachment = stream.Attachment
	subIter := stream.cfg.BorrowIterator(nil)
	keyValues := encodedKeyValues{}
	for mapIter.Next() {
		key, elem := mapIter.Key(), mapIter.Value()
		subStreamIndex := subStream.Buffered()
		fmt.Printf("key: %v %#v\n", key, key)
		encoder.keyEncoder.Encode(unsafe.Pointer(key.UnsafeAddr()), subStream)
		if subStream.Error != nil && subStream.Error != io.EOF && stream.Error == nil {
			stream.Error = subStream.Error
		}
		encodedKey := subStream.Buffer()[subStreamIndex:]
		subIter.ResetBytes(encodedKey)
		decodedKey := subIter.ReadString()
		if stream.indention > 0 {
			subStream.writeTwoBytes(byte(':'), byte(' '))
		} else {
			subStream.writeByte(':')
		}
		encoder.elemEncoder.Encode(unsafe.Pointer(elem.UnsafeAddr()), subStream)
		keyValues = append(keyValues, encodedKV{
			key:      decodedKey,
			keyValue: subStream.Buffer()[subStreamIndex:],
		})
	}
	sort.Sort(keyValues)
	for i, keyValue := range keyValues {
		if i != 0 {
			stream.WriteMore()
		}
		stream.Write(keyValue.keyValue)
	}
	if subStream.Error != nil && stream.Error == nil {
		stream.Error = subStream.Error
	}
	stream.WriteObjectEnd()
	stream.cfg.ReturnStream(subStream)
	stream.cfg.ReturnIterator(subIter)
}

func (encoder *sortKeysMapEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	iter := reflect.NewAt(encoder.mapType, ptr).MapRange()
	return !iter.Next()
}

type encodedKeyValues []encodedKV

type encodedKV struct {
	key      string
	keyValue []byte
}

func (sv encodedKeyValues) Len() int           { return len(sv) }
func (sv encodedKeyValues) Swap(i, j int)      { sv[i], sv[j] = sv[j], sv[i] }
func (sv encodedKeyValues) Less(i, j int) bool { return sv[i].key < sv[j].key }
