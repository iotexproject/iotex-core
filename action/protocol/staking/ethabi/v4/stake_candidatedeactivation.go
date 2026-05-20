// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package v4 historically hosts versioned bundled-struct getters
// (candidateByAddressV4, candidatesV4, ...). This file deliberately does NOT
// follow that pattern: candidateDeactivation is a single-purpose getter
// scoped to the Yap-hardfork exit queue feature.
//
// The ABI convention going forward is:
//
//   - The big bundled struct getters (candidateByAddress / candidateBy*)
//     are FROZEN at V4 — new fields will not be appended. New consumers
//     pair the V4 result with a focused getter via multicall.
//
//   - New per-feature state is exposed through its own getter that returns
//     only the fields it owns. The selector lives forever; the bundled
//     struct ABI doesn't churn.
//
// This getter piggy-backs on the existing CANDIDATE_BY_ADDRESS read so it
// needs no new ReadStakingDataMethod enum nor a new state-reader handler;
// only the ABI marshalling layer differs.
package v4

import (
	"encoding/hex"
	"math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

// candidateExitRequestedSentinel mirrors the chain-side sentinel used to
// encode "Request received, schedule pending" in Candidate.DeactivatedAt.
// Kept local so the ABI layer doesn't reach into the staking handler package.
const candidateExitRequestedSentinel = uint64(math.MaxUint64)

const _candidateDeactivationInterfaceABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "ownerAddress",
				"type": "address"
			}
		],
		"name": "candidateDeactivation",
		"outputs": [
			{
				"internalType": "bool",
				"name": "requested",
				"type": "bool"
			},
			{
				"internalType": "uint64",
				"name": "scheduledAtBlock",
				"type": "uint64"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _candidateDeactivationMethod abi.Method

func init() {
	_candidateDeactivationMethod = abiutil.MustLoadMethod(_candidateDeactivationInterfaceABI, "candidateDeactivation")
}

// candidateDeactivationStateContext piggy-backs on the existing
// CANDIDATE_BY_ADDRESS state reader (which already returns CandidateV2 with
// deactivatedAt populated since proto field 10) but re-encodes the ABI
// output to expose only the exit-queue fields.
type candidateDeactivationStateContext struct {
	*protocol.BaseStateContext
}

func newCandidateDeactivationStateContext(data []byte) (*candidateDeactivationStateContext, error) {
	paramsMap := map[string]interface{}{}
	if err := _candidateDeactivationMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	ownerAddress, ok := paramsMap["ownerAddress"].(common.Address)
	if !ok {
		return nil, stakingComm.ErrDecodeFailure
	}
	owner, err := address.FromBytes(ownerAddress[:])
	if err != nil {
		return nil, err
	}

	method := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS,
	}
	methodBytes, err := proto.Marshal(method)
	if err != nil {
		return nil, err
	}
	arguments := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				OwnerAddr: owner.String(),
			},
		},
	}
	argumentsBytes, err := proto.Marshal(arguments)
	if err != nil {
		return nil, err
	}
	return &candidateDeactivationStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: &_candidateDeactivationMethod,
		},
	}, nil
}

// EncodeToEth strips the CandidateV2 response down to (requested, scheduledAtBlock).
//
//	deactivatedAt = 0                 → requested=false, scheduledAtBlock=0
//	deactivatedAt = MaxUint64 sentinel → requested=true,  scheduledAtBlock=0
//	deactivatedAt = N (>0, not MAX)   → requested=true,  scheduledAtBlock=N
//
// The sentinel decode keeps clients from having to know the in-chain magic value.
func (r *candidateDeactivationStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var cand iotextypes.CandidateV2
	if err := proto.Unmarshal(resp.Data, &cand); err != nil {
		return "", err
	}
	var (
		requested        bool
		scheduledAtBlock uint64
	)
	switch cand.DeactivatedAt {
	case 0:
		// no exit in flight; defaults already correct
	case candidateExitRequestedSentinel:
		requested = true
	default:
		requested = true
		scheduledAtBlock = cand.DeactivatedAt
	}

	data, err := r.Method.Outputs.Pack(requested, scheduledAtBlock)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
