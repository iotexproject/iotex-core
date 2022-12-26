package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
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

	rewardingPool, _ := address.FromString(address.RewardingProtocol)
	rewardEthAddr := common.BytesToAddress(rewardingPool.Bytes())

	rc := &ClaimFromRewardingFund{}

	rc.amount = big.NewInt(101)
	tx, err := rc.ToEthTx()
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

	rc, err := NewClaimFromRewardingFundFromABIBinary(data)
	r.Nil(err)
	r.IsType(&ClaimFromRewardingFund{}, rc)
	r.EqualValues("101", rc.Amount().String())
	r.EqualValues([]byte{1, 2, 3}, rc.Data())
}
