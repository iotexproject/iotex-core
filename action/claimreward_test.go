package action_test

import (
	"math"
	"math/big"
	"testing"
	_ "unsafe"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/assertions"
)

//go:linkname _claimRewardingMethod github.com/iotexproject/iotex-core/action._claimRewardingMethod
var _claimRewardingMethod abi.Method

//go:linkname _rewardingProtocolEthAddr github.com/iotexproject/iotex-core/action._rewardingProtocolEthAddr
var _rewardingProtocolEthAddr common.Address

func TestClaimRewardIntrinsicGas(t *testing.T) {
	r := require.New(t)

	builder := &action.ClaimFromRewardingFundBuilder{}

	rc := builder.Build()
	gas, err := rc.IntrinsicGas()
	r.NoError(err)
	r.Equal(uint64(10000), gas)

	builder.Reset()
	builder.SetAmount(big.NewInt(100000000))
	rc = builder.Build()
	gas, err = rc.IntrinsicGas()
	r.NoError(err)
	r.Equal(uint64(10000), gas)

	builder.Reset()
	builder.SetAmount(big.NewInt(100000000))
	builder.SetData([]byte{1})
	rc = builder.Build()
	gas, err = rc.IntrinsicGas()
	r.NoError(err)
	r.Equal(uint64(10100), gas)
}

func TestClaimRewardSanityCheck(t *testing.T) {
	r := require.New(t)

	builder := &action.ClaimFromRewardingFundBuilder{}

	builder.SetAmount(big.NewInt(1))
	rc := builder.Build()
	r.NoError(rc.SanityCheck())

	builder.Reset()
	builder.SetAmount(big.NewInt(-1))
	rc = builder.Build()
	err := rc.SanityCheck()
	r.ErrorIs(err, action.ErrNegativeValue)
}

func TestClaimRewardCost(t *testing.T) {
	r := require.New(t)

	builder := &action.ClaimFromRewardingFundBuilder{}

	builder.SetGasPrice(big.NewInt(1000000000000))
	rc := builder.Build()
	cost, err := rc.Cost()
	r.NoError(err)
	r.Equal("10000000000000000", cost.String())

	builder.Reset()
	builder.SetGasPrice(big.NewInt(1000000000000))
	builder.SetAmount(big.NewInt(100))
	rc = builder.Build()
	cost, err = rc.Cost()
	r.NoError(err)
	r.Equal("10000000000000000", cost.String())

	builder.Reset()
	builder.SetGasPrice(big.NewInt(1000000000000))
	builder.SetAmount(big.NewInt(100))
	builder.SetData([]byte{1})
	rc = builder.Build()
	cost, err = rc.Cost()
	r.NoError(err)
	r.Equal("10100000000000000", cost.String())
}

func TestClaimRewardToEthTx(t *testing.T) {
	r := require.New(t)

	builder := &action.ClaimFromRewardingFundBuilder{}

	builder.SetAmount(big.NewInt(101))
	rc := builder.Build()
	tx, err := rc.ToEthTx(0)
	r.NoError(err)
	r.Equal(tx.To().String(), _rewardingProtocolEthAddr.String())
	r.Equal(tx.Data()[:4], _claimRewardingMethod.ID)
	r.Equal(tx.Value().String(), "0")

	builder.Reset()
	builder.SetAmount(big.NewInt(101))
	builder.SetData([]byte{1, 2, 3})
	builder.SetAddress("0x1")
	rc = builder.Build()
	tx, err = rc.ToEthTx(0)
	r.NoError(err)
	r.Equal(tx.Data()[:4], _claimRewardingMethod.ID)
	r.Equal(tx.Value().String(), "0")
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
				Type:    assertions.MustNoErrorV(abi.NewType("uint256", "uint256", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "data",
				Type:    assertions.MustNoErrorV(abi.NewType("uint8[]", "uint8[]", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "address",
				Type:    assertions.MustNoErrorV(abi.NewType("string", "string", nil)),
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
		input := assertions.MustNoErrorV(method.Inputs.Pack(amount, data, address))
		methodsig := []byte{'1', '2', '3', 4} // invalid
		calldata := append(methodsig, input...)

		_, err := action.NewClaimFromRewardingFundFromABIBinary(calldata)
		r.ErrorContains(err, "failed to decode")
	})

	t.Run("MissingSomeArgument", func(t *testing.T) {
		_inputs := _claimRewardingMethod.Inputs
		calldata := append(
			method.ID,
			assertions.MustNoErrorV(inputs.Pack(amount, data, address))...,
		)

		for i := 0; i < len(_inputs); i++ {
			old := inputs[i].Name
			_inputs[i].Name = "any"
			_, err := action.NewClaimFromRewardingFundFromABIBinary(calldata)
			r.ErrorContains(err, "failed to decode")
			_inputs[i].Name = old
		}
	})

	t.Run("Success", func(t *testing.T) {
		calldata := append(
			method.ID[:],
			assertions.MustNoErrorV(method.Inputs.Pack(amount, data, address))...)
		ret, err := action.NewClaimFromRewardingFundFromABIBinary(calldata)
		r.NoError(err)
		r.Equal(ret.Address(), address)
		r.Equal(ret.Amount(), amount)
		r.Equal(ret.Data(), data)
	})
}

/*
	func TestClaimFromRewardingFund(t *testing.T) {
		b := ClaimFromRewardingFundBuilder{}
		s1 := b.SetAmount(big.NewInt(1)).
			SetData([]byte{2}).
			Build()
		proto := s1.Proto()
		s2 := ClaimFromRewardingFund{}
		require.NoError(t, s2.LoadProto(proto))
		assert.Equal(t, s1.Amount(), s2.Amount())
		assert.Equal(t, s2.Data(), s2.Data())
	}
*/
func TestClaimFromRewardingFund(t *testing.T) {
	r := require.New(t)
	c := &action.ClaimFromRewardingFund{}

	t.Run("InvalidAmountProtoValue", func(t *testing.T) {
		p := &iotextypes.ClaimFromRewardingFund{
			Amount: "0xz100", // invalid amount proto value
		}
		err := c.LoadProto(p)
		r.ErrorContains(err, "failed to set claim amount")
	})

	t.Run("Success", func(t *testing.T) {
		p := &iotextypes.ClaimFromRewardingFund{
			Amount:  "100",
			Data:    []byte("abc"),
			Address: "0x123",
		}
		t.Run("FromProto", func(t *testing.T) {
			err := c.LoadProto(p)
			r.NoError(err)
			r.Equal(c.Amount(), assertions.MustBeTrueV(new(big.Int).SetString(p.Amount, 10)))
			r.Equal(c.Data(), p.Data)
			r.Equal(c.Address(), p.Address)
			r.Equal(c.Proto(), p)
			r.Equal(c.Serialize(), assertions.MustNoErrorV(proto.Marshal(p)))
			r.NoError(c.SanityCheck())

			intrinsicGas, err := c.IntrinsicGas()
			r.NoError(err)

			cost, err := c.Cost()
			r.Equal(cost, new(big.Int).Mul(c.GasPrice(), new(big.Int).SetUint64(intrinsicGas)))
		})

		t.Run("FromBuilder", func(t *testing.T) {
			builder := &action.ClaimFromRewardingFundBuilder{}
			builder.SetAmount(assertions.MustBeTrueV(new(big.Int).SetString(p.Amount, 10)))
			builder.SetData(p.Data)
			builder.SetAddress(p.Address)
			builder.SetVersion(0)
			c2 := builder.Build()
			r.Equal(c2.Address(), c.Address())
			r.Equal(c2.Data(), c.Data())
			r.Equal(c2.Amount(), c.Amount())
		})

		t.Run("FailedToCalIntrinsicGas", func(t *testing.T) {
			t.Skip("required data size over max length can make")
			size := (math.MaxUint64 - action.ClaimFromRewardingFundBaseGas) / action.ClaimFromRewardingFundGasPerByte
			p := &iotextypes.ClaimFromRewardingFund{
				Data: make([]byte, size+1),
			}
			err := c.LoadProto(p)
			r.NoError(err)
			_, err = c.IntrinsicGas()
			r.ErrorIs(err, action.ErrInsufficientFunds)
		})
	})
}
