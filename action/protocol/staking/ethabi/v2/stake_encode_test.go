package v2

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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

func TestCompositeBucketsEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("40f086d6", hex.EncodeToString(_compositeBucketsMethod.ID))

	input, _ := hex.DecodeString("40f086d600000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000005")
	ctx, err := newCompositeBucketsStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, compositeVoteBucketList())})
	r.NoError(err)
	r.Equal(_compositeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	bad := compositeVoteBucketList()
	bad.Buckets[0].StakedAmount = "xx"
	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, bad)})
	r.ErrorIs(err, stakingComm.ErrConvertBigNumber)

	_, err = newCompositeBucketsStateContext([]byte{0x01})
	r.Error(err)
}

func TestCompositeBucketsByCandidateEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("33df73c7", hex.EncodeToString(_compositeBucketsByCandidateMethod.ID))

	input, _ := hex.DecodeString("33df73c7000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000568656c6c6f000000000000000000000000000000000000000000000000000000")
	ctx, err := newCompositeBucketsByCandidateStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, compositeVoteBucketList())})
	r.NoError(err)
	r.Equal(_compositeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	_, err = newCompositeBucketsByCandidateStateContext([]byte{0xde, 0xad})
	r.Error(err)
}

func TestCompositeBucketsByIndexesEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("347cdbd5", hex.EncodeToString(_compositeBucketsByIndexesMethod.ID))

	input, _ := hex.DecodeString("347cdbd50000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	ctx, err := newCompositeBucketsByIndexesStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, compositeVoteBucketList())})
	r.NoError(err)
	r.Equal(_compositeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	_, err = newCompositeBucketsByIndexesStateContext([]byte{0x01})
	r.Error(err)
}

func TestCompositeBucketsByVoterEncodeToEth(t *testing.T) {
	r := require.New(t)

	r.Equal("80965570", hex.EncodeToString(_compositeBucketsByVoterMethod.ID))

	input, _ := hex.DecodeString("80965570000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	ctx, err := newCompositeBucketsByVoterStateContext(input[4:])
	r.NoError(err)

	out, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: mustMarshal(t, compositeVoteBucketList())})
	r.NoError(err)
	r.Equal(_compositeVoteBucketListEth, out)

	_, err = ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte{0xff}})
	r.Error(err)

	_, err = newCompositeBucketsByVoterStateContext([]byte{0x01, 0x02})
	r.Error(err)
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

	input, _ := hex.DecodeString("017619d40000000000000000000000000000000000000000000000000000000000000064")
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
