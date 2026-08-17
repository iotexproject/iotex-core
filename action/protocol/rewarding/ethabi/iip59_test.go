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
		{"pendingVoterReward", &_pendingVoterRewardMethod, "PendingVoterReward"},
		{"delegateRewardSnapshot", &_delegateRewardSnapshotMethod, "DelegateRewardSnapshot"},
		{"delegatePayoutAddress", &_delegatePayoutAddressMethod, "DelegatePayoutAddress"},
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

func TestIIP59DelegatePayoutAddressEncoding(t *testing.T) {
	r := require.New(t)
	ctx, err := newDelegatePayoutAddressStateContext(
		mustPackInput(t, _delegatePayoutAddressMethod, common.BytesToAddress(identityset.Address(1).Bytes())),
	)
	r.NoError(err)
	configured := common.BytesToAddress(identityset.Address(2).Bytes())
	data, err := proto.Marshal(&rewardingpb.DelegatePayoutAddress{
		Address: configured.Bytes(), OnchainRewardEnabled: true,
	})
	r.NoError(err)
	encoded, err := ctx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err := _delegatePayoutAddressMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
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

func TestIIP59PendingRewardAndDelegatesEncoding(t *testing.T) {
	r := require.New(t)
	poolCtx, err := newPendingVoterRewardStateContext(mustPackInput(t, _pendingVoterRewardMethod, common.Address{}))
	r.NoError(err)
	encoded, err := poolCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: []byte("12345")})
	r.NoError(err)
	decoded := mustDecodeHex(t, encoded)
	values, err := _pendingVoterRewardMethod.Outputs.Unpack(decoded)
	r.NoError(err)
	r.Zero(values[0].(*big.Int).Cmp(big.NewInt(12345)))

	ids := [][]byte{identityset.Address(1).Bytes(), identityset.Address(2).Bytes()}
	data, err := proto.Marshal(&rewardingpb.PendingVoterRewardDelegates{DelegateIdentifiers: ids})
	r.NoError(err)
	delegatesCtx, err := newPendingVoterRewardDelegatesStateContext()
	r.NoError(err)
	encoded, err = delegatesCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: data})
	r.NoError(err)
	values, err = _pendingVoterRewardDelegatesMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal([]common.Address{
		common.BytesToAddress(ids[0]), common.BytesToAddress(ids[1]),
	}, values[0].([]common.Address))
}

func TestIIP59DistributionAndSnapshotEncoding(t *testing.T) {
	r := require.New(t)
	cursorData, err := proto.Marshal(&rewardingpb.VoterRewardDistributionState{
		TargetEra:       4,
		Completed:       true,
		CompletedHeight: 99,
		FreezeHeight:    8_800,
		StartVoter:      identityset.Address(5).Bytes(),
		ScanPhase:       2,
		ResumeVoter:     identityset.Address(6).Bytes(),
		SettlementSeed:  common.HexToHash("0x9876").Bytes(),
		DelegateAllocations: []*rewardingpb.VoterRewardDelegateAllocation{{
			CandidateIdentifier:    identityset.Address(3).Bytes(),
			VoterAmountFrozen:      big.NewInt(1000).Bytes(),
			VoterAmountDistributed: big.NewInt(400).Bytes(),
			TotalWeight:            big.NewInt(300).Bytes(),
			SelfStakeBucketIdx:     42,
		}},
	})
	r.NoError(err)
	cursorCtx, err := newVoterRewardDistributionStateContext()
	r.NoError(err)
	encoded, err := cursorCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: cursorData})
	r.NoError(err)
	values, err := _voterRewardDistributionMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
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

	snapshotData, err := proto.Marshal(&stakingpb.CandidateRewardSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		CommissionConfigured:       true,
		TotalWeight:                big.NewInt(300).Bytes(),
		FreezeHeight:               8_800,
		SelfStakeBucketIdx:         42,
	})
	r.NoError(err)
	snapshotCtx, err := newDelegateRewardSnapshotStateContext(
		mustPackInput(t, _delegateRewardSnapshotMethod, common.BytesToAddress(identityset.Address(3).Bytes())),
	)
	r.NoError(err)
	encoded, err = snapshotCtx.EncodeToEth(&iotexapi.ReadStateResponse{Data: snapshotData})
	r.NoError(err)
	values, err = _delegateRewardSnapshotMethod.Outputs.Unpack(mustDecodeHex(t, encoded))
	r.NoError(err)
	r.Equal(uint64(1000), values[0])
	r.Equal(uint64(2000), values[1])
	r.Equal(true, values[2])
	r.Zero(values[3].(*big.Int).Cmp(big.NewInt(300)))
	// The snapshot now exposes the frozen bucket height and self-stake bucket
	// directly; voter weights are recomputed from bucket state on demand.
	r.Equal(uint64(8_800), values[4])
	r.Equal(uint64(42), values[5])
}
