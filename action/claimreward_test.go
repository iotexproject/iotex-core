package action

import (
	"encoding/hex"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	_defaultGasPrice      = big.NewInt(1000000000000)
	_errNegativeNumberMsg = "negative value"
)

func TestClaimRewardSerialize(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}
	data := rc.Serialize()
	r.NotNil(data)

	rc.amount = big.NewInt(100)
	rc.data = []byte{1}
	data = rc.Serialize()
	r.NotNil(data)
	r.EqualValues("0a03313030120101", hex.EncodeToString(data))
}

func TestClaimRewardIntrinsicGas(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}
	gas, err := rc.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(10000, gas)

	rc.amount = big.NewInt(100000000)
	gas, err = rc.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(10000, gas)

	rc.data = []byte{1}
	gas, err = rc.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(10100, gas)
}

func TestClaimRewardSanityCheck(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}

	rc.amount = big.NewInt(1)
	err := rc.SanityCheck()
	r.NoError(err)

	rc.amount = big.NewInt(-1)
	err = rc.SanityCheck()
	r.NotNil(err)
	r.EqualValues(_errNegativeNumberMsg, err.Error())
}

func TestClaimRewardCost(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}
	rc.gasPrice = _defaultGasPrice
	cost, err := rc.Cost()
	r.Nil(err)
	r.EqualValues("10000000000000000", cost.String())

	rc.amount = big.NewInt(100)
	cost, err = rc.Cost()
	r.Nil(err)
	r.EqualValues("10000000000000000", cost.String())

	rc.data = []byte{1}
	cost, err = rc.Cost()
	r.Nil(err)
	r.EqualValues("10100000000000000", cost.String())
}

func TestClaimRewardEncodeABIBinary(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}
	rc.amount = big.NewInt(101)
	data, err := rc.encodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rc.data = []byte{1, 2, 3}
	data, err = rc.encodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(data),
	)
}

func TestClaimRewardToEthTx(t *testing.T) {
	r := require.New(t)

	rc := &ClaimFromRewardingFund{}
	rc.amount = big.NewInt(101)
	tx, err := rc.ToEthTx(0)
	r.Nil(err)
	r.EqualValues(_rewardingProtocolEthAddr, *tx.To())
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())

	rc.data = []byte{1, 2, 3}
	tx, err = rc.ToEthTx(0)
	r.Nil(err)
	r.EqualValues(_rewardingProtocolEthAddr, *tx.To())
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())
}

func MustNoErrorV[V any](v V, err error) V {
	if err != nil {
		panic(err)
	}
	return v
}

func TestNewRewardingClaimFromABIBinary(t *testing.T) {
	r := require.New(t)

	var (
		method  abi.Method               // abi method
		amount  = big.NewInt(100)        // input amount
		data    = []uint8{'a', 'b', 'c'} // input data
		address = "0x1231231232113"      // input address
		inputs  = abi.Arguments{
			abi.Argument{
				Name:    "amount",
				Type:    MustNoErrorV(abi.NewType("uint256", "uint256", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "data",
				Type:    MustNoErrorV(abi.NewType("uint8[]", "uint8[]", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "address",
				Type:    MustNoErrorV(abi.NewType("string", "string", nil)),
				Indexed: false,
			},
		}
		outputs = abi.Arguments{}
	)

	t.Run("CheckMethodDefine", func(t *testing.T) {
		method = abi.NewMethod("claim", "claim", abi.Function, "nonpayable", false, false, inputs, outputs)
		r.Equal(method, _claimRewardingMethod)
	})

	t.Run("InvalidMethodSignature", func(t *testing.T) {
		input := MustNoErrorV(method.Inputs.Pack(amount, data, address))
		sig := []byte{'1', '2', '3', 4} // invalid
		calldata := append(sig, input...)

		_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.ErrorContains(err, "failed to decode")
	})

	t.Run("MissingSomeArgument", func(t *testing.T) {
		_inputs := _claimRewardingMethod.Inputs
		calldata := append(
			method.ID,
			MustNoErrorV(inputs.Pack(amount, data, address))...,
		)

		for i := 0; i < len(_inputs); i++ {
			old := inputs[i].Name
			_inputs[i].Name = "any"
			_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
			r.ErrorContains(err, "failed to decode")
			_inputs[i].Name = old
		}
	})

	t.Run("Success", func(t *testing.T) {
		calldata := append(method.ID[:], MustNoErrorV(_claimRewardingMethod.Inputs.Pack(amount, data, address))...)
		ret, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.NoError(err)
		r.Equal(ret.Address(), address)
		r.Equal(ret.Amount(), amount)
		r.Equal(ret.Data(), data)
	})
}
