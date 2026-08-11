package ethabi

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestIIP59AddressMethodDispatch(t *testing.T) {
	r := require.New(t)
	candidate := common.BytesToAddress(identityset.Address(7).Bytes())
	methods := []struct {
		methodName string
		method     *abi.Method
		stateName  string
	}{
		{"pendingBlockRewardPool", &_pendingBlockRewardPoolMethod, "PendingBlockRewardPool"},
		{"voterRewardDelegateSnapshot", &_voterRewardDelegateSnapshotMethod, "VoterRewardSnapshot"},
		{"voterRewardAddress", &_voterRewardAddressMethod, "VoterRewardAddress"},
		{"voterRewardDestination", &_voterRewardDestinationMethod, "VoterRewardDestination"},
	}
	for _, test := range methods {
		calldata, err := test.method.Inputs.Pack(candidate)
		r.NoError(err)
		ctx, err := BuildReadStateRequest(append(test.method.ID, calldata...))
		r.NoError(err, test.methodName)
		r.Equal(test.stateName, string(ctx.Parameters().MethodName))
		r.Equal(identityset.Address(7).String(), string(ctx.Parameters().Arguments[0]))
	}
}

func TestIIP59VoterRewardDestinationEncoding(t *testing.T) {
	r := require.New(t)
	voter := common.BytesToAddress(identityset.Address(7).Bytes())
	ctx, err := newVoterRewardDestinationStateContext(
		mustPackInput(t, _voterRewardDestinationMethod, voter),
	)
	r.NoError(err)
	recipient := common.BytesToAddress(identityset.Address(8).Bytes())
	data, err := proto.Marshal(&rewardingpb.VoterRewardDestination{
		Recipient: recipient.Bytes(), ExplicitlySet: true, UpdatedHeight: 12345,
	})
	r.NoError(err)
	encoded, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err := _voterRewardDestinationMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(recipient, values[0])
	r.Equal(true, values[1])
	r.Equal(uint64(12345), values[2])
}

func TestIIP59VoterRewardStatusDispatchAndEncoding(t *testing.T) {
	r := require.New(t)
	voter := common.BytesToAddress(identityset.Address(8).Bytes())
	calldata, err := _voterRewardStatusMethod.Inputs.Pack(voter)
	r.NoError(err)
	ctx, err := BuildReadStateRequest(append(_voterRewardStatusMethod.ID, calldata...))
	r.NoError(err)
	r.Equal("VoterRewardStatus", string(ctx.Parameters().MethodName))
	r.Len(ctx.Parameters().Arguments, 1)
	r.Equal(identityset.Address(8).String(), string(ctx.Parameters().Arguments[0]))

	data, err := proto.Marshal(&rewardingpb.VoterRewardStatus{
		TargetEra:           24,
		EraStartEpoch:       1,
		EraEndEpoch:         24,
		SettlementCompleted: true,
		CompletedHeight:     8640,
		Status:              rewardingpb.VoterRewardStatus_WAITING,
		RewardAmount:        big.NewInt(123456789).Bytes(),
	})
	r.NoError(err)
	encoded, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err := _voterRewardStatusMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(24), values[0])
	r.Equal(uint64(1), values[1])
	r.Equal(uint64(24), values[2])
	r.Equal(true, values[3])
	r.Equal(uint64(8640), values[4])
	r.Equal(uint8(rewardingpb.VoterRewardStatus_WAITING), values[5])
	r.Zero(values[6].(*big.Int).Cmp(big.NewInt(123456789)))
}

func TestIIP59RewardAddressEncoding(t *testing.T) {
	r := require.New(t)
	ctx, err := newVoterRewardAddressStateContext(
		mustPackInput(t, _voterRewardAddressMethod, common.BytesToAddress(identityset.Address(1).Bytes())),
	)
	r.NoError(err)
	configured := common.BytesToAddress(identityset.Address(2).Bytes())
	data, err := proto.Marshal(&rewardingpb.VoterRewardAddress{
		Address: configured.Bytes(), ExplicitlySet: true,
	})
	r.NoError(err)
	encoded, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err := _voterRewardAddressMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(configured, values[0])
	r.Equal(true, values[1])
}

func mustPackInput(t *testing.T, method abi.Method, args ...interface{}) []byte {
	t.Helper()
	data, err := method.Inputs.Pack(args...)
	require.NoError(t, err)
	return data
}

func mustDecodeHex(t *testing.T, data string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(data)
	require.NoError(t, err)
	return decoded
}

func TestIIP59PendingPoolAndIndexEncoding(t *testing.T) {
	r := require.New(t)
	poolCtx, err := newPendingBlockRewardPoolStateContext(mustPackInput(t, _pendingBlockRewardPoolMethod, common.Address{}))
	r.NoError(err)
	encoded, err := poolCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte("12345")})
	r.NoError(err)
	decoded := mustDecodeHex(t, encoded)
	values, err := _pendingBlockRewardPoolMethod.Outputs.Unpack(decoded)
	r.NoError(err)
	r.Zero(values[0].(*big.Int).Cmp(big.NewInt(12345)))

	ids := [][]byte{identityset.Address(1).Bytes(), identityset.Address(2).Bytes()}
	data, err := proto.Marshal(&rewardingpb.PendingBlockRewardPoolIndex{CandidateIdentifiers: ids})
	r.NoError(err)
	indexCtx, err := newPendingBlockRewardPoolIndexStateContext()
	r.NoError(err)
	encoded, err = indexCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err = _pendingBlockRewardPoolIndexMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal([]common.Address{
		common.BytesToAddress(ids[0]), common.BytesToAddress(ids[1]),
	}, values[0].([]common.Address))
}

func TestIIP59CursorAndSnapshotEncoding(t *testing.T) {
	r := require.New(t)
	cursorData, err := proto.Marshal(&rewardingpb.EpochDrainCursor{
		TargetEra:       4,
		Completed:       true,
		CompletedHeight: 99,
		FreezeHeight:    8_800,
		StartVoter:      identityset.Address(5).Bytes(),
		ScanPhase:       2,
		ResumeVoter:     identityset.Address(6).Bytes(),
		SettlementSeed:  common.HexToHash("0x9876").Bytes(),
		Delegates: []*rewardingpb.EpochDrainDelegateWork{{
			CandidateIdentifier:    identityset.Address(3).Bytes(),
			VoterAmountFrozen:      big.NewInt(1000).Bytes(),
			VoterAmountDistributed: big.NewInt(400).Bytes(),
			TotalWeight:            big.NewInt(300).Bytes(),
			SelfStakeBucketIdx:     42,
		}},
	})
	r.NoError(err)
	cursorCtx, err := newEraDrainCursorStateContext()
	r.NoError(err)
	encoded, err := cursorCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: cursorData})
	r.NoError(err)
	values, err := _eraDrainCursorMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(4), values[0])
	r.Equal(true, values[1])
	r.Equal(uint64(99), values[2])
	r.Equal(uint64(8_800), values[3])
	r.Equal(common.BytesToAddress(identityset.Address(5).Bytes()), values[4])
	r.Equal(uint32(2), values[5])
	r.Equal(identityset.Address(6).Bytes(), values[6])
	r.Equal([32]byte(common.HexToHash("0x9876")), values[7])
	r.Equal([]common.Address{common.BytesToAddress(identityset.Address(3).Bytes())}, values[8])
	r.Zero(values[9].([]*big.Int)[0].Cmp(big.NewInt(1000)))
	r.Zero(values[10].([]*big.Int)[0].Cmp(big.NewInt(400)))
	r.Zero(values[11].([]*big.Int)[0].Cmp(big.NewInt(300)))
	r.Equal([]uint64{42}, values[12].([]uint64))

	snapshotData, err := proto.Marshal(&stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		Registered:                 true,
		OnchainRewardEnabled:       true,
		TotalWeight:                big.NewInt(300).Bytes(),
		FreezeHeight:               8_800,
		SelfStakeBucketIdx:         42,
	})
	r.NoError(err)
	snapshotCtx, err := newVoterRewardDelegateSnapshotStateContext(
		mustPackInput(t, _voterRewardDelegateSnapshotMethod, common.BytesToAddress(identityset.Address(3).Bytes())),
	)
	r.NoError(err)
	encoded, err = snapshotCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: snapshotData})
	r.NoError(err)
	values, err = _voterRewardDelegateSnapshotMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(1000), values[0])
	r.Equal(uint64(2000), values[1])
	r.Equal(true, values[2])
	r.Equal(true, values[3])
	r.Zero(values[4].(*big.Int).Cmp(big.NewInt(300)))
	// The snapshot now exposes the frozen bucket height and self-stake bucket
	// directly; voter weights are recomputed from bucket state on demand.
	r.Equal(uint64(8_800), values[5])
	r.Equal(uint64(42), values[6])
}

// TestIIP59RetiredVoterRewardSnapshotSelector pins the removal of the original
// voterRewardSnapshot(address) selector. Its return tuple ended in
// (address[] voters, uint256[] weights); the replacement ends in
// (uint64 freezeHeight, uint64 selfStakeBucketIdx). Re-registering the old
// 4-byte id — under any name — would let a caller built against the old ABI
// decode the new tuple as the old one and read a freeze height as an array
// offset. IIP-59 is unactivated, so nothing is owed compatibility here.
func TestIIP59RetiredVoterRewardSnapshotSelector(t *testing.T) {
	r := require.New(t)
	retired := abiutil.MustLoadMethod(`[
		{"inputs":[{"name":"candidateId","type":"address"}],"name":"voterRewardSnapshot","outputs":[{"name":"blockCommissionBasisPoints","type":"uint64"},{"name":"epochCommissionBasisPoints","type":"uint64"},{"name":"registered","type":"bool"},{"name":"onchainRewardEnabled","type":"bool"},{"name":"totalWeight","type":"uint256"},{"name":"snapshotHash","type":"bytes32"},{"name":"voters","type":"address[]"},{"name":"weights","type":"uint256[]"}],"stateMutability":"view","type":"function"}
	]`, "voterRewardSnapshot")

	r.NotEqual(retired.ID, _voterRewardDelegateSnapshotMethod.ID,
		"the replacement must not reuse the retired selector")

	calldata, err := retired.Inputs.Pack(common.BytesToAddress(identityset.Address(7).Bytes()))
	r.NoError(err)
	_, err = BuildReadStateRequest(append(retired.ID, calldata...))
	r.ErrorIs(err, errInvalidCallSig)
}

// TestIIP59RetiredEpochDrainCursorSelector pins the removal of the original
// epochDrainCursor() selector, for the same reason as the snapshot view above
// and with a sharper failure mode: the method takes no arguments, so the old
// selector is reachable from calldata that is nothing but the 4-byte id. Its
// tuple carried the candidate-major quartet (delegateIndex, voterIndex,
// delegateStartIndex, voterStartIndices); the drain is voter-major now and the
// replacement carries (startVoter, scanPhase, resumeVoter) instead. Decoded
// against the old ABI, bytes from the circular-address cursor would be read as
// candidate-major positions, producing plausible but wrong progress.
func TestIIP59RetiredEpochDrainCursorSelector(t *testing.T) {
	r := require.New(t)
	retired := abiutil.MustLoadMethod(`[
		{"inputs":[],"name":"epochDrainCursor","outputs":[{"name":"targetEra","type":"uint64"},{"name":"startEpoch","type":"uint64"},{"name":"endEpoch","type":"uint64"},{"name":"completed","type":"bool"},{"name":"completedHeight","type":"uint64"},{"name":"delegateIndex","type":"uint32"},{"name":"voterIndex","type":"uint32"},{"name":"settlementSeed","type":"bytes32"},{"name":"delegateStartIndex","type":"uint32"},{"name":"candidateIds","type":"address[]"},{"name":"voterStartIndices","type":"uint32[]"},{"name":"voterAmounts","type":"uint256[]"},{"name":"distributedAmounts","type":"uint256[]"},{"name":"rewardAddresses","type":"address[]"},{"name":"epochCommissions","type":"uint256[]"},{"name":"totalWeights","type":"uint256[]"}],"stateMutability":"view","type":"function"}
	]`, "epochDrainCursor")

	r.NotEqual(retired.ID, _eraDrainCursorMethod.ID,
		"the replacement must not reuse the retired selector")

	_, err := BuildReadStateRequest(retired.ID)
	r.ErrorIs(err, errInvalidCallSig)
}

func TestIIP59RetiredShardEraDrainCursorSelector(t *testing.T) {
	r := require.New(t)
	retired := abiutil.MustLoadMethod(`[
		{"inputs":[],"name":"eraDrainCursor","outputs":[{"name":"targetEra","type":"uint64"},{"name":"completed","type":"bool"},{"name":"completedHeight","type":"uint64"},{"name":"freezeHeight","type":"uint64"},{"name":"startShard","type":"uint32"},{"name":"shardsDone","type":"uint32"},{"name":"resumeVoter","type":"bytes"},{"name":"settlementSeed","type":"bytes32"},{"name":"candidateIds","type":"address[]"},{"name":"voterAmounts","type":"uint256[]"},{"name":"distributedAmounts","type":"uint256[]"},{"name":"totalWeights","type":"uint256[]"},{"name":"selfStakeBucketIdxs","type":"uint64[]"}],"stateMutability":"view","type":"function"}
	]`, "eraDrainCursor")

	r.NotEqual(retired.ID, _eraDrainCursorMethod.ID,
		"the global voter scan must not reuse the retired shard cursor selector")
	_, err := BuildReadStateRequest(retired.ID)
	r.ErrorIs(err, errInvalidCallSig)
}
