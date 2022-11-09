package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

var _defaultGasPrice = big.NewInt(1000000000000)

func TestRewardingClaimProto(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
	r.EqualValues(big.NewInt(0), rc.Amount())

	p := rc.Proto()
	r.NotNil(p)
	r.IsType(&iotextypes.ClaimFromRewardingFund{}, p)
	r.Nil(p.Data)
	r.EqualValues("", p.Amount)

	rc.amount = big.NewInt(100)
	rc.data = []byte{1}
	p = rc.Proto()
	r.NotNil(p)
	r.EqualValues([]byte{1}, p.Data)
	r.EqualValues("100", p.Amount)
}

func TestRewardingClaimSerialize(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
	data := rc.Serialize()
	r.NotNil(data)
	r.EqualValues([]byte{}, data)

	rc.amount = big.NewInt(100)
	rc.data = []byte{1}
	data = rc.Serialize()
	r.NotNil(data)
	r.EqualValues("0a03313030120101", hex.EncodeToString(data))
}

func TestRewardingClaimIntrinsicGas(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
	gas, err := rc.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10000, gas)

	rc.amount = big.NewInt(100000000)
	gas, err = rc.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10000, gas)

	rc.data = []byte{1}
	gas, err = rc.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10100, gas)
}

func TestRewardingClaimSanityCheck(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
	err := rc.SanityCheck()
	r.NotNil(err)
	r.EqualValues("negative value: invalid amount", err.Error())

	rc.amount = big.NewInt(1)
	err = rc.SanityCheck()
	r.Nil(err)

	rc.amount = big.NewInt(-1)
	err = rc.SanityCheck()
	r.NotNil(err)
	r.EqualValues("negative value: invalid amount", err.Error())
}

func TestRewardingClaimCost(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
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

func TestRewardingClaimEncodeABIBinary(t *testing.T) {
	r := require.New(t)

	rc := &RewardingClaim{}
	data, err := rc.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rc.amount = big.NewInt(101)
	data, err = rc.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rc.data = []byte{1, 2, 3}
	data, err = rc.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(data),
	)
}

func TestRewardingClaimToEthTx(t *testing.T) {
	r := require.New(t)

	rewardingPool, _ := address.FromString(address.RewardingProtocol)
	rewardEthAddr := common.BytesToAddress(rewardingPool.Bytes())

	rc := &RewardingClaim{}
	tx, err := rc.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())

	rc.amount = big.NewInt(101)
	tx, err = rc.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())

	rc.data = []byte{1, 2, 3}
	tx, err = rc.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())
}

func TestNewRewardingClaimFromABIBinary(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("2df163ef000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")

	rc, err := NewRewardingClaimFromABIBinary(data)
	r.Nil(err)
	r.IsType(&RewardingClaim{}, rc)
	r.EqualValues("101", rc.Amount().String())
	r.EqualValues([]byte{1, 2, 3}, rc.Data())
}

func TestRewardingDepositProto(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	r.EqualValues(big.NewInt(0), rp.Amount())

	p := rp.Proto()
	r.NotNil(p)
	r.IsType(&iotextypes.DepositToRewardingFund{}, p)
	r.Nil(p.Data)
	r.EqualValues("", p.Amount)

	rp.amount = big.NewInt(100)
	rp.data = []byte{1}
	p = rp.Proto()
	r.NotNil(p)
	r.EqualValues([]byte{1}, p.Data)
	r.EqualValues("100", p.Amount)
}

func TestRewardingDepositSerialize(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	data := rp.Serialize()
	r.NotNil(data)
	r.EqualValues([]byte{}, data)

	rp.amount = big.NewInt(100)
	rp.data = []byte{1}
	data = rp.Serialize()
	r.NotNil(data)
	r.EqualValues("0a03313030120101", hex.EncodeToString(data))
}

func TestRewardingDepositIntrinsicGas(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	gas, err := rp.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10000, gas)

	rp.amount = big.NewInt(100000000)
	gas, err = rp.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10000, gas)

	rp.data = []byte{1}
	gas, err = rp.IntrinsicGas()
	r.Nil(err)
	r.EqualValues(10100, gas)
}

func TestRewardingDepositSanityCheck(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	err := rp.SanityCheck()
	r.NotNil(err)
	r.EqualValues("negative value: invalid amount", err.Error())

	rp.amount = big.NewInt(1)
	err = rp.SanityCheck()
	r.Nil(err)

	rp.amount = big.NewInt(-1)
	err = rp.SanityCheck()
	r.NotNil(err)
	r.EqualValues("negative value: invalid amount", err.Error())
}

func TestRewardingDepositCost(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	rp.gasPrice = _defaultGasPrice
	cost, err := rp.Cost()
	r.Nil(err)
	r.EqualValues("10000000000000000", cost.String())

	rp.amount = big.NewInt(100)
	cost, err = rp.Cost()
	r.Nil(err)
	r.EqualValues("10000000000000100", cost.String())

	rp.data = []byte{1}
	cost, err = rp.Cost()
	r.Nil(err)
	r.EqualValues("10100000000000100", cost.String())
}

func TestRewardingDepositEncodeABIBinary(t *testing.T) {
	r := require.New(t)

	rp := &RewardingDeposit{}
	data, err := rp.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rp.amount = big.NewInt(101)
	data, err = rp.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(data),
	)

	rp.data = []byte{1, 2, 3}
	data, err = rp.EncodeABIBinary()
	r.Nil(err)
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(data),
	)
}

func TestRewardingDepositToEthTx(t *testing.T) {
	r := require.New(t)

	rewardingPool, _ := address.FromString(address.RewardingProtocol)
	rewardEthAddr := common.BytesToAddress(rewardingPool.Bytes())

	rp := &RewardingDeposit{}
	tx, err := rp.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("0", tx.Value().String())

	rp.amount = big.NewInt(101)
	tx, err = rp.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("101", tx.Value().String())

	rp.data = []byte{1, 2, 3}
	tx, err = rp.ToEthTx()
	r.Nil(err)
	r.EqualValues(rewardEthAddr, *tx.To())
	r.EqualValues(
		"27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003",
		hex.EncodeToString(tx.Data()),
	)
	r.EqualValues("101", tx.Value().String())
}

func TestNewRewardingDepositFromABIBinary(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("27852a6b000000000000000000000000000000000000000000000000000000000000006500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003")

	rp, err := NewRewardingDepositFromABIBinary(data)
	r.Nil(err)
	r.IsType(&RewardingDeposit{}, rp)
	r.EqualValues("101", rp.Amount().String())
	r.EqualValues([]byte{1, 2, 3}, rp.Data())
}
