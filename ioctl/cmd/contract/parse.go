// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

// ErrInvalidArg indicates argument is invalid
var (
	ErrInvalidArg = errors.New("invalid argument")
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

func parseOutput(targetAbi *abi.ABI, targetMethod string, result string) (string, error) {
	resultBytes, err := hex.DecodeString(result)
	if err != nil {
		return "", output.NewError(output.ConvertError, "failed to decode result", err)
	}

	var (
		outputArgs = targetAbi.Methods[targetMethod].Outputs
		tupleStr   = make([]string, 0, len(outputArgs))
	)

	v, err := targetAbi.Unpack(targetMethod, resultBytes)
	if err != nil {
		return "", output.NewError(output.SerializationError, "failed to parse output", err)
	}

	if len(outputArgs) == 1 {
		elemStr, _ := parseOutputArgument(v[0], &outputArgs[0].Type)
		return elemStr, nil
	}

	for i, field := range v {
		elemStr, _ := parseOutputArgument(field, &outputArgs[i].Type)
		tupleStr = append(tupleStr, outputArgs[i].Name+":"+elemStr)
	}
	return "{" + strings.Join(tupleStr, " ") + "}", nil
}

// parseInputArgument parses input's argument as golang variable
func parseInputArgument(t *abi.Type, arg interface{}) (interface{}, error) {
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

		slice := reflect.MakeSlice(t.GetType(), 0, t.Size)

		s := reflect.ValueOf(arg)
		for i := 0; i < s.Len(); i++ {
			ele, err := parseInputArgument(t.Elem, s.Index(i).Interface())
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

		arrayType := reflect.ArrayOf(t.Size, t.Elem.GetType())
		array := reflect.New(arrayType).Elem()

		s := reflect.ValueOf(arg)
		for i := 0; i < s.Len(); i++ {
			ele, err := parseInputArgument(t.Elem, s.Index(i).Interface())
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
			arg, err = addrutil.IoAddrToEvmAddr(addrString)
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

		var value int64
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
				value, err = strconv.ParseInt(arg.(string), 10, 8)
				arg = int8(value)
			} else {
				arg = int8(arg.(float64))
			}
		case 16:
			if k == reflect.String {
				value, err = strconv.ParseInt(arg.(string), 10, 16)
				arg = int16(value)
			} else {
				arg = int16(arg.(float64))
			}
		case 32:
			if k == reflect.String {
				value, err = strconv.ParseInt(arg.(string), 10, 32)
				arg = int32(value)
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

		var value uint64
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
				value, err = strconv.ParseUint(arg.(string), 10, 8)
				arg = uint8(value)
			} else {
				arg = uint8(arg.(float64))
			}
		case 16:
			if k == reflect.String {
				value, err = strconv.ParseUint(arg.(string), 10, 16)
				arg = uint16(value)
			} else {
				arg = uint16(arg.(float64))
			}
		case 32:
			if k == reflect.String {
				value, err = strconv.ParseUint(arg.(string), 10, 32)
				arg = uint32(value)
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

		bytes := reflect.MakeSlice(t.GetType(), 0, len(bytecode))

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

// parseOutputArgument parses output's argument as human-readable string
func parseOutputArgument(v interface{}, t *abi.Type) (string, bool) {
	str := fmt.Sprint(v)
	ok := false

	switch t.T {
	case abi.StringTy, abi.BoolTy:
		// case abi.StringTy & abi.BoolTy can be handled by fmt.Sprint()
		ok = true

	case abi.TupleTy:
		if reflect.TypeOf(v).Kind() == reflect.Struct {
			ok = true

			tupleStr := make([]string, 0, len(t.TupleElems))
			for i, elem := range t.TupleElems {
				elemStr, elemOk := parseOutputArgument(reflect.ValueOf(v).Field(i).Interface(), elem)
				tupleStr = append(tupleStr, t.TupleRawNames[i]+":"+elemStr)
				ok = ok && elemOk
			}

			str = "{" + strings.Join(tupleStr, " ") + "}"
		}

	case abi.SliceTy, abi.ArrayTy:
		if reflect.TypeOf(v).Kind() == reflect.Slice || reflect.TypeOf(v).Kind() == reflect.Array {
			ok = true

			value := reflect.ValueOf(v)
			sliceStr := make([]string, 0, value.Len())
			for i := 0; i < value.Len(); i++ {
				elemStr, elemOk := parseOutputArgument(value.Index(i).Interface(), t.Elem)
				sliceStr = append(sliceStr, elemStr)
				ok = ok && elemOk
			}

			str = "[" + strings.Join(sliceStr, " ") + "]"
		}

	case abi.IntTy, abi.UintTy:
		if reflect.TypeOf(v) == reflect.TypeOf(big.NewInt(0)) {
			var bigInt *big.Int
			bigInt, ok = v.(*big.Int)
			if ok {
				str = bigInt.String()
			}
		} else if 2 <= reflect.TypeOf(v).Kind() && reflect.TypeOf(v).Kind() <= 11 {
			// other integer types (int8,uint16,...) can be handled by fmt.Sprint(v)
			ok = true
		}

	case abi.AddressTy:
		if reflect.TypeOf(v) == reflect.TypeOf(common.Address{}) {
			var ethAddr common.Address
			ethAddr, ok = v.(common.Address)
			if ok {
				ioAddress, err := address.FromBytes(ethAddr.Bytes())
				if err == nil {
					str = ioAddress.String()
				}
			}
		}

	case abi.BytesTy:
		if reflect.TypeOf(v) == reflect.TypeOf([]byte{}) {
			var bytes []byte
			bytes, ok = v.([]byte)
			if ok {
				str = "0x" + hex.EncodeToString(bytes)
			}
		}

	case abi.FixedBytesTy, abi.FunctionTy:
		if reflect.TypeOf(v).Kind() == reflect.Array && reflect.TypeOf(v).Elem() == reflect.TypeOf(byte(0)) {
			bytesValue := reflect.ValueOf(v)
			byteSlice := reflect.MakeSlice(reflect.TypeOf([]byte{}), bytesValue.Len(), bytesValue.Len())
			reflect.Copy(byteSlice, bytesValue)

			str = "0x" + hex.EncodeToString(byteSlice.Bytes())
			ok = true
		}
	}

	return str, ok
}
