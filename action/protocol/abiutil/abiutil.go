package abiutil

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func MustLoadMethod(abiStr, method string) abi.Method {
	_interface, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		panic(err)
	}
	_method, ok := _interface.Methods[method]
	if !ok {
		panic("fail to load the method")
	}
	return _method
}
