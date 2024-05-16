package common

import "errors"

var (
	ErrInvalidCallData  = errors.New("invalid call binary data")
	ErrInvalidCallSig   = errors.New("invalid call sig")
	ErrConvertBigNumber = errors.New("convert big number error")
	ErrDecodeFailure    = errors.New("decode data error")
)

const (
	ErrInvalidMsg = "address length = 40, expecting 41: invalid address"
)
