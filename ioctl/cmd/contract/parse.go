// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

func parseAbi(abiBytes []byte) (*abi.ABI, error) {
	parsedAbi, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal abi", err)
	}
	return &parsedAbi, nil
}

func parseInput(rowInput string) (map[string]interface{}, error) {
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(rowInput), &input); err != nil {
		return nil, output.NewError(output.SerializationError, "failed to unmarshal arguments", err)
	}
	return input, nil
}

func parseArgument(t *abi.Type, arg interface{}) (interface{}, error) {
	switch t.T {
	default:
		return nil, ErrInvalidArg

	case abi.BoolTy:
		if reflect.TypeOf(arg).Kind() != reflect.Bool {
			return nil, ErrInvalidArg
		}

	case abi.StringTy:
		if reflect.TypeOf(arg).Kind() != reflect.String {
			return nil, ErrInvalidArg
		}

	case abi.SliceTy:
		if reflect.TypeOf(arg).Kind() != reflect.Slice {
			return nil, ErrInvalidArg
		}

		slice := reflect.MakeSlice(t.Type, 0, t.Size)

		s := reflect.ValueOf(arg)
		for i := 0; i < s.Len(); i++ {
			ele, err := parseArgument(t.Elem, s.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			slice = reflect.Append(slice, reflect.ValueOf(ele))
		}

		arg = slice.Interface()

	case abi.ArrayTy:
		if reflect.TypeOf(arg).Kind() != reflect.Slice {
			return nil, ErrInvalidArg
		}

		arrayType := reflect.ArrayOf(t.Size, t.Elem.Type)
		array := reflect.New(arrayType).Elem()

		s := reflect.ValueOf(arg)
		for i := 0; i < s.Len(); i++ {
			ele, err := parseArgument(t.Elem, s.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			array.Index(i).Set(reflect.ValueOf(ele))
		}

		arg = array.Interface()

	// support both of Ether address & IoTeX address input
	case abi.AddressTy:
		var err error
		addrString, ok := arg.(string)
		if !ok {
			return nil, ErrInvalidArg
		}

		if common.IsHexAddress(addrString) {
			arg = common.HexToAddress(addrString)
		} else {
			arg, err = util.IoAddrToEvmAddr(addrString)
			if err != nil {
				return nil, err
			}
		}

	// support both number & string input
	case abi.IntTy:
		var ok bool
		var err error

		k := reflect.TypeOf(arg).Kind()
		if k != reflect.String && k != reflect.Float64 {
			return nil, ErrInvalidArg
		}

		switch t.Size {
		default:
			if k == reflect.String {
				arg, ok = new(big.Int).SetString(arg.(string), 10)
				if !ok {
					return nil, ErrInvalidArg
				}
			} else {
				arg = big.NewInt(int64(arg.(float64)))
			}
		case 8:
			if k == reflect.String {
				arg, err = strconv.ParseInt(arg.(string), 10, 8)
			} else {
				arg = int8(arg.(float64))
			}
		case 16:
			if k == reflect.String {
				arg, err = strconv.ParseInt(arg.(string), 10, 16)
			} else {
				arg = int16(arg.(float64))
			}
		case 32:
			if k == reflect.String {
				arg, err = strconv.ParseInt(arg.(string), 10, 32)
			} else {
				arg = int32(arg.(float64))
			}
		case 64:
			if k == reflect.String {
				arg, err = strconv.ParseInt(arg.(string), 10, 64)
			} else {
				arg = int64(arg.(float64))
			}
		}

		if err != nil {
			return nil, err
		}

	// support both number & string input
	case abi.UintTy:
		var ok bool
		var err error

		k := reflect.TypeOf(arg).Kind()
		if k != reflect.String && k != reflect.Float64 {
			return nil, ErrInvalidArg
		}

		switch t.Size {
		default:
			if k == reflect.String {
				arg, ok = new(big.Int).SetString(arg.(string), 10)
				if !ok {
					return nil, ErrInvalidArg
				}
			} else {
				arg = big.NewInt(int64(arg.(float64)))
			}

			if arg.(*big.Int).Cmp(big.NewInt(0)) < 0 {
				return nil, ErrInvalidArg
			}
		case 8:
			if k == reflect.String {
				arg, err = strconv.ParseUint(arg.(string), 10, 8)
			} else {
				arg = uint8(arg.(float64))
			}
		case 16:
			if k == reflect.String {
				arg, err = strconv.ParseUint(arg.(string), 10, 16)
			} else {
				arg = uint16(arg.(float64))
			}
		case 32:
			if k == reflect.String {
				arg, err = strconv.ParseUint(arg.(string), 10, 32)
			} else {
				arg = uint32(arg.(float64))
			}
		case 64:
			if k == reflect.String {
				arg, err = strconv.ParseUint(arg.(string), 10, 64)
			} else {
				arg = uint64(arg.(float64))
			}
		}

		if err != nil {
			return nil, err
		}

	case abi.BytesTy:
		if reflect.TypeOf(arg).Kind() != reflect.String {
			return nil, ErrInvalidArg
		}

		bytecode, err := decodeBytecode(arg.(string))
		if err != nil {
			return nil, err
		}

		bytes := reflect.MakeSlice(t.Type, 0, len(bytecode))

		for _, oneByte := range bytecode {
			bytes = reflect.Append(bytes, reflect.ValueOf(oneByte))
		}

		arg = bytes.Interface()

	case abi.FixedBytesTy, abi.FunctionTy:
		if reflect.TypeOf(arg).Kind() != reflect.String {
			return nil, ErrInvalidArg
		}

		bytecode, err := decodeBytecode(arg.(string))
		if err != nil {
			return nil, err
		}

		if t.Size != len(bytecode) {
			return nil, ErrInvalidArg
		}

		bytesType := reflect.ArrayOf(t.Size, reflect.TypeOf(uint8(0)))
		bytes := reflect.New(bytesType).Elem()

		for i, oneByte := range bytecode {
			bytes.Index(i).Set(reflect.ValueOf(oneByte))
		}

		arg = bytes.Interface()

	}
	return arg, nil
}
