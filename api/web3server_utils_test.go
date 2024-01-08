package api

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestParseCallObject(t *testing.T) {
	require := require.New(t)

	testData := []struct {
		name  string
		input string

		from     string
		to       string
		gasLimit uint64
		gasPrice *big.Int
		value    *big.Int
		data     []byte
		err      error
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
			   },
			   1]}`,
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
			   },
			   1]}`,
			from:     address.ZeroAddress,
			to:       "io10sfcvmuj2000083qqd8d6qg7r457vll9gly090",
			gasLimit: 20000,
			gasPrice: new(big.Int).SetInt64(1000000000000),
			value:    new(big.Int).SetInt64(1),
			data:     []byte{0x6d, 0x4c, 0xe6, 0x3c},
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			in := gjson.Parse(test.input)
			from, to, gasLimit, gasPrice, value, data, err := parseCallObject(&in)
			require.Equal(test.from, from.String())
			require.Equal(test.to, to)
			require.Equal(test.gasLimit, gasLimit)
			require.Equal(test.gasPrice, gasPrice)
			require.Equal(test.value, value)
			require.Equal(test.data, data)
			require.Equal(test.err, err)
		})
	}
}
