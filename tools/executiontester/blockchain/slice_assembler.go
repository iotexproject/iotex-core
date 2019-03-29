package blockchain

import (
	"reflect"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// ICanBytes is []byte
	ICanBytes interface {
		Bytes() []byte
	}

	// SliceAssembler is an interface to pack arguments for ABI
	SliceAssembler interface {
		PackArguments(args ...interface{}) ([]byte, error)
		IsDynamicType(param interface{}) bool
	}

	sliceAssembler struct {
	}
)

// NewSliceAssembler creates a slice assembler
func NewSliceAssembler() SliceAssembler {
	return &sliceAssembler{}
}

func (sa *sliceAssembler) IsDynamicType(param interface{}) bool {
	t := reflect.TypeOf(param)

	if t.Kind() == reflect.String {
		return true
	}

	if t.Kind() != reflect.Slice {
		return false
	}

	if t.Elem().Kind() == reflect.Uint8 && reflect.ValueOf(param).Len() <= 32 {
		return false
	}
	return true
}

func (sa *sliceAssembler) PackArguments(args ...interface{}) ([]byte, error) {
	var variableInput []byte
	var ret []byte
	inputOffset := len(args) * 32

	for _, a := range args {
		packed := sa.pack(a)

		if !sa.IsDynamicType(a) {
			ret = append(ret, packed...)
			continue
		}

		ret = append(ret, packNum(reflect.ValueOf(inputOffset))...)
		inputOffset += len(packed)
		variableInput = append(variableInput, packed...)
	}

	ret = append(ret, variableInput...)
	return ret, nil
}

func (sa *sliceAssembler) pack(a interface{}) []byte {
	switch a.(type) {
	case string:
		bs, _ := packByte([]byte(a.(string)))
		return bs

	case []byte:
		if len(a.([]byte)) > 32 {
			bs, _ := packByte(a.([]byte))
			return bs
		}
	}

	if !sa.IsDynamicType(a) {
		switch a.(type) {
		case []byte:
			bs := rightPadBytes(a.([]byte), 32)
			return bs[:]
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return packNum(reflect.ValueOf(a))
		case ICanBytes:
			bs := hash.BytesToHash256(a.(ICanBytes).Bytes())
			return bs[:]
		default:
			panic("fail")
		}
	}

	v := reflect.ValueOf(a)
	ret := packNum(reflect.ValueOf(v.Len()))
	for i := 0; i < v.Len(); i++ {
		switch v.Index(i).Interface().(type) {
		case ICanBytes:
			item := v.Index(i)
			if !item.IsValid() {
				item = item.Elem()
			}
			bs := hash.BytesToHash256(item.Interface().(ICanBytes).Bytes())
			ret = append(ret, bs[:]...)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			ret = append(ret, packNum(reflect.ValueOf(v.Index(i).Interface()))...)
		default:
			panic("abi: fatal error")
		}
	}
	return ret
}
