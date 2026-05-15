package ethabi

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func TestRolldposABI(t *testing.T) {
	r := require.New(t)

	checkUint64 := func(calldata []byte, method *abi.Method, expectParam *protocol.Parameters, value uint64) {
		sc, err := BuildReadStateRequest(append(method.ID, calldata...))
		r.NoError(err)
		r.EqualValues(expectParam.MethodName, sc.Parameters().MethodName)
		r.ElementsMatch(expectParam.Arguments, sc.Parameters().Arguments)

		height := uint64(12345)

		out, err := sc.EncodeToEth(&iotexapi.ReadStateResponse{
			Data: []byte(strconv.FormatUint(value, 10)),
			BlockIdentifier: &iotextypes.BlockIdentifier{
				Height: height,
			},
		})
		r.NoError(err)

		data, err := hex.DecodeString(out)
		r.NoError(err)

		// Unpack as a slice since ABI returns multiple values
		results, err := method.Outputs.Unpack(data)
		r.NoError(err)
		r.Equal(2, len(results))

		resultValue, ok := results[0].(*big.Int)
		r.True(ok)
		resultHeight, ok := results[1].(*big.Int)
		r.True(ok)

		r.Equal(value, resultValue.Uint64())
		r.Equal(height, resultHeight.Uint64())
	}

	t.Run("NumCandidateDelegates", func(t *testing.T) {
		data, err := numCandidateDelegatesMethod.Inputs.Pack()
		r.NoError(err)
		checkUint64(data, numCandidateDelegatesMethod, &protocol.Parameters{MethodName: []byte("NumCandidateDelegates"), Arguments: nil}, 36)
	})

	t.Run("NumDelegates", func(t *testing.T) {
		data, err := numDelegatesMethod.Inputs.Pack()
		r.NoError(err)
		checkUint64(data, numDelegatesMethod, &protocol.Parameters{MethodName: []byte("NumDelegates"), Arguments: nil}, 24)
	})

	t.Run("NumSubEpochs", func(t *testing.T) {
		blockHeight := big.NewInt(1000)
		data, err := numSubEpochsMethod.Inputs.Pack(blockHeight)
		r.NoError(err)
		args := [][]byte{[]byte(blockHeight.Text(10))}
		checkUint64(data, numSubEpochsMethod, &protocol.Parameters{MethodName: []byte("NumSubEpochs"), Arguments: args}, 15)
	})

	t.Run("EpochNumber", func(t *testing.T) {
		blockHeight := big.NewInt(5000)
		data, err := epochNumberMethod.Inputs.Pack(blockHeight)
		r.NoError(err)
		args := [][]byte{[]byte(blockHeight.Text(10))}
		checkUint64(data, epochNumberMethod, &protocol.Parameters{MethodName: []byte("EpochNumber"), Arguments: args}, 14)
	})

	t.Run("EpochHeight", func(t *testing.T) {
		epochNumber := big.NewInt(10)
		data, err := epochHeightMethod.Inputs.Pack(epochNumber)
		r.NoError(err)
		args := [][]byte{[]byte(epochNumber.Text(10))}
		checkUint64(data, epochHeightMethod, &protocol.Parameters{MethodName: []byte("EpochHeight"), Arguments: args}, 3241)
	})

	t.Run("EpochLastHeight", func(t *testing.T) {
		epochNumber := big.NewInt(10)
		data, err := epochLastHeightMethod.Inputs.Pack(epochNumber)
		r.NoError(err)
		args := [][]byte{[]byte(epochNumber.Text(10))}
		checkUint64(data, epochLastHeightMethod, &protocol.Parameters{MethodName: []byte("EpochLastHeight"), Arguments: args}, 3600)
	})

	t.Run("SubEpochNumber", func(t *testing.T) {
		blockHeight := big.NewInt(3300)
		data, err := subEpochNumberMethod.Inputs.Pack(blockHeight)
		r.NoError(err)
		args := [][]byte{[]byte(blockHeight.Text(10))}
		checkUint64(data, subEpochNumberMethod, &protocol.Parameters{MethodName: []byte("SubEpochNumber"), Arguments: args}, 2)
	})
}
