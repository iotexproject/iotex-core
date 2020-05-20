// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAbiFile(t *testing.T) {
	r := require.New(t)
	testAbiFile := "test.abi"
	abi, err := readAbiFile(testAbiFile)
	r.NoError(err)
	r.Equal("", abi.Constructor.Name)
	r.Equal(3, len(abi.Methods))
	r.Equal("recipients", abi.Methods["multiSend"].Inputs[0].Name)
}

func TestParseArguments(t *testing.T) {
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
			// IntTy/Uintty larger than int64/uint64 should be passes by string, otherwise the precision losses
			`{"a":["87543498528347976543703735394",123123,5,-12,-134958],"b":[1,2,0]}`,
		}, {
			"0x2db6ad32",
			"testEmpty",
			"",
		},
	}

	for _, test := range tests {
		expect, err := decodeBytecode(test.expectCode)
		r.NoError(err)

		bytecode, err := packArguments(testAbi, test.method, test.inputs)
		r.NoError(err)
		fmt.Println(hex.EncodeToString(bytecode))
		r.True(bytes.Equal(expect, bytecode))
	}
}
