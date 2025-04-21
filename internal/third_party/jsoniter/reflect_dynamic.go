package jsoniter

import (
	"fmt"
	"reflect"
	"unsafe"
)

type dynamicEncoder struct {
	valType reflect.Type
}

func (encoder *dynamicEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	obj := reflect.Indirect(reflect.NewAt(encoder.valType, ptr)).Interface()
	stream.WriteVal(obj)
}

func (encoder *dynamicEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return reflect.NewAt(encoder.valType, ptr).IsNil()
}

type efaceDecoder struct {
}

func (decoder *efaceDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	pObj := (*interface{})(ptr)
	obj := *pObj
	if obj == nil {
		*pObj = iter.Read()
		return
	}
	typ := reflect.TypeOf(obj)
	if typ.Kind() != reflect.Ptr {
		*pObj = iter.Read()
		return
	}
	ptrElemType := typ.Elem()
	if iter.WhatIsNext() == NilValue {
		if ptrElemType.Kind() != reflect.Ptr {
			iter.skipFourBytes('n', 'u', 'l', 'l')
			*pObj = nil
			return
		}
	}
	if reflect.ValueOf(obj).IsNil() {
		obj := reflect.New(ptrElemType)
		iter.ReadVal(obj)
		*pObj = obj
		return
	}
	iter.ReadVal(obj)
}

type ifaceDecoder struct {
	valType reflect.Type
}

func (decoder *ifaceDecoder) Decode(ptr unsafe.Pointer, iter *Iterator) {
	if iter.ReadNil() {
		value := reflect.NewAt(decoder.valType, ptr)
		fmt.Printf("valType: %v, %#v, val: %v %#v\n", decoder.valType, decoder.valType, value, value)
		value.SetPointer(reflect.New(decoder.valType).UnsafePointer())
		//value.Set(reflect.New(decoder.valType))
		return
	}
	value := reflect.NewAt(decoder.valType, ptr)
	if value.IsNil() {
		iter.ReportError("decode non empty interface", "can not unmarshal into nil")
		return
	}
	obj := reflect.Indirect(value).Interface()
	iter.ReadVal(obj)
}
