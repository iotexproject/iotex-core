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
	data, err := proto.Marshal(&rewardingpb.Exempt{Addrs: ids})
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
		TargetEra:     4,
		DelegateIndex: 1,
		VoterIndex:    25,
		Delegates: []*rewardingpb.EpochDrainDelegateWork{{
			CandidateIdentifier:    identityset.Address(3).Bytes(),
			VoterAmountFrozen:      big.NewInt(1000).Bytes(),
			VoterAmountDistributed: big.NewInt(400).Bytes(),
			RewardAddress:          identityset.Address(4).Bytes(),
			EpochCommission:        big.NewInt(200).Bytes(),
			TotalWeight:            big.NewInt(300).Bytes(),
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
	r.Equal(uint32(1), values[1])
	r.Equal(uint32(25), values[2])
	r.Zero(values[4].([]*big.Int)[0].Cmp(big.NewInt(1000)))
	r.Zero(values[5].([]*big.Int)[0].Cmp(big.NewInt(400)))
	r.Zero(values[8].([]*big.Int)[0].Cmp(big.NewInt(300)))

	snapshotHash := common.HexToHash("0x1234")
	snapshotData, err := proto.Marshal(&stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		Registered:                 true,
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
	r.Zero(values[3].(*big.Int).Cmp(big.NewInt(300)))
	r.Equal([32]byte(snapshotHash), values[4])
	r.Len(values[5].([]common.Address), 2)
	r.Len(values[6].([]*big.Int), 2)
}
