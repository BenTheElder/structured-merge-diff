package jsoniter

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/modern-go/reflect2"
)

// ValDecoder is an internal type registered to cache as needed.
// Don't confuse jsoniter.ValDecoder with json.Decoder.
// For json.Decoder's adapter, refer to jsoniter.AdapterDecoder(todo link).
//
// Reflection on type to create decoders, which is then cached
// Reflection on value is avoided as we can, as the reflect.Value itself will allocate, with following exceptions
// 1. create instance of new value, for example *int will need a int to be allocated
// 2. append to slice, if the existing cap is not enough, allocate will be done using Reflect.New
// 3. assignment to map, both key and value will be reflect.Value
// For a simple struct binding, it will be reflect.Value free and allocation free
type ValDecoder interface {
	Decode(ptr unsafe.Pointer, iter *Iterator)
}

// ValEncoder is an internal type registered to cache as needed.
// Don't confuse jsoniter.ValEncoder with json.Encoder.
// For json.Encoder's adapter, refer to jsoniter.AdapterEncoder(todo godoc link).
type ValEncoder interface {
	IsEmpty(ptr unsafe.Pointer) bool
	Encode(ptr unsafe.Pointer, stream *Stream)
}

type checkIsEmpty interface {
	IsEmpty(ptr unsafe.Pointer) bool
}

type ctx struct {
	*frozenConfig
	prefix   string
	encoders map[reflect.Type]ValEncoder
	decoders map[reflect.Type]ValDecoder
}

func (b *ctx) caseSensitive() bool {
	if b.frozenConfig == nil {
		// default is case-insensitive
		return false
	}
	return b.frozenConfig.caseSensitive
}

func (b *ctx) append(prefix string) *ctx {
	return &ctx{
		frozenConfig: b.frozenConfig,
		prefix:       b.prefix + " " + prefix,
		encoders:     b.encoders,
		decoders:     b.decoders,
	}
}

// ReadVal copy the underlying JSON into go interface, same as json.Unmarshal
func (iter *Iterator) ReadVal(obj interface{}) {
	depth := iter.depth
	cacheKey := reflect.TypeOf(obj)
	decoder := iter.cfg.getDecoderFromCache(cacheKey)
	if decoder == nil {
		typ := reflect.TypeOf(obj)
		if typ == nil || typ.Kind() != reflect.Ptr {
			iter.ReportError("ReadVal", "can only unmarshal into pointer")
			return
		}
		decoder = iter.cfg.DecoderOf(typ)
	}
	ptr := reflect2.PtrOf(obj)
	if ptr == nil {
		iter.ReportError("ReadVal", "can not read into nil pointer")
		return
	}
	decoder.Decode(ptr, iter)
	if iter.depth != depth {
		iter.ReportError("ReadVal", "unexpected mismatched nesting")
		return
	}
}

// WriteVal copy the go interface into underlying JSON, same as json.Marshal
func (stream *Stream) WriteVal(val interface{}) {
	if nil == val {
		stream.WriteNil()
		return
	}
	cacheKey := reflect.TypeOf(val)
	encoder := stream.cfg.getEncoderFromCache(cacheKey)
	if encoder == nil {
		typ := reflect.TypeOf(val)
		encoder = stream.cfg.EncoderOf(typ)
	}
	encoder.Encode(reflect2.PtrOf(val), stream)
}

func (cfg *frozenConfig) DecoderOf(typ reflect.Type) ValDecoder {
	decoder := cfg.getDecoderFromCache(typ)
	if decoder != nil {
		return decoder
	}
	ctx := &ctx{
		frozenConfig: cfg,
		prefix:       "",
		decoders:     map[reflect.Type]ValDecoder{},
		encoders:     map[reflect.Type]ValEncoder{},
	}
	// assume pointer type
	decoder = decoderOfType(ctx, typ.Elem())
	cfg.addDecoderToCache(typ, decoder)
	return decoder
}

func decoderOfType(ctx *ctx, typ reflect.Type) ValDecoder {
	decoder := getTypeDecoderFromExtension(ctx, typ)
	if decoder != nil {
		return decoder
	}
	decoder = createDecoderOfType(ctx, typ)
	for _, extension := range extensions {
		decoder = extension.DecorateDecoder(typ, decoder)
	}
	decoder = ctx.decoderExtension.DecorateDecoder(typ, decoder)
	for _, extension := range ctx.extraExtensions {
		decoder = extension.DecorateDecoder(typ, decoder)
	}
	return decoder
}

func createDecoderOfType(ctx *ctx, typ reflect.Type) ValDecoder {
	decoder := ctx.decoders[typ]
	if decoder != nil {
		return decoder
	}
	placeholder := &placeholderDecoder{}
	ctx.decoders[typ] = placeholder
	decoder = _createDecoderOfType(ctx, typ)
	placeholder.decoder = decoder
	return decoder
}

func _createDecoderOfType(ctx *ctx, typ reflect.Type) ValDecoder {
	decoder := createDecoderOfJsonRawMessage(ctx, typ)
	if decoder != nil {
		return decoder
	}
	decoder = createDecoderOfJsonNumber(ctx, typ)
	if decoder != nil {
		return decoder
	}
	decoder = createDecoderOfMarshaler(ctx, typ)
	if decoder != nil {
		return decoder
	}
	decoder = createDecoderOfAny(ctx, typ)
	if decoder != nil {
		return decoder
	}
	decoder = createDecoderOfNative(ctx, typ)
	if decoder != nil {
		return decoder
	}
	switch typ.Kind() {
	case reflect.Interface:
		ifaceType, isIFace := typ.(*reflect2.UnsafeIFaceType)
		if isIFace {
			return &ifaceDecoder{valType: ifaceType}
		}
		return &efaceDecoder{}
	case reflect.Struct:
		return decoderOfStruct(ctx, typ)
	case reflect.Array:
		return decoderOfArray(ctx, typ)
	case reflect.Slice:
		return decoderOfSlice(ctx, typ)
	case reflect.Map:
		return decoderOfMap(ctx, typ)
	case reflect.Ptr:
		return decoderOfOptional(ctx, typ)
	default:
		return &lazyErrorDecoder{err: fmt.Errorf("%s%s is unsupported type", ctx.prefix, typ.String())}
	}
}

func (cfg *frozenConfig) EncoderOf(typ reflect.Type) ValEncoder {
	encoder := cfg.getEncoderFromCache(typ)
	if encoder != nil {
		return encoder
	}
	ctx := &ctx{
		frozenConfig: cfg,
		prefix:       "",
		decoders:     map[reflect.Type]ValDecoder{},
		encoders:     map[reflect.Type]ValEncoder{},
	}
	encoder = encoderOfType(ctx, typ)
	if likePtrType(typ) {
		encoder = &onePtrEncoder{encoder}
	}
	cfg.addEncoderToCache(typ, encoder)
	return encoder
}

// TODO
func likePtrKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Func:
		return true
	}
	return false
}

func likePtrType(typ reflect.Type) bool {
	if likePtrKind(typ.Kind()) {
		return true
	}
	if typ.Kind() == reflect.Struct {
		if typ.NumField() != 1 {
			return false
		}
		return likePtrType(typ.Field(0).Type)
	}
	if typ.Kind() == reflect.Array {
		if typ.Len() != 1 {
			return false
		}
		return likePtrType(typ.Elem())
	}
	return false
}

// end TODO

type onePtrEncoder struct {
	encoder ValEncoder
}

func (encoder *onePtrEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.encoder.IsEmpty(unsafe.Pointer(&ptr))
}

func (encoder *onePtrEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	encoder.encoder.Encode(unsafe.Pointer(&ptr), stream)
}

func encoderOfType(ctx *ctx, typ reflect.Type) ValEncoder {
	encoder := getTypeEncoderFromExtension(ctx, typ)
	if encoder != nil {
		return encoder
	}
	encoder = createEncoderOfType(ctx, typ)
	for _, extension := range extensions {
		encoder = extension.DecorateEncoder(typ, encoder)
	}
	encoder = ctx.encoderExtension.DecorateEncoder(typ, encoder)
	for _, extension := range ctx.extraExtensions {
		encoder = extension.DecorateEncoder(typ, encoder)
	}
	return encoder
}

func createEncoderOfType(ctx *ctx, typ reflect.Type) ValEncoder {
	encoder := ctx.encoders[typ]
	if encoder != nil {
		return encoder
	}
	placeholder := &placeholderEncoder{}
	ctx.encoders[typ] = placeholder
	encoder = _createEncoderOfType(ctx, typ)
	placeholder.encoder = encoder
	return encoder
}
func _createEncoderOfType(ctx *ctx, typ reflect.Type) ValEncoder {
	encoder := createEncoderOfJsonRawMessage(ctx, typ)
	if encoder != nil {
		return encoder
	}
	encoder = createEncoderOfJsonNumber(ctx, typ)
	if encoder != nil {
		return encoder
	}
	encoder = createEncoderOfMarshaler(ctx, typ)
	if encoder != nil {
		return encoder
	}
	encoder = createEncoderOfAny(ctx, typ)
	if encoder != nil {
		return encoder
	}
	encoder = createEncoderOfNative(ctx, typ)
	if encoder != nil {
		return encoder
	}
	kind := typ.Kind()
	switch kind {
	case reflect.Interface:
		return &dynamicEncoder{typ}
	case reflect.Struct:
		return encoderOfStruct(ctx, typ)
	case reflect.Array:
		return encoderOfArray(ctx, typ)
	case reflect.Slice:
		return encoderOfSlice(ctx, typ)
	case reflect.Map:
		return encoderOfMap(ctx, typ)
	case reflect.Ptr:
		return encoderOfOptional(ctx, typ)
	default:
		return &lazyErrorEncoder{err: fmt.Errorf("%s%s is unsupported type", ctx.prefix, typ.String())}
	}
}

type lazyErrorDecoder struct {
	err error
}

func (decoder *lazyErrorDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if iter.WhatIsNext() != NilValue {
		if iter.Error == nil {
			iter.Error = decoder.err
		}
	} else {
		iter.Skip()
	}
}

type lazyErrorEncoder struct {
	err error
}

func (encoder *lazyErrorEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	if ptr == nil {
		stream.WriteNil()
	} else if stream.Error == nil {
		stream.Error = encoder.err
	}
}

func (encoder *lazyErrorEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return false
}

type placeholderDecoder struct {
	decoder ValDecoder
}

func (decoder *placeholderDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	decoder.decoder.Decode(ptr, iter)
}

type placeholderEncoder struct {
	encoder ValEncoder
}

func (encoder *placeholderEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	encoder.encoder.Encode(ptr, stream)
}

func (encoder *placeholderEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.encoder.IsEmpty(ptr)
}
