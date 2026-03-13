// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// ScheduleCandidateDeactivation schedules the deactivation of a candidate
type ScheduleCandidateDeactivation struct {
	delegate address.Address
}

// NewScheduleCandidateDeactivation creates a ScheduleCandidateDeactivation action
func NewScheduleCandidateDeactivation(
	delegate address.Address,
) *ScheduleCandidateDeactivation {
	return &ScheduleCandidateDeactivation{
		delegate: delegate,
	}
}

// LoadProto converts a proto message into action.
func (r *ScheduleCandidateDeactivation) LoadProto(pb *iotextypes.ScheduleCandidateDeactivation) error {
	if pb == nil {
		return ErrNilProto
	}
	if r == nil {
		return ErrNilAction
	}
	delegate, err := address.FromString(pb.GetDelegate())
	if err != nil {
		return err
	}
	r.delegate = delegate

	return nil
}

// FillAction fill the action with core
func (act *ScheduleCandidateDeactivation) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_ScheduleCandidateDeactivation{ScheduleCandidateDeactivation: act.Proto()}
}

// Proto converts the action into a proto message.
func (r *ScheduleCandidateDeactivation) Proto() *iotextypes.ScheduleCandidateDeactivation {
	return &iotextypes.ScheduleCandidateDeactivation{
		Delegate: r.delegate.String(),
	}
}

// Delegate returns the delegate to exit
func (r *ScheduleCandidateDeactivation) Delegate() address.Address { return r.delegate }

// Serialize returns the byte representation of the action.
func (r *ScheduleCandidateDeactivation) Serialize() []byte {
	return byteutil.Must(proto.Marshal(r.Proto()))
}

// IntrinsicGas returns the intrinsic gas of the action
func (r *ScheduleCandidateDeactivation) IntrinsicGas() (uint64, error) {
	return 0, nil
}

// SanityCheck checks the sanity of the action
func (r *ScheduleCandidateDeactivation) SanityCheck() error { return nil }
