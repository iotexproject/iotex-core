package ethabi

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

var errInvalidMsg = "address length = 40, expecting 41: invalid address"

func TestBuildReadStateRequestError(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("8ae8a8a4")
	req, err := BuildReadStateRequest(data)

	r.Nil(req)
	r.EqualValues("invalid call sig", err.Error())
}

func TestBuildReadStateRequestInvalid(t *testing.T) {
	r := require.New(t)

	data, _ := hex.DecodeString("8ae8a8")
	req, err := BuildReadStateRequest(data)

	r.Nil(req)
	r.EqualValues("invalid call binary data", err.Error())
}

func TestEncodeCandidateToEth(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "10000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(err)
	r.EqualValues("0x0000000000000000000000000000000000000001", cand.OwnerAddress.Hex())
	r.EqualValues("0x0000000000000000000000000000000000000002", cand.OperatorAddress.Hex())
	r.EqualValues("0x0000000000000000000000000000000000000003", cand.RewardAddress.Hex())
	r.EqualValues("hello", cand.Name)
	r.EqualValues("10000000000000000000", cand.TotalWeightedVotes.String())
	r.EqualValues(100, cand.SelfStakeBucketIdx)
	r.EqualValues("5000000000000000000", cand.SelfStakingTokens.String())
}

func TestEncodeCandidateToEthErrorOwnerAddress(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833x",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "10000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(cand)
	r.EqualError(err, errInvalidMsg)
}

func TestEncodeCandidateToEthErrorOperatorAddress(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8g",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "10000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(cand)
	r.EqualError(err, errInvalidMsg)
}

func TestEncodeCandidateToEthErrorRewardAddress(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzddd",
		Name:               "hello",
		TotalWeightedVotes: "10000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(cand)
	r.EqualError(err, errInvalidMsg)
}

func TestEncodeCandidateToEthErrorTotalWeightedVotes(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "XXX",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "5000000000000000000",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(cand)
	r.EqualValues("convert big number error", err.Error())
}

func TestEncodeCandidateToEthErrorSelfStakingTokens(t *testing.T) {
	r := require.New(t)

	candidate := &iotextypes.CandidateV2{
		OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
		OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
		RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
		Name:               "hello",
		TotalWeightedVotes: "5000000000000000000",
		SelfStakeBucketIdx: 100,
		SelfStakingTokens:  "XXXX",
	}

	cand, err := encodeCandidateToEth(candidate)

	r.Nil(cand)
	r.EqualValues("convert big number error", err.Error())
}
