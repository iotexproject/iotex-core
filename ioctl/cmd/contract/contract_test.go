// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestParseAbiFile(t *testing.T) {
	r := require.New(t)
	testAbiFile := "test.abi"
	abi, err := readAbiFile(testAbiFile)
	r.NoError(err)
	r.Equal("", abi.Constructor.Name)
	r.Equal(10, len(abi.Methods))
	r.Equal("recipients", abi.Methods["multiSend"].Inputs[0].Name)
}

func TestParseInput(t *testing.T) {
	r := require.New(t)

	testAbiFile := "test.abi"
	testAbi, err := readAbiFile(testAbiFile)
	r.NoError(err)

	tests := []struct {
		expectCode string
		method     string
		inputs     string
	}{
		{
			"0xe3b48f48000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000b9c46db7b8464bad383a595fd7aa7845fbdd642b000000000000000000000000aa77fbf8596e0de5ce362dbd5ab29599a6c38ac4000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000002fa7bc000000000000000000000000000000000000000000000000000000000000007b0000000000000000000000000000000000000000000000000000000000000009504c454153452121210000000000000000000000000000000000000000000000",
			"multiSend",
			`{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":["3123132","123"],"payload":"PLEASE!!!"}`,
		}, {
			"0xba025b7100000000000000000000000000000000000000011ade48e4922161024e211c62000000000000000000000000000000000000000000000000000000000001e0f30000000000000000000000000000000000000000000000000000000000000005fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff4fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffdf0d2000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000",
			"testArray",
			// IntTy/UintTy larger than int64/uint64 should be passes by string, otherwise the precision losses
			`{"a":["87543498528347976543703735394",123123,5,-12,-134958],"b":[1,2,0]}`,
		}, {
			"0x3ca8b1a70000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000001007a675668798ab73482982748287842900000000000000000000000000000000",
			"testBytes",
			`{"t":"0x07a675668798ab734829827482878429"}`,
		}, {
			"0x901e5dda1221120000000000000000000000000000000000000000000000000000000000",
			"testFixedBytes",
			`{"t":"0x122112"}`,
		}, {
			"0x1bfc56c6000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000010496f54655820426c6f636b636861696e00000000000000000000000000000000",
			"testBoolAndString",
			`{"a":true,"s":"IoTeX Blockchain"}`,
		},
	}

	for _, test := range tests {
		expect, err := decodeBytecode(test.expectCode)
		r.NoError(err)

		bytecode, err := packArguments(testAbi, test.method, test.inputs)
		r.NoError(err)

		r.True(bytes.Equal(expect, bytecode))
	}
}

func TestParseOutput(t *testing.T) {
	r := require.New(t)

	testAbiFile := "test.abi"
	testAbi, err := readAbiFile(testAbiFile)
	r.NoError(err)

	tests := []struct {
		expectResult string
		method       string
		outputs      string
	}{
		{
			"0",
			"minTips",
			"0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			"io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng",
			"owner",
			"000000000000000000000000c7f43fab2ca353d29ce0da04851ab74f45b09593",
		},
		{
			"Hello World",
			"getMessage",
			"0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000b48656c6c6f20576f726c64000000000000000000000000000000000000000000",
		},
		{
			"{i:17 abc:[io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7 io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng]}",
			"testTuple",
			"00000000000000000000000000000000000000000000000000000000000000110000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c7f43fab2ca353d29ce0da04851ab74f45b09593",
		},
	}

	for _, test := range tests {
		v, err := parseOutput(testAbi, test.method, test.outputs)
		r.NoError(err)
		r.Equal(test.expectResult, fmt.Sprint(v))
	}
}

func TestParseOutputArgument(t *testing.T) {
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
		t, err := abi.NewType(test.t, test.components)
		r.NoError(err)
		result, ok := parseOutputArgument(test.v, &t)
		r.True(ok)
		r.Equal(test.expect, result)
	}
}
