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
	r.Equal(1, len(abi.Methods))
	r.Equal("recipients", abi.Methods["multiSend"].Inputs[0].Name)
	r.Equal(3, len(abi.Events))
	r.Equal("amount", abi.Events["Transfer"].Inputs[1].Name)
}

func TestParseArguments(t *testing.T) {
	r := require.New(t)
	expect, err := decodeBytecode("0xe3b48f48000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000b9c46db7b8464bad383a595fd7aa7845fbdd642b000000000000000000000000aa77fbf8596e0de5ce362dbd5ab29599a6c38ac4000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000002fa7bc000000000000000000000000000000000000000000000000000000000000007b0000000000000000000000000000000000000000000000000000000000000009504c454153452121210000000000000000000000000000000000000000000000")
	r.NoError(err)

	testAbiFile := "test.abi"
	testAbi, err := readAbiFile(testAbiFile)
	r.NoError(err)

	testMethod := "multiSend"
	testInput := `{"recipients":["io1h8zxmdacge966wp6t90a02ncghaa6eptnftfqr","io14fmlh7zedcx7tn3k9k744v54nxnv8zky86tjhj"],"amounts":[3123132,123],"payload":"PLEASE!!!"}`

	bytecode, err := packArguments(testAbi, testMethod, testInput)
	r.NoError(err)
	fmt.Printf(hex.EncodeToString(bytecode))
	r.True(bytes.Equal(expect, bytecode))
}

// TODO: more tests later
