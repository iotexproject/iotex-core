package api

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseCallObject(t *testing.T) {
	require := require.New(t)

	testData := []struct {
		name  string
		input string

		from      string
		to        string
		gasLimit  uint64
		gasPrice  *big.Int
		gasTipCap *big.Int
		gasFeeCap *big.Int
		value     *big.Int
		data      []byte
		acl       types.AccessList
		err       error
	}{
		{
			name: "legacy",
			input: `{"params":[{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"data":     "0x6d4ce63c"
			   }, "0x1"]}`,
			from:     address.ZeroAddress,
			to:       "io10sfcvmuj2000083qqd8d6qg7r457vll9gly090",
			gasLimit: 20000,
			gasPrice: new(big.Int).SetInt64(1000000000000),
			value:    new(big.Int).SetInt64(1),
			data:     []byte{0x6d, 0x4c, 0xe6, 0x3c},
		},
		{
			name: "input instead of data",
			input: `{"params":[{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"input":     "0x6d4ce63c"
			   }, "0x1"]}`,
			from:     address.ZeroAddress,
			to:       "io10sfcvmuj2000083qqd8d6qg7r457vll9gly090",
			gasLimit: 20000,
			gasPrice: new(big.Int).SetInt64(1000000000000),
			value:    new(big.Int).SetInt64(1),
			data:     []byte{0x6d, 0x4c, 0xe6, 0x3c},
		},
		{
			name: "dynamicfee",
			input: `{"params":[{
				"from":     "0x1a2f3b98e2f5a0f9f9f3f3f3f3f3f3f3f3f3f3f3",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"maxFeePerGas": "0xe8d4a51000",
				"maxPriorityFeePerGas": "0xd4a51000",
				"value":    "0x1",
				"input":    "0x6d4ce63c",
				"accessList": [
					{
						"address": "0x1a2f3b98e2f5a0f9f9f3f3f3f3f3f3f3f3f3f3f3",
						"storageKeys": ["0x0000000000000000000000001a2f3b98e2f5a0f9f9f3f3f3f3f3f3f3f3f3f3f3"]
					}
				]
			   }, "0x1"]}`,
			from:      "io1rghnhx8z7ks0n70n70el8uln70el8ulnp8hq9l",
			to:        "io10sfcvmuj2000083qqd8d6qg7r457vll9gly090",
			gasLimit:  20000,
			gasPrice:  new(big.Int).SetInt64(0),
			gasTipCap: new(big.Int).SetInt64(0xd4a51000),
			gasFeeCap: new(big.Int).SetInt64(0xe8d4a51000),
			value:     new(big.Int).SetInt64(1),
			data:      []byte{0x6d, 0x4c, 0xe6, 0x3c},
			acl: types.AccessList{
				{
					Address:     common.HexToAddress("0x1a2f3b98e2f5a0f9f9f3f3f3f3f3f3f3f3f3f3f3"),
					StorageKeys: []common.Hash{common.HexToHash("0x0000000000000000000000001a2f3b98e2f5a0f9f9f3f3f3f3f3f3f3f3f3f3f3")},
				},
			},
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			in := gjson.Parse(test.input)
			callMsg, err := parseCallObject(&in)
			require.Equal(test.from, callMsg.From.String())
			require.Equal(test.to, callMsg.To)
			require.Equal(test.gasLimit, callMsg.Gas)
			require.Equal(test.gasPrice, callMsg.GasPrice)
			require.Equal(test.gasTipCap, callMsg.GasTipCap)
			require.Equal(test.gasFeeCap, callMsg.GasFeeCap)
			require.Equal(test.value, callMsg.Value)
			require.Equal(test.data, callMsg.Data)
			require.Equal(test.acl, callMsg.AccessList)
			require.Equal(test.err, err)
		})
	}

	t.Run("parse gas price", func(t *testing.T) {
		input := `{"params":[{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "unknown",
				"value":    "0x1",
				"input":     "0x6d4ce63c"
			   },
			   1]}`
		in := gjson.Parse(input)
		_, err := parseCallObject(&in)
		require.EqualError(err, "gasPrice: unknown: wrong type of params")
	})

	t.Run("parse value", func(t *testing.T) {
		input := `{"params":[{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "unknown",
				"input":     "0x6d4ce63c"
			   },
			   1]}`
		in := gjson.Parse(input)
		_, err := parseCallObject(&in)
		require.EqualError(err, "value: unknown: wrong type of params")
	})

}

func TestParseBlockNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	t.Run("earliest block number", func(t *testing.T) {
		num, _ := web3svr.parseBlockNumber("earliest")
		require.Equal(num, uint64(0x1))
	})

	t.Run("pending block number", func(t *testing.T) {
		core.EXPECT().TipHeight().Return(uint64(0x1))
		num, _ := web3svr.parseBlockNumber("pending")
		require.Equal(num, uint64(0x1))
	})

	t.Run("latest block number", func(t *testing.T) {
		core.EXPECT().TipHeight().Return(uint64(0x1))
		num, _ := web3svr.parseBlockNumber("latest")
		require.Equal(num, uint64(0x1))
	})

	t.Run("nil block number", func(t *testing.T) {
		core.EXPECT().TipHeight().Return(uint64(0x1))
		num, _ := web3svr.parseBlockNumber("")
		require.Equal(num, uint64(0x1))
	})
}
