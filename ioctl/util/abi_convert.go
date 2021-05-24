package util

import (
	"github.com/pkg/errors"
)

// vars
var (
	ErrWrongType = errors.New("wrong data type held by interface{}")
)

// To32Bytes asserts/converts interface{} to [32]byte
func To32Bytes(v interface{}) ([32]byte, error) {
	if b32, ok := v.([32]byte); ok {
		return b32, nil
	}
	return [32]byte{}, ErrWrongType
}

// ToByteSlice asserts/converts interface{} to []byte
func ToByteSlice(v interface{}) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	return nil, ErrWrongType
}
