// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
)

// typedCandidateStateManager is the state manager for typed candidate
type typedCandidateStateManager struct {
	protocol.StateManager
}

func (csm *typedCandidateStateManager) isRegistered(candType CandidateType, operatorAddr address.Address) bool {
	// TODO: implement this
	return false
}

func (csm *typedCandidateStateManager) upsert(cand *TypedCandidate) error {
	// TODO: implement this
	return nil
}
