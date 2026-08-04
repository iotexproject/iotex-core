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
	{"inputs":[{"name":"candidateId","type":"address"}],"name":"pendingBlockRewardPool","outputs":[{"name":"amount","type":"uint256"}],"stateMutability":"view","type":"function"},
	{"inputs":[],"name":"pendingBlockRewardPoolIndex","outputs":[{"name":"candidateIds","type":"address[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[],"name":"eraDrainCursor","outputs":[{"name":"targetEra","type":"uint64"},{"name":"startEpoch","type":"uint64"},{"name":"endEpoch","type":"uint64"},{"name":"completed","type":"bool"},{"name":"completedHeight","type":"uint64"},{"name":"startShard","type":"uint32"},{"name":"shardsDone","type":"uint32"},{"name":"resumeVoter","type":"bytes"},{"name":"settlementSeed","type":"bytes32"},{"name":"candidateIds","type":"address[]"},{"name":"voterAmounts","type":"uint256[]"},{"name":"distributedAmounts","type":"uint256[]"},{"name":"rewardAddresses","type":"address[]"},{"name":"epochCommissions","type":"uint256[]"},{"name":"totalWeights","type":"uint256[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"candidateId","type":"address"}],"name":"voterRewardDelegateSnapshot","outputs":[{"name":"blockCommissionBasisPoints","type":"uint64"},{"name":"epochCommissionBasisPoints","type":"uint64"},{"name":"registered","type":"bool"},{"name":"onchainRewardEnabled","type":"bool"},{"name":"totalWeight","type":"uint256"},{"name":"snapshotHash","type":"bytes32"},{"name":"freezeHeight","type":"uint64"},{"name":"selfStakeBucketIdx","type":"uint64"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"candidateId","type":"address"}],"name":"voterRewardAddress","outputs":[{"name":"rewardAddress","type":"address"},{"name":"explicitlySet","type":"bool"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"voter","type":"address"}],"name":"voterRewardDestination","outputs":[{"name":"recipient","type":"address"},{"name":"explicitlySet","type":"bool"},{"name":"updatedHeight","type":"uint64"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"voter","type":"address"}],"name":"voterRewardStatus","outputs":[{"name":"targetEra","type":"uint64"},{"name":"eraStartEpoch","type":"uint64"},{"name":"eraEndEpoch","type":"uint64"},{"name":"settlementCompleted","type":"bool"},{"name":"completedHeight","type":"uint64"},{"name":"status","type":"uint8"},{"name":"rewardAmount","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

var (
	_pendingBlockRewardPoolMethod      abi.Method
	_pendingBlockRewardPoolIndexMethod abi.Method
	// Not named epochDrainCursor, for the same reason the snapshot view below is
	// not named voterRewardSnapshot. That name took no arguments, so keeping it
	// would have kept its 4-byte selector while the tuple underneath it changed
	// shape: the per-delegate (delegateIndex, voterIndex, delegateStartIndex,
	// voterStartIndices) quartet is gone, replaced by the shard triple
	// (startShard, shardsDone, resumeVoter). A caller built against the old ABI
	// would decode a shard counter as a delegate index and read a plausible,
	// wrong cursor.
	_eraDrainCursorMethod abi.Method
	// The delegate snapshot view is deliberately not named voterRewardSnapshot.
	// That name once returned (…, address[] voters, uint256[] weights) off the
	// materialized per-voter entry list; the list is gone and the tuple now ends
	// in (uint64 freezeHeight, uint64 selfStakeBucketIdx). Keeping the old name
	// would keep the old 4-byte selector and decode the new tuple as the old one
	// for every caller that never rebuilt. IIP-59 is unactivated, so the old
	// selector is removed outright rather than aliased.
	_voterRewardDelegateSnapshotMethod abi.Method
	_voterRewardAddressMethod          abi.Method
	_voterRewardDestinationMethod      abi.Method
	_voterRewardStatusMethod           abi.Method
)

func init() {
	_pendingBlockRewardPoolMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingBlockRewardPool")
	_pendingBlockRewardPoolIndexMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingBlockRewardPoolIndex")
	_eraDrainCursorMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "eraDrainCursor")
	_voterRewardDelegateSnapshotMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardDelegateSnapshot")
	_voterRewardAddressMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardAddress")
	_voterRewardDestinationMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardDestination")
	_voterRewardStatusMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardStatus")
}

type VoterRewardStatusStateContext struct {
	*protocolctx.BaseStateContext
}

func newVoterRewardStatusStateContext(data []byte) (*VoterRewardStatusStateContext, error) {
	params := make(map[string]interface{})
	if err := _voterRewardStatusMethod.Inputs.UnpackIntoMap(params, data); err != nil {
		return nil, err
	}
	// One argument. The drain pays a voter once for their whole entitlement
	// across every delegate, so there is no per-candidate answer to ask for.
	voter, ok := params["voter"].(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	voterAddress, err := address.FromBytes(voter.Bytes())
	if err != nil {
		return nil, err
	}
	return &VoterRewardStatusStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("VoterRewardStatus"),
		Arguments:  [][]byte{[]byte(voterAddress.String())},
	}}}, nil
}

func (r *VoterRewardStatusStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	status := &rewardingpb.VoterRewardStatus{}
	if err := proto.Unmarshal(resp.Data, status); err != nil {
		return "", err
	}
	data, err := _voterRewardStatusMethod.Outputs.Pack(
		status.GetTargetEra(), status.GetEraStartEpoch(), status.GetEraEndEpoch(),
		status.GetSettlementCompleted(), status.GetCompletedHeight(), uint8(status.GetStatus()),
		new(big.Int).SetBytes(status.GetRewardAmount()),
	)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
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

func newPendingBlockRewardPoolStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _pendingBlockRewardPoolMethod, "PendingBlockRewardPool", "candidateId")
}

// newVoterRewardDelegateSnapshotStateContext builds the per-delegate snapshot
// read. The native ReadState method keeps its original name: only the eth ABI
// tuple changed, and the repo pairs a renamed selector with an unchanged native
// method elsewhere too (candidateByAddressV4 → CANDIDATE_BY_ADDRESS).
func newVoterRewardDelegateSnapshotStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _voterRewardDelegateSnapshotMethod, "VoterRewardSnapshot", "candidateId")
}

func newVoterRewardAddressStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _voterRewardAddressMethod, "VoterRewardAddress", "candidateId")
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
	case _pendingBlockRewardPoolMethod.Name:
		amount, ok := new(big.Int).SetString(string(resp.Data), 10)
		if !ok {
			return "", errConvertBigNumber
		}
		data, err = r.method.Outputs.Pack(amount)
	case _voterRewardDelegateSnapshotMethod.Name:
		// The `voters` and `weights` arrays this used to return are gone with
		// the frozen entry list they came from. Callers that wanted a specific
		// voter's standing call voterRewardStatus(voter), which is per-voter
		// and sums across every delegate the voter is frozen against;
		// freezeHeight and selfStakeBucketIdx are returned in their place so a
		// caller can recompute snapshotHash from the scalars it just read.
		snapshot := &stakingpb.CandidatePollSnapshot{}
		if err = proto.Unmarshal(resp.Data, snapshot); err == nil {
			data, err = r.method.Outputs.Pack(
				snapshot.GetBlockCommissionBasisPoints(), snapshot.GetEpochCommissionBasisPoints(),
				snapshot.GetRegistered(), snapshot.GetOnchainRewardEnabled(),
				new(big.Int).SetBytes(snapshot.GetTotalWeight()), common.BytesToHash(snapshot.GetSnapshotHash()),
				snapshot.GetFreezeHeight(), snapshot.GetSelfStakeBucketIdx(),
			)
		}
	case _voterRewardAddressMethod.Name:
		state := &rewardingpb.VoterRewardAddress{}
		if err = proto.Unmarshal(resp.Data, state); err == nil {
			data, err = r.method.Outputs.Pack(
				common.BytesToAddress(state.GetAddress()), state.GetExplicitlySet(),
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

type PendingBlockRewardPoolIndexStateContext struct {
	*protocolctx.BaseStateContext
}

func newPendingBlockRewardPoolIndexStateContext() (*PendingBlockRewardPoolIndexStateContext, error) {
	return &PendingBlockRewardPoolIndexStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("PendingBlockRewardPoolIndex"),
	}}}, nil
}

func (r *PendingBlockRewardPoolIndexStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	index := &rewardingpb.PendingBlockRewardPoolIndex{}
	if err := proto.Unmarshal(resp.Data, index); err != nil {
		return "", err
	}
	ids := make([]common.Address, len(index.GetCandidateIdentifiers()))
	for i, id := range index.GetCandidateIdentifiers() {
		ids[i] = common.BytesToAddress(id)
	}
	data, err := _pendingBlockRewardPoolIndexMethod.Outputs.Pack(ids)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

type EraDrainCursorStateContext struct {
	*protocolctx.BaseStateContext
}

func newEraDrainCursorStateContext() (*EraDrainCursorStateContext, error) {
	return &EraDrainCursorStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("EpochDrainCursor"),
	}}}, nil
}

func (r *EraDrainCursorStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	cursor := &rewardingpb.EpochDrainCursor{}
	if err := proto.Unmarshal(resp.Data, cursor); err != nil {
		return "", err
	}
	delegates := cursor.GetDelegates()
	ids := make([]common.Address, len(delegates))
	voterAmounts := make([]*big.Int, len(delegates))
	distributedAmounts := make([]*big.Int, len(delegates))
	rewardAddresses := make([]common.Address, len(delegates))
	epochCommissions := make([]*big.Int, len(delegates))
	totalWeights := make([]*big.Int, len(delegates))
	for i, delegate := range delegates {
		ids[i] = common.BytesToAddress(delegate.GetCandidateIdentifier())
		voterAmounts[i] = new(big.Int).SetBytes(delegate.GetVoterAmountFrozen())
		distributedAmounts[i] = new(big.Int).SetBytes(delegate.GetVoterAmountDistributed())
		rewardAddresses[i] = common.BytesToAddress(delegate.GetRewardAddress())
		epochCommissions[i] = new(big.Int).SetBytes(delegate.GetEpochCommission())
		totalWeights[i] = new(big.Int).SetBytes(delegate.GetTotalWeight())
	}
	data, err := _eraDrainCursorMethod.Outputs.Pack(
		cursor.GetTargetEra(), cursor.GetStartEpoch(), cursor.GetEndEpoch(), cursor.GetCompleted(), cursor.GetCompletedHeight(),
		cursor.GetStartShard(), cursor.GetShardsDone(), cursor.GetResumeVoter(),
		common.BytesToHash(cursor.GetSettlementSeed()),
		ids, voterAmounts, distributedAmounts, rewardAddresses, epochCommissions, totalWeights,
	)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
