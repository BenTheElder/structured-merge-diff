package jsoniter

import (
	"reflect"
	"unsafe"
)

func decoderOfOptional(ctx *ctx, typ reflect.Type) ValDecoder {
	elemType := typ.Elem()
	decoder := decoderOfType(ctx, elemType)
	return &OptionalDecoder{elemType, decoder}
}

func encoderOfOptional(ctx *ctx, typ reflect.Type) ValEncoder {
	elemType := typ.Elem()
	elemEncoder := encoderOfType(ctx, elemType)
	encoder := &OptionalEncoder{elemEncoder}
	return encoder
}

type OptionalDecoder struct {
	ValueType    reflect.Type
	ValueDecoder ValDecoder
}

func (decoder *OptionalDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if iter.ReadNil() {
		*((*unsafe.Pointer)(ptr)) = nil
	} else {
		if *((*unsafe.Pointer)(ptr)) == nil {
			//pointer to null, we have to allocate memory to hold the value
			newPtr := reflect.New(decoder.ValueType).UnsafePointer()
			decoder.ValueDecoder.Decode(newPtr, iter)
			*((*unsafe.Pointer)(ptr)) = newPtr
		} else {
			//reuse existing instance
			decoder.ValueDecoder.Decode(*((*unsafe.Pointer)(ptr)), iter)
		}
	}
}

type dereferenceDecoder struct {
	// only to deference a pointer
	valueType    reflect.Type
	valueDecoder ValDecoder
}

func (decoder *dereferenceDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if *((*unsafe.Pointer)(ptr)) == nil {
		//pointer to null, we have to allocate memory to hold the value
		newPtr := reflect.New(decoder.valueType).UnsafePointer()
		decoder.valueDecoder.Decode(newPtr, iter)
		*((*unsafe.Pointer)(ptr)) = newPtr
	} else {
		//reuse existing instance
		decoder.valueDecoder.Decode(*((*unsafe.Pointer)(ptr)), iter)
	}
}

type OptionalEncoder struct {
	ValueEncoder ValEncoder
}

func (encoder *OptionalEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *((*unsafe.Pointer)(ptr)) == nil {
		stream.WriteNil()
	} else {
		encoder.ValueEncoder.Encode(*((*unsafe.Pointer)(ptr)), stream)
	}
}

func (encoder *OptionalEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*unsafe.Pointer)(ptr)) == nil
}

type dereferenceEncoder struct {
	ValueEncoder ValEncoder
}

func (encoder *dereferenceEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	if *((*unsafe.Pointer)(ptr)) == nil {
		stream.WriteNil()
	} else {
		encoder.ValueEncoder.Encode(*((*unsafe.Pointer)(ptr)), stream)
	}
}

func (encoder *dereferenceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	dePtr := *((*unsafe.Pointer)(ptr))
	if dePtr == nil {
		return true
	}
	return encoder.ValueEncoder.IsEmpty(dePtr)
}

func (encoder *dereferenceEncoder) IsEmbeddedPtrNil(ptr unsafe.Pointer) bool {
	deReferenced := *((*unsafe.Pointer)(ptr))
	if deReferenced == nil {
		return true
	}
	isEmbeddedPtrNil, converted := encoder.ValueEncoder.(IsEmbeddedPtrNil)
	if !converted {
		return false
	}
	fieldPtr := unsafe.Pointer(deReferenced)
	return isEmbeddedPtrNil.IsEmbeddedPtrNil(fieldPtr)
}

type referenceEncoder struct {
	encoder ValEncoder
}

func (encoder *referenceEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	encoder.encoder.Encode(unsafe.Pointer(&ptr), stream)
}

func (encoder *referenceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.encoder.IsEmpty(unsafe.Pointer(&ptr))
}

type referenceDecoder struct {
	decoder ValDecoder
}

func (decoder *referenceDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	decoder.decoder.Decode(unsafe.Pointer(&ptr), iter)
}
