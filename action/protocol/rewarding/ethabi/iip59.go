package ethabi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"google.golang.org/protobuf/proto"

	protocolctx "github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
)

const _iip59InterfaceABI = `[
	{"inputs":[{"name":"delegateId","type":"address"}],"name":"pendingVoterReward","outputs":[{"name":"amount","type":"uint256"}],"stateMutability":"view","type":"function"},
	{"inputs":[],"name":"pendingVoterRewardDelegates","outputs":[{"name":"delegateIds","type":"address[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[],"name":"voterRewardDistribution","outputs":[{"name":"targetEra","type":"uint64"},{"name":"completed","type":"bool"},{"name":"completedHeight","type":"uint64"},{"name":"freezeHeight","type":"uint64"},{"name":"startVoter","type":"address"},{"name":"scanPhase","type":"uint32"},{"name":"resumeVoter","type":"bytes"},{"name":"settlementSeed","type":"bytes32"},{"name":"delegateIds","type":"address[]"},{"name":"voterAmounts","type":"uint256[]"},{"name":"distributedAmounts","type":"uint256[]"},{"name":"totalWeights","type":"uint256[]"},{"name":"selfStakeBucketIdxs","type":"uint64[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"delegateId","type":"address"}],"name":"delegateRewardSnapshot","outputs":[{"name":"blockCommissionBasisPoints","type":"uint64"},{"name":"epochCommissionBasisPoints","type":"uint64"},{"name":"commissionConfigured","type":"bool"},{"name":"totalWeight","type":"uint256"},{"name":"freezeHeight","type":"uint64"},{"name":"selfStakeBucketIdx","type":"uint64"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"delegateId","type":"address"}],"name":"delegatePayoutAddress","outputs":[{"name":"payoutAddress","type":"address"},{"name":"onchainRewardEnabled","type":"bool"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"voter","type":"address"}],"name":"voterRewardDestination","outputs":[{"name":"recipient","type":"address"},{"name":"explicitlySet","type":"bool"},{"name":"updatedHeight","type":"uint64"}],"stateMutability":"view","type":"function"}
]`

var (
	_pendingVoterRewardMethod          abi.Method
	_pendingVoterRewardDelegatesMethod abi.Method
	_voterRewardDistributionMethod     abi.Method
	_delegateRewardSnapshotMethod      abi.Method
	_delegatePayoutAddressMethod       abi.Method
	_voterRewardDestinationMethod      abi.Method
)

func init() {
	_pendingVoterRewardMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingVoterReward")
	_pendingVoterRewardDelegatesMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingVoterRewardDelegates")
	_voterRewardDistributionMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardDistribution")
	_delegateRewardSnapshotMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "delegateRewardSnapshot")
	_delegatePayoutAddressMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "delegatePayoutAddress")
	_voterRewardDestinationMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardDestination")
}

type iip59AddressStateContext struct {
	*protocolctx.BaseStateContext
	method abi.Method
}

func newIIP59AddressStateContext(
	data []byte,
	method abi.Method,
	methodName string,
	argumentName string,
) (*iip59AddressStateContext, error) {
	params := make(map[string]interface{})
	if err := method.Inputs.UnpackIntoMap(params, data); err != nil {
		return nil, err
	}
	argument, ok := params[argumentName].(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	ioAddress, err := address.FromBytes(argument.Bytes())
	if err != nil {
		return nil, err
	}
	return &iip59AddressStateContext{
		BaseStateContext: &protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
			MethodName: []byte(methodName),
			Arguments:  [][]byte{[]byte(ioAddress.String())},
		}},
		method: method,
	}, nil
}

func newPendingVoterRewardStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _pendingVoterRewardMethod, "PendingVoterReward", "delegateId")
}

func newDelegateRewardSnapshotStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _delegateRewardSnapshotMethod, "DelegateRewardSnapshot", "delegateId")
}

func newDelegatePayoutAddressStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _delegatePayoutAddressMethod, "DelegatePayoutAddress", "delegateId")
}

func newVoterRewardDestinationStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _voterRewardDestinationMethod, "VoterRewardDestination", "voter")
}

func (r *iip59AddressStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var (
		data []byte
		err  error
	)
	switch r.method.Name {
	case _pendingVoterRewardMethod.Name:
		amount, ok := new(big.Int).SetString(string(resp.Data), 10)
		if !ok {
			return "", errConvertBigNumber
		}
		data, err = r.method.Outputs.Pack(amount)
	case _delegateRewardSnapshotMethod.Name:
		snapshot := &stakingpb.CandidateRewardSnapshot{}
		if err = proto.Unmarshal(resp.Data, snapshot); err == nil {
			data, err = r.method.Outputs.Pack(
				snapshot.GetBlockCommissionBasisPoints(), snapshot.GetEpochCommissionBasisPoints(),
				snapshot.GetCommissionConfigured(),
				new(big.Int).SetBytes(snapshot.GetTotalWeight()), snapshot.GetFreezeHeight(),
				snapshot.GetSelfStakeBucketIdx(),
			)
		}
	case _delegatePayoutAddressMethod.Name:
		state := &rewardingpb.DelegatePayoutAddress{}
		if err = proto.Unmarshal(resp.Data, state); err == nil {
			data, err = r.method.Outputs.Pack(
				common.BytesToAddress(state.GetAddress()), state.GetOnchainRewardEnabled(),
			)
		}
	case _voterRewardDestinationMethod.Name:
		state := &rewardingpb.VoterRewardDestination{}
		if err = proto.Unmarshal(resp.Data, state); err == nil {
			data, err = r.method.Outputs.Pack(
				common.BytesToAddress(state.GetRecipient()), state.GetExplicitlySet(), state.GetUpdatedHeight(),
			)
		}
	default:
		return "", errInvalidCallSig
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

type PendingVoterRewardDelegatesStateContext struct {
	*protocolctx.BaseStateContext
}

func newPendingVoterRewardDelegatesStateContext() (*PendingVoterRewardDelegatesStateContext, error) {
	return &PendingVoterRewardDelegatesStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("PendingVoterRewardDelegates"),
	}}}, nil
}

func (r *PendingVoterRewardDelegatesStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	delegates := &rewardingpb.PendingVoterRewardDelegates{}
	if err := proto.Unmarshal(resp.Data, delegates); err != nil {
		return "", err
	}
	ids := make([]common.Address, len(delegates.GetDelegateIdentifiers()))
	for i, id := range delegates.GetDelegateIdentifiers() {
		ids[i] = common.BytesToAddress(id)
	}
	data, err := _pendingVoterRewardDelegatesMethod.Outputs.Pack(ids)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

type VoterRewardDistributionStateContext struct {
	*protocolctx.BaseStateContext
}

func newVoterRewardDistributionStateContext() (*VoterRewardDistributionStateContext, error) {
	return &VoterRewardDistributionStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("VoterRewardDistribution"),
	}}}, nil
}

func (r *VoterRewardDistributionStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	cursor := &rewardingpb.VoterRewardDistributionState{}
	if err := proto.Unmarshal(resp.Data, cursor); err != nil {
		return "", err
	}
	delegates := cursor.GetDelegateAllocations()
	ids := make([]common.Address, len(delegates))
	voterAmounts := make([]*big.Int, len(delegates))
	distributedAmounts := make([]*big.Int, len(delegates))
	totalWeights := make([]*big.Int, len(delegates))
	selfStakeBucketIdxs := make([]uint64, len(delegates))
	for i, delegate := range delegates {
		ids[i] = common.BytesToAddress(delegate.GetCandidateIdentifier())
		voterAmounts[i] = new(big.Int).SetBytes(delegate.GetVoterAmountFrozen())
		distributedAmounts[i] = new(big.Int).SetBytes(delegate.GetVoterAmountDistributed())
		totalWeights[i] = new(big.Int).SetBytes(delegate.GetTotalWeight())
		selfStakeBucketIdxs[i] = delegate.GetSelfStakeBucketIdx()
	}
	data, err := _voterRewardDistributionMethod.Outputs.Pack(
		cursor.GetTargetEra(), cursor.GetCompleted(), cursor.GetCompletedHeight(), cursor.GetFreezeHeight(),
		common.BytesToAddress(cursor.GetStartVoter()), cursor.GetScanPhase(), cursor.GetResumeVoter(),
		common.BytesToHash(cursor.GetSettlementSeed()),
		ids, voterAmounts, distributedAmounts, totalWeights, selfStakeBucketIdxs,
	)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
