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
	//fmt.Printf("%#v\n%v\n%#v\n%v\n", encoder.sliceType, encoder.sliceType, value, value)
	if value.IsNil() {
		stream.WriteNil()
		return
	}
	// deref pointer
	value = value.Elem()
	//fmt.Printf("%#v\n%v\n%#v\n%v\n", encoder.sliceType, encoder.sliceType, value, value)
	length := value.Len()
	if length == 0 {
		stream.WriteEmptyArray()
		return
	}
	stream.WriteArrayStart()
	encoder.elemEncoder.Encode(unsafe.Pointer(value.Index(0).UnsafeAddr()), stream)
	for i := 1; i < length; i++ {
		stream.WriteMore()
		elemPtr := unsafe.Pointer(value.Index(i).UnsafeAddr())
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
	value := reflect.Indirect(reflect.NewAt(sliceType, ptr))
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
	length := 1
	value.Grow(1)
	value.SetLen(length)
	elemPtr := unsafe.Pointer(value.Index(0).UnsafeAddr())
	decoder.elemDecoder.Decode(elemPtr, iter)
	for c = iter.nextToken(); c == ','; c = iter.nextToken() {
		idx := length
		length += 1
		value.Grow(1)
		value.SetLen(length)
		elemPtr = unsafe.Pointer(value.Index(idx).UnsafeAddr())
		decoder.elemDecoder.Decode(elemPtr, iter)
	}
	if c != ']' {
		iter.ReportError("decode slice", "expect ], but found "+string([]byte{c}))
		return
	}
}
