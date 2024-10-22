package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
)

func TestCandidateTransferOwnership(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		nonce        uint64
		gasLimit     uint64
		gasPrice     *big.Int
		newOwner     string
		payload      []byte
		intrinsicGas uint64
		cost         string
		serialize    string
		expected     error
		sanityCheck  error
	}{
		// valid test
		{
			1,
			1000000,
			big.NewInt(1000),
			"io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he",
			[]byte("payload"),
			10700,
			"10700000",
			"0a29696f3130613239387a6d7a7672743467757137396139663478377165646a353979376572793834686512077061796c6f6164",
			nil,
			nil,
		},
		//invalid address
		{
			1,
			1000000,
			big.NewInt(1000),
			"ab-10",
			[]byte("payload"),
			0,
			"",
			"",
			address.ErrInvalidAddr,
			nil,
		},
		//invalid gas price
		{
			1,
			1000000,
			big.NewInt(-1000),
			"io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he",
			nil,
			0,
			"",
			"",
			nil,
			ErrNegativeValue,
		},
	}
	for _, test := range tests {
		cr, err := NewCandidateTransferOwnership(test.newOwner, test.payload)
		require.Equal(test.expected, errors.Cause(err))
		if err != nil {
			continue
		}
		elp := (&EnvelopeBuilder{}).SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(cr).Build()
		err = elp.SanityCheck()
		require.Equal(test.sanityCheck, errors.Cause(err))
		if err != nil {
			continue
		}

		require.Equal(test.serialize, hex.EncodeToString(cr.Serialize()))
		require.Equal(test.gasLimit, elp.Gas())
		require.Equal(test.gasPrice, elp.GasPrice())
		require.Equal(test.nonce, elp.Nonce())
		require.Equal(test.newOwner, cr.NewOwner().String())
		require.Equal(test.payload, cr.Payload())

		gas, err := cr.IntrinsicGas()
		require.NoError(err)
		require.Equal(test.intrinsicGas, gas)
		cost, err := elp.Cost()
		require.NoError(err)
		require.Equal(test.cost, cost.Text(10))

		cr2 := &CandidateTransferOwnership{}
		require.NoError(cr2.LoadProto(cr.Proto()))
		require.Equal(test.newOwner, cr2.NewOwner().String())
		require.Equal(test.payload, cr2.Payload())

	}
}

func TestCandidateTransferOwnershipABIEncodeAndDecode(t *testing.T) {
	require := require.New(t)
	cr, err := NewCandidateTransferOwnership("io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", []byte("payload"))
	require.NoError(err)
	enc, err := cr.EthData()
	require.NoError(err)

	cr2, err := NewCandidateTransferOwnershipFromABIBinary(enc)
	require.NoError(err)
	require.Equal(cr.NewOwner().String(), cr2.NewOwner().String())
	require.Equal(cr.Payload(), cr2.Payload())

	cr2.newOwner = nil
	enc, err = cr2.EthData()
	require.Equal(ErrAddress, errors.Cause(err))
	require.Nil(enc)

	//invalid data
	data := []byte{1, 2, 3, 4}
	cr2, err = NewCandidateTransferOwnershipFromABIBinary(data)
	require.Equal(errDecodeFailure, err)
	require.Nil(cr2)
}
