package jsoniter

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

func decoderOfSlice(ctx *ctx, typ reflect.Type) ValDecoder {
	decoder := decoderOfType(ctx.append("[sliceElem]"), typ.Elem())
	return &sliceDecoder{typ, decoder}
}

func encoderOfSlice(ctx *ctx, typ reflect.Type) ValEncoder {
	encoder := encoderOfType(ctx.append("[sliceElem]"), typ.Elem())
	return &sliceEncoder{typ, encoder}
}

type sliceEncoder struct {
	sliceType   reflect.Type
	elemEncoder ValEncoder
}

func (encoder *sliceEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	value := reflect.NewAt(encoder.sliceType, ptr)
	if value.IsNil() {
		stream.WriteNil()
		return
	}
	length := value.Len()
	if length == 0 {
		stream.WriteEmptyArray()
		return
	}
	stream.WriteArrayStart()
	encoder.elemEncoder.Encode(value.Index(0).UnsafePointer(), stream)
	for i := 1; i < length; i++ {
		stream.WriteMore()
		elemPtr := value.Index(i).UnsafePointer()
		encoder.elemEncoder.Encode(elemPtr, stream)
	}
	stream.WriteArrayEnd()
	if stream.Error != nil && stream.Error != io.EOF {
		stream.Error = fmt.Errorf("%v: %s", encoder.sliceType, stream.Error.Error())
	}
}

func (encoder *sliceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	value := reflect.NewAt(encoder.sliceType, ptr)
	return value.Len() == 0
}

type sliceDecoder struct {
	sliceType   reflect.Type
	elemDecoder ValDecoder
}

func (decoder *sliceDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	decoder.doDecode(ptr, iter)
	if iter.Error != nil && iter.Error != io.EOF {
		iter.Error = fmt.Errorf("%v: %s", decoder.sliceType, iter.Error.Error())
	}
}

func (decoder *sliceDecoder) doDecode(ptr unsafe.Pointer, iter *Iterator) {
	c := iter.nextToken()
	sliceType := decoder.sliceType
	value := reflect.NewAt(sliceType, ptr)
	if c == 'n' {
		iter.skipThreeBytes('u', 'l', 'l')
		value.SetZero()
		return
	}
	if c != '[' {
		iter.ReportError("decode slice", "expect [ or n, but found "+string([]byte{c}))
		return
	}
	c = iter.nextToken()
	if c == ']' {
		value.Set(reflect.MakeSlice(sliceType, 0, 0))
		return
	}
	iter.unreadByte()
	value.Grow(1)
	elemPtr := value.UnsafePointer()
	decoder.elemDecoder.Decode(elemPtr, iter)
	length := 1
	for c = iter.nextToken(); c == ','; c = iter.nextToken() {
		idx := length
		length += 1
		value.Grow(length)
		elemPtr = value.Index(idx).UnsafePointer()
		decoder.elemDecoder.Decode(elemPtr, iter)
	}
	if c != ']' {
		iter.ReportError("decode slice", "expect ], but found "+string([]byte{c}))
		return
	}
}
