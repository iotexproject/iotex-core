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
		{"voterRewardSnapshot", &_voterRewardSnapshotMethod, "VoterRewardSnapshot"},
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
	candidate := common.BytesToAddress(identityset.Address(7).Bytes())
	voter := common.BytesToAddress(identityset.Address(8).Bytes())
	calldata, err := _voterRewardStatusMethod.Inputs.Pack(candidate, voter)
	r.NoError(err)
	ctx, err := BuildReadStateRequest(append(_voterRewardStatusMethod.ID, calldata...))
	r.NoError(err)
	r.Equal("VoterRewardStatus", string(ctx.Parameters().MethodName))
	r.Equal(identityset.Address(7).String(), string(ctx.Parameters().Arguments[0]))
	r.Equal(identityset.Address(8).String(), string(ctx.Parameters().Arguments[1]))

	data, err := proto.Marshal(&rewardingpb.VoterRewardStatus{
		TargetEra:           24,
		EraStartEpoch:       1,
		EraEndEpoch:         24,
		SettlementCompleted: true,
		CompletedHeight:     8640,
		Status:              rewardingpb.VoterRewardStatus_WAITING,
		LogicalVoterIndex:   23,
		VoterStartIndex:     11,
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
	r.Equal(uint32(23), values[6])
	r.Equal(uint32(11), values[7])
	r.Zero(values[8].(*big.Int).Cmp(big.NewInt(123456789)))
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
		TargetEra:          4,
		StartEpoch:         1,
		EndEpoch:           4,
		Completed:          true,
		CompletedHeight:    99,
		DelegateIndex:      1,
		VoterIndex:         25,
		SettlementSeed:     common.HexToHash("0x9876").Bytes(),
		DelegateStartIndex: 2,
		Delegates: []*rewardingpb.EpochDrainDelegateWork{{
			CandidateIdentifier:    identityset.Address(3).Bytes(),
			VoterAmountFrozen:      big.NewInt(1000).Bytes(),
			VoterAmountDistributed: big.NewInt(400).Bytes(),
			RewardAddress:          identityset.Address(4).Bytes(),
			EpochCommission:        big.NewInt(200).Bytes(),
			TotalWeight:            big.NewInt(300).Bytes(),
			VoterStartIndex:        17,
		}},
	})
	r.NoError(err)
	cursorCtx, err := newEpochDrainCursorStateContext()
	r.NoError(err)
	encoded, err := cursorCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: cursorData})
	r.NoError(err)
	values, err := _epochDrainCursorMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(4), values[0])
	r.Equal(uint64(1), values[1])
	r.Equal(uint64(4), values[2])
	r.Equal(true, values[3])
	r.Equal(uint64(99), values[4])
	r.Equal(uint32(1), values[5])
	r.Equal(uint32(25), values[6])
	r.Equal([32]byte(common.HexToHash("0x9876")), values[7])
	r.Equal(uint32(2), values[8])
	r.Equal([]common.Address{common.BytesToAddress(identityset.Address(3).Bytes())}, values[9])
	r.Equal([]uint32{17}, values[10])
	r.Zero(values[11].([]*big.Int)[0].Cmp(big.NewInt(1000)))
	r.Zero(values[12].([]*big.Int)[0].Cmp(big.NewInt(400)))
	r.Equal([]common.Address{common.BytesToAddress(identityset.Address(4).Bytes())}, values[13])
	r.Zero(values[14].([]*big.Int)[0].Cmp(big.NewInt(200)))
	r.Zero(values[15].([]*big.Int)[0].Cmp(big.NewInt(300)))

	snapshotHash := common.HexToHash("0x1234")
	snapshotData, err := proto.Marshal(&stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		Registered:                 true,
		OnchainRewardEnabled:       true,
		TotalWeight:                big.NewInt(300).Bytes(),
		SnapshotHash:               snapshotHash.Bytes(),
		Entries: []*stakingpb.VoterWeightEntry{
			{Voter: identityset.Address(5).Bytes(), Weight: big.NewInt(100).Bytes()},
			{Voter: identityset.Address(6).Bytes(), Weight: big.NewInt(200).Bytes()},
		},
	})
	r.NoError(err)
	snapshotCtx, err := newVoterRewardSnapshotStateContext(
		mustPackInput(t, _voterRewardSnapshotMethod, common.BytesToAddress(identityset.Address(3).Bytes())),
	)
	r.NoError(err)
	encoded, err = snapshotCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: snapshotData})
	r.NoError(err)
	values, err = _voterRewardSnapshotMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(1000), values[0])
	r.Equal(uint64(2000), values[1])
	r.Equal(true, values[2])
	r.Equal(true, values[3])
	r.Zero(values[4].(*big.Int).Cmp(big.NewInt(300)))
	r.Equal([32]byte(snapshotHash), values[5])
	r.Len(values[6].([]common.Address), 2)
	r.Len(values[7].([]*big.Int), 2)
}
