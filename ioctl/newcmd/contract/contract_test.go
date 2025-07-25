// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewContractCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("contract", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any())
	client.EXPECT().SetInsecureWithFlag(gomock.Any())

	cmd := NewContractCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "Available Commands")
}

func TestReadAbiFile(t *testing.T) {
	require := require.New(t)
	testAbiFile := "test.abi"
	abi, err := readAbiFile(testAbiFile)
	require.NoError(err)
	require.Equal("", abi.Constructor.Name)
	require.Len(abi.Methods, 10)
	require.Equal("recipients", abi.Methods["multiSend"].Inputs[0].Name)
}

func TestParseInput(t *testing.T) {
	require := require.New(t)

	testAbiFile := "test.abi"
	testAbi, err := readAbiFile(testAbiFile)
	require.NoError(err)

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
		require.NoError(err)

		bytecode, err := packArguments(testAbi, test.method, test.inputs)
		require.NoError(err)

		require.True(bytes.Equal(expect, bytecode))
	}
}

func TestParseOutput(t *testing.T) {
	require := require.New(t)

	testAbiFile := "test.abi"
	testAbi, err := readAbiFile(testAbiFile)
	require.NoError(err)

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
		require.NoError(err)
		require.Equal(test.expectResult, fmt.Sprint(v))
	}
}
