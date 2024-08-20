package action

import (
	"math/big"
	"testing"
	_ "unsafe"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/util/assertions"
)

func TestClaimRewardIntrinsicGas(t *testing.T) {
	r := require.New(t)

	builder := &ClaimFromRewardingFundBuilder{}

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

	builder := &ClaimFromRewardingFundBuilder{}

	builder.SetAmount(big.NewInt(1))
	rc := builder.Build()
	r.NoError(rc.SanityCheck())

	builder.Reset()
	builder.SetAmount(big.NewInt(-1))
	rc = builder.Build()
	err := rc.SanityCheck()
	r.ErrorIs(err, ErrNegativeValue)
}

func TestClaimRewardCost(t *testing.T) {
	r := require.New(t)

	builder := &ClaimFromRewardingFundBuilder{}

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

func TestNewRewardingClaimFromABIBinary(t *testing.T) {
	r := require.New(t)

	var (
		method abi.Method                                    // abi method
		amount = big.NewInt(100)                             // input amount
		data   = []uint8{'a', 'b', 'c'}                      // input data
		addr   = "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he" // input address
		inputs = abi.Arguments{
			abi.Argument{
				Name:    "amount",
				Type:    assertions.MustNoErrorV(abi.NewType("uint256", "uint256", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "address",
				Type:    assertions.MustNoErrorV(abi.NewType("string", "string", nil)),
				Indexed: false,
			},
			abi.Argument{
				Name:    "data",
				Type:    assertions.MustNoErrorV(abi.NewType("uint8[]", "uint8[]", nil)),
				Indexed: false,
			},
		}
		outputs = abi.Arguments{}
	)

	t.Run("CheckMethodDefine", func(t *testing.T) {
		method = abi.NewMethod("claimFor", "claimFor", abi.Function, "nonpayable", false, false, inputs, outputs)
		r.Equal(method, _claimRewardingMethodV2)
	})

	t.Run("InvalidMethodSignature", func(t *testing.T) {
		input := assertions.MustNoErrorV(method.Inputs.Pack(amount, addr, data))
		methodsig := []byte{'1', '2', '3', 4} // invalid
		calldata := append(methodsig, input...)

		_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.Equal(errWrongMethodSig, err)
	})

	t.Run("MissingSomeArgument", func(t *testing.T) {
		_inputs := _claimRewardingMethodV2.Inputs
		calldata := append(
			method.ID,
			assertions.MustNoErrorV(inputs.Pack(amount, addr, data))...,
		)

		for i := 0; i < len(_inputs); i++ {
			old := inputs[i].Name
			_inputs[i].Name = "any"
			_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
			r.Equal(errDecodeFailure, err)
			_inputs[i].Name = old
		}
	})

	t.Run("EmptyAddress", func(t *testing.T) {
		calldata := append(
			method.ID[:],
			assertions.MustNoErrorV(method.Inputs.Pack(amount, "", data))...)
		_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.ErrorContains(err, "address is empty")
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		calldata := append(
			method.ID[:],
			assertions.MustNoErrorV(method.Inputs.Pack(amount, "0x1231231232113", data))...)
		_, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.Equal(address.ErrInvalidAddr, errors.Cause(err))
	})

	t.Run("Success", func(t *testing.T) {
		calldata := append(
			method.ID[:],
			assertions.MustNoErrorV(method.Inputs.Pack(amount, addr, data))...)
		ret, err := NewClaimFromRewardingFundFromABIBinary(calldata)
		r.NoError(err)
		r.Equal(ret.Address().String(), addr)
		r.Equal(ret.Amount(), amount)
		r.Equal(ret.Data(), data)
	})
}

func TestClaimFromRewardingFund(t *testing.T) {
	r := require.New(t)
	c := &ClaimFromRewardingFund{}

	t.Run("InvalidAmountProtoValue", func(t *testing.T) {
		p := &iotextypes.ClaimFromRewardingFund{
			Amount: "0xz100", // invalid amount proto value
		}
		err := c.LoadProto(p)
		r.ErrorContains(err, "failed to set claim amount")
	})

	t.Run("InvalidAddressProtoValue", func(t *testing.T) {
		p := &iotextypes.ClaimFromRewardingFund{
			Amount:  "100", // invalid amount proto value
			Address: "0x123",
		}
		err := c.LoadProto(p)
		r.Equal(errors.Cause(err), address.ErrInvalidAddr)
	})

	t.Run("FromProto", func(t *testing.T) {
		p := &iotextypes.ClaimFromRewardingFund{
			Amount:  "100",
			Data:    []byte("abc"),
			Address: "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he",
		}
		for i := 0; i < 2; i++ {
			if i == 1 {
				// test empty address
				p.Address = ""
			}
			err := c.LoadProto(p)
			r.NoError(err)
			r.Equal(c.Amount(), assertions.MustBeTrueV(new(big.Int).SetString(p.Amount, 10)))
			r.Equal(c.Data(), p.Data)
			if i == 0 {
				r.Equal(c.Address().String(), p.Address)
			} else {
				r.Nil(c.Address())
			}
			r.Equal(c.Proto(), p)
			r.NoError(c.SanityCheck())

			intrinsicGas, err := c.IntrinsicGas()
			r.NoError(err)

			cost, err := c.Cost()
			r.Equal(cost, new(big.Int).Mul(c.GasPrice(), new(big.Int).SetUint64(intrinsicGas)))
		}
	})

	t.Run("FromBuilder", func(t *testing.T) {
		var addr address.Address
		for i := 0; i < 2; i++ {
			if i == 1 {
				addr, _ = address.FromString("io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he")
			}
			builder := &ClaimFromRewardingFundBuilder{}
			builder.SetAmount(big.NewInt(205))
			builder.SetData([]byte("abc"))
			builder.SetAddress(addr)
			builder.SetVersion(0)
			c2 := builder.Build()
			if i == 0 {
				r.Nil(c2.Address())
			} else {
				r.Equal(c2.Address().String(), "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he")
			}
			r.Equal(c2.Data(), []byte("abc"))
			r.Equal(c2.Amount().String(), "205")
		}
	})
}
