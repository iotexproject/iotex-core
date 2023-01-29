// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func Test_parseOutputArgument(t *testing.T) {
	r := require.New(t)

	bigInt, _ := new(big.Int).SetString("2346783498523230921101011", 10)
	var bytes31 [31]byte
	var bytes24 [24]byte
	copy(bytes31[:], "test byte31313131313131313131")
	copy(bytes24[:], "test function (=24-byte)")

	tests := []struct {
		v          interface{}
		t          string
		components []abi.ArgumentMarshaling
		expect     string
	}{
		{
			int16(-3),
			"int16",
			nil,
			"-3",
		},
		{
			uint64(98237478346),
			"uint64",
			nil,
			"98237478346",
		},
		{
			bigInt,
			"uint233",
			nil,
			"2346783498523230921101011",
		},
		{
			common.HexToAddress("c7F43FaB2ca353d29cE0DA04851aB74f45B09593"),
			"address",
			nil,
			"io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng",
		},
		{
			[]byte("test bytes"),
			"bytes",
			nil,
			"0x74657374206279746573",
		},
		{
			bytes31,
			"bytes31",
			nil,
			"0x74657374206279746533313331333133313331333133313331333133310000",
		},
		{
			[5]string{"IoTeX blockchain", "Raullen", "MenloPark", "2020/06/13", "Frank-is-testing!"},
			"string[5]",
			nil,
			"[IoTeX blockchain Raullen MenloPark 2020/06/13 Frank-is-testing!]",
		},
		{
			[][31]byte{bytes31, bytes31},
			"bytes31[]",
			nil,
			"[0x74657374206279746533313331333133313331333133313331333133310000 0x74657374206279746533313331333133313331333133313331333133310000]",
		},
		{
			struct {
				A string
				B [24]byte
				C []*big.Int
			}{"tuple test!", bytes24, []*big.Int{big.NewInt(-123), bigInt, big.NewInt(0)}},
			"tuple",
			[]abi.ArgumentMarshaling{{Name: "a", Type: "string"}, {Name: "b", Type: "bytes24"}, {Name: "c", Type: "int256[]"}},
			"{a:tuple test! b:0x746573742066756e6374696f6e20283d32342d6279746529 c:[-123 2346783498523230921101011 0]}",
		},
	}

	for _, test := range tests {
		t, err := abi.NewType(test.t, "", test.components)
		r.NoError(err)
		result, ok := parseOutputArgument(test.v, &t)
		r.True(ok)
		r.Equal(test.expect, result)
	}
}

func Test_parseAbi(t *testing.T) {
	r := require.New(t)

	abiBytes := []byte(`[
{
	"constant": false,
    "inputs": [
      {
        "name": "recipients",
        "type": "address[]"
      },
      {
        "name": "amounts",
        "type": "uint256[]"
      },
      {
        "name": "payload",
        "type": "string"
      }
    ],
    "name": "multiSend",
    "outputs": [],
    "payable": true,
    "stateMutability": "payable",
    "type": "function"
}
]`)

	abi, err := parseAbi(abiBytes)
	r.NoError(err)

	r.Len(abi.Methods, 1)
	method, ok := abi.Methods["multiSend"]
	r.True(ok)
	r.False(method.IsConstant())
	r.True(method.IsPayable())
	r.Equal(method.StateMutability, "payable")
	r.Len(method.Inputs, 3)
	r.Equal(method.Inputs[0].Name, "recipients")
	r.Equal(method.Inputs[1].Name, "amounts")
	r.Equal(method.Inputs[2].Name, "payload")
	r.Len(method.Outputs, 0)
}

func Test_parseInput(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		rowInput string
		want     map[string]interface{}
		wantErr  bool
	}{
		{
			`{"name": "Marry"}`,
			map[string]interface{}{
				"name": "Marry",
			},
			false,
		},
		{
			`{"age": 12}`,
			map[string]interface{}{
				"age": float64(12),
			},
			false,
		},
		{
			`{"names": ["marry", "alice"]}`,
			map[string]interface{}{
				"names": []interface{}{"marry", "alice"},
			},
			false,
		},
	}
	for _, tt := range tests {
		got, err := parseInput(tt.rowInput)
		if (err != nil) != tt.wantErr {
			require.FailNow("parseInput() error = %v, wantErr %v", err, tt.wantErr)
		}
		if !reflect.DeepEqual(got, tt.want) {
			require.FailNow("parseInput() = %#v, want %#v", got, tt.want)
		}
	}
}

func Test_parseInputArgument(t *testing.T) {
	require := require.New(t)

	abiType, err := abi.NewType("string[]", "", nil)
	require.NoError(err)

	tests := []struct {
		t       *abi.Type
		arg     interface{}
		want    interface{}
		wantErr bool
	}{
		{
			&abiType,
			[]interface{}{"hello world", "happy holidays"},
			[]string{"hello world", "happy holidays"},
			false,
		},
	}
	for _, tt := range tests {
		got, err := parseInputArgument(tt.t, tt.arg)
		if (err != nil) != tt.wantErr {
			require.FailNow("parseInputArgument() error = %v, wantErr %v", err, tt.wantErr)
		}
		if !reflect.DeepEqual(got, tt.want) {
			require.FailNow("parseInputArgument() = %#v, want %#v", got, tt.want)
		}
	}
}
