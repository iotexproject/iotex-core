// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

// PutPollResult represents put the poll result from gravity chain.
type PutPollResult struct {
	height     uint64
	candidates state.CandidateList
}

// NewPutPollResult instantiates a putting poll result action struct.
func NewPutPollResult(
	height uint64,
	candidates state.CandidateList,
) *PutPollResult {
	return &PutPollResult{
		height:     height,
		candidates: candidates,
	}
}

// LoadProto converts a proto message into put block action.
func (r *PutPollResult) LoadProto(putPollResultPb *iotextypes.PutPollResult) error {
	if putPollResultPb == nil {
		return ErrNilProto
	}
	if r == nil {
		return ErrNilAction
	}
	*r = PutPollResult{}

	r.height = putPollResultPb.Height

	return r.candidates.LoadProto(putPollResultPb.Candidates)
}

func (act *PutPollResult) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_PutPollResult{PutPollResult: act.Proto()}
}

// Proto converts put poll result action into a proto message.
func (r *PutPollResult) Proto() *iotextypes.PutPollResult {
	return &iotextypes.PutPollResult{
		Height:     r.height,
		Candidates: r.candidates.Proto(),
	}
}

// Height returns put poll result height.
func (r *PutPollResult) Height() uint64 { return r.height }

// Candidates returns the list of candidates.
func (r *PutPollResult) Candidates() state.CandidateList { return r.candidates }

// Serialize returns the byte representation of put poll result action.
func (r *PutPollResult) Serialize() []byte {
	return byteutil.Must(proto.Marshal(r.Proto()))
}

// IntrinsicGas returns the intrinsic gas of a put poll result action
func (r *PutPollResult) IntrinsicGas() (uint64, error) {
	return 0, nil
}

func (r *PutPollResult) SanityCheck() error { return nil }
