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
	{"inputs":[],"name":"epochDrainCursor","outputs":[{"name":"targetEra","type":"uint64"},{"name":"delegateIndex","type":"uint32"},{"name":"voterIndex","type":"uint32"},{"name":"candidateIds","type":"address[]"},{"name":"voterAmounts","type":"uint256[]"},{"name":"distributedAmounts","type":"uint256[]"},{"name":"rewardAddresses","type":"address[]"},{"name":"epochCommissions","type":"uint256[]"},{"name":"totalWeights","type":"uint256[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"name":"candidateId","type":"address"}],"name":"voterRewardSnapshot","outputs":[{"name":"blockCommissionBasisPoints","type":"uint64"},{"name":"epochCommissionBasisPoints","type":"uint64"},{"name":"registered","type":"bool"},{"name":"totalWeight","type":"uint256"},{"name":"snapshotHash","type":"bytes32"},{"name":"voters","type":"address[]"},{"name":"weights","type":"uint256[]"}],"stateMutability":"view","type":"function"}
]`

var (
	_pendingBlockRewardPoolMethod      abi.Method
	_pendingBlockRewardPoolIndexMethod abi.Method
	_epochDrainCursorMethod            abi.Method
	_voterRewardSnapshotMethod         abi.Method
)

func init() {
	_pendingBlockRewardPoolMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingBlockRewardPool")
	_pendingBlockRewardPoolIndexMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "pendingBlockRewardPoolIndex")
	_epochDrainCursorMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "epochDrainCursor")
	_voterRewardSnapshotMethod = abiutil.MustLoadMethod(_iip59InterfaceABI, "voterRewardSnapshot")
}

type iip59AddressStateContext struct {
	*protocolctx.BaseStateContext
	method abi.Method
}

func newIIP59AddressStateContext(data []byte, method abi.Method, methodName string) (*iip59AddressStateContext, error) {
	params := make(map[string]interface{})
	if err := method.Inputs.UnpackIntoMap(params, data); err != nil {
		return nil, err
	}
	candidate, ok := params["candidateId"].(common.Address)
	if !ok {
		return nil, errDecodeFailure
	}
	candidateID, err := address.FromBytes(candidate.Bytes())
	if err != nil {
		return nil, err
	}
	return &iip59AddressStateContext{
		BaseStateContext: &protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
			MethodName: []byte(methodName),
			Arguments:  [][]byte{[]byte(candidateID.String())},
		}},
		method: method,
	}, nil
}

func newPendingBlockRewardPoolStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _pendingBlockRewardPoolMethod, "PendingBlockRewardPool")
}

func newVoterRewardSnapshotStateContext(data []byte) (*iip59AddressStateContext, error) {
	return newIIP59AddressStateContext(data, _voterRewardSnapshotMethod, "VoterRewardSnapshot")
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
	case _voterRewardSnapshotMethod.Name:
		snapshot := &stakingpb.CandidatePollSnapshot{}
		if err = proto.Unmarshal(resp.Data, snapshot); err == nil {
			voters := make([]common.Address, len(snapshot.GetEntries()))
			weights := make([]*big.Int, len(snapshot.GetEntries()))
			for i, entry := range snapshot.GetEntries() {
				voters[i] = common.BytesToAddress(entry.GetVoter())
				weights[i] = new(big.Int).SetBytes(entry.GetWeight())
			}
			data, err = r.method.Outputs.Pack(
				snapshot.GetBlockCommissionBasisPoints(), snapshot.GetEpochCommissionBasisPoints(),
				snapshot.GetRegistered(),
				new(big.Int).SetBytes(snapshot.GetTotalWeight()), common.BytesToHash(snapshot.GetSnapshotHash()),
				voters, weights,
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
	index := &rewardingpb.Exempt{}
	if err := proto.Unmarshal(resp.Data, index); err != nil {
		return "", err
	}
	ids := make([]common.Address, len(index.GetAddrs()))
	for i, id := range index.GetAddrs() {
		ids[i] = common.BytesToAddress(id)
	}
	data, err := _pendingBlockRewardPoolIndexMethod.Outputs.Pack(ids)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

type EpochDrainCursorStateContext struct {
	*protocolctx.BaseStateContext
}

func newEpochDrainCursorStateContext() (*EpochDrainCursorStateContext, error) {
	return &EpochDrainCursorStateContext{&protocolctx.BaseStateContext{Parameter: &protocolctx.Parameters{
		MethodName: []byte("EpochDrainCursor"),
	}}}, nil
}

func (r *EpochDrainCursorStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
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
	data, err := _epochDrainCursorMethod.Outputs.Pack(
		cursor.GetTargetEra(), cursor.GetDelegateIndex(), cursor.GetVoterIndex(),
		ids, voterAmounts, distributedAmounts, rewardAddresses, epochCommissions, totalWeights,
	)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
