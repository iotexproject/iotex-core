package v2

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

// _compositeVoteBucketListEth is the expected eth-abi encoding of compositeVoteBucketList()
// through the composite (14-field) VoteBucket tuple[] output layout shared by all v2
// composite bucket views.
const _compositeVoteBucketListEth = "00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000c800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000650000000000000000000000000000000000000000000000001bc16d674ec800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c900000000000000000000000000000000000000000000000000000000000000c900000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000003b9aca01000000000000000000000000000000000000000000000000000000003b9aca02"

func compositeVoteBucketList() *iotextypes.VoteBucketList {
	return &iotextypes.VoteBucketList{Buckets: []*iotextypes.VoteBucket{
		{
			Index:            1,
			CandidateAddress: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqryn4k9fw",
			StakedAmount:     "1000000000000000000",
			StakedDuration:   1_000_000,
			CreateTime:       &timestamppb.Timestamp{Seconds: 1_000_000_000},
			StakeStartTime:   &timestamppb.Timestamp{Seconds: 1_000_000_001},
			UnstakeStartTime: &timestamppb.Timestamp{Seconds: 1_000_000_002},
			AutoStake:        true,
			Owner:            "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxgce2xkh",
			ContractAddress:  "",
		},
		{
			Index:                     2,
			CandidateAddress:          "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqr9wrzs5u",
			StakedAmount:              "2000000000000000000",
			AutoStake:                 false,
			Owner:                     "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
			ContractAddress:           "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxf907nt9",
			StakedDurationBlockNumber: 1_000_000,
			CreateBlockHeight:         1_000_000_000,
			StakeStartBlockHeight:     1_000_000_001,
			UnstakeStartBlockHeight:   1_000_000_002,
		},
	}}
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}

// TestCompositeBucketViewsEncodeToEth exercises the EncodeToEth path and decode
// error paths for every v2 composite vote-bucket view. All four views share the
// composite VoteBucket output layout, so they must encode the same list to the
// same bytes.
func TestCompositeBucketViewsEncodeToEth(t *testing.T) {
	cases := []struct {
		name   string
		method *abi.Method
		// input is the full eth_call data (selector + args) taken from the
		// corresponding TestBuildReadStateRequest* fixture.
		input  string
		newCtx func([]byte) (protocol.StateContext, error)
	}{
		{
			name:   "compositeBuckets",
			method: &_compositeBucketsMethod,
			input:  "40f086d600000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000005",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newCompositeBucketsStateContext(d) },
		},
		{
			name:   "compositeBucketsByCandidate",
			method: &_compositeBucketsByCandidateMethod,
			input:  "33df73c7000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newCompositeBucketsByCandidateStateContext(d) },
		},
		{
			name:   "compositeBucketsByIndexes",
			method: &_compositeBucketsByIndexesMethod,
			input:  "347cdbd50000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newCompositeBucketsByIndexesStateContext(d) },
		},
		{
			name:   "compositeBucketsByVoter",
			method: &_compositeBucketsByVoterMethod,
			input:  "80965570000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002",
			newCtx: func(d []byte) (protocol.StateContext, error) { return newCompositeBucketsByVoterStateContext(d) },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)

			// method signature guard: the selector must match the call data
			r.Equal(hex.EncodeToString(c.method.ID), c.input[:8])

			data, err := hex.DecodeString(c.input)
			r.NoError(err)
			ctx, err := c.newCtx(data[4:])
			r.NoError(err)

			// well-formed payload round-trips to the expected eth encoding
			out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, compositeVoteBucketList())})
			r.NoError(err)
			r.Equal(_compositeVoteBucketListEth, out)

			// malformed proto payload -> unmarshal error
			_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
			r.Error(err)

			// non-numeric staked amount -> convert big number error
			bad := compositeVoteBucketList()
			bad.Buckets[0].StakedAmount = "not-a-number"
			_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, bad)})
			r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

			// malformed call data -> decode error
			_, err = c.newCtx([]byte{0xde, 0xad})
			r.Error(err)
		})
	}
}

func TestCompositeTotalStakingAmountEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("3aad591c", hex.EncodeToString(_compositeTotalStakingAmountMethod.ID))

	ctx, err := newCompositeTotalStakingAmountContext()
	r.NoError(err)

	meta := &iotextypes.AccountMeta{Balance: "123456789012345678901234567890"}
	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, meta)})
	r.NoError(err)
	// 123456789012345678901234567890 packed as a uint256
	r.Equal("00000000000000000000000000000000000000018ee90ff6c373e0ee4e3f0ad2", out)

	// malformed proto payload -> unmarshal error
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	// non-numeric balance -> convert big number error
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, &iotextypes.AccountMeta{Balance: "xxx"})})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)
}

func TestContractBucketTypesEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("017619d4", hex.EncodeToString(_contractBucketTypesMethod.ID))

	input, err := hex.DecodeString("017619d40000000000000000000000000000000000000000000000000000000000000064")
	r.NoError(err)
	ctx, err := newContractBucketTypesStateContext(input[4:])
	r.NoError(err)

	btList := &iotextypes.ContractStakingBucketTypeList{BucketTypes: []*iotextypes.ContractStakingBucketType{
		{StakedAmount: "1000000000000000000", StakedDuration: 1_000_000},
		{StakedAmount: "2000000000000000000", StakedDuration: 2_000_000},
	}}
	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, btList)})
	r.NoError(err)
	r.Equal("000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000f42400000000000000000000000000000000000000000000000001bc16d674ec8000000000000000000000000000000000000000000000000000000000000001e8480", out)

	// malformed proto payload -> unmarshal error
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	// non-numeric staked amount -> convert big number error
	badBt := &iotextypes.ContractStakingBucketTypeList{BucketTypes: []*iotextypes.ContractStakingBucketType{{StakedAmount: "xx", StakedDuration: 1}}}
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, badBt)})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

	// malformed contract address argument -> decode error
	_, err = newContractBucketTypesStateContext([]byte{0x01, 0x02})
	r.Error(err)
}
