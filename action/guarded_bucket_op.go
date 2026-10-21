// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// GuardedBucketOp performs an operation authorized by a bucket guard.
type GuardedBucketOp struct {
	bucketguard.GuardedBucketOp
}

// NewGuardedBucketOp returns a GuardedBucketOp action.
func NewGuardedBucketOp(op *bucketguard.GuardedBucketOp) *GuardedBucketOp {
	return &GuardedBucketOp{GuardedBucketOp: *op}
}

// Serialize returns a raw byte stream of the action.
func (g *GuardedBucketOp) Serialize() []byte {
	return byteutil.Must(proto.Marshal(g.Proto()))
}

// FillAction fills the action core.
func (g *GuardedBucketOp) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_GuardedBucketOp{GuardedBucketOp: g.Proto()}
}

// Proto converts to the protobuf action.
func (g *GuardedBucketOp) Proto() *iotextypes.GuardedBucketOp {
	pb := &iotextypes.GuardedBucketOp{
		BucketIndex:         g.BucketIndex,
		Generation:          g.Generation,
		Nonce:               g.Nonce,
		Deadline:            g.Deadline,
		Op:                  g.Op,
		Recipient:           append([]byte(nil), g.Recipient...),
		NewOwner:            append([]byte(nil), g.NewOwner...),
		CandidateName:       g.CandidateName,
		ExpectedCandidateID: append([]byte(nil), g.ExpectedCandidateID...),
		Duration:            g.Duration,
		AutoStake:           g.AutoStake,
		ClearGuard:          g.ClearGuard,
		Sig:                 append([]byte(nil), g.Sig...),
	}
	if g.NewGuard != nil {
		pb.NewGuard = &iotextypes.NewGuard{
			AlgID:     g.NewGuard.AlgID,
			PublicKey: append([]byte(nil), g.NewGuard.PublicKey...),
			AcceptSig: append([]byte(nil), g.NewGuard.AcceptSig...),
		}
	}
	return pb
}

// LoadProto converts a protobuf action.
func (g *GuardedBucketOp) LoadProto(pbAct *iotextypes.GuardedBucketOp) error {
	if pbAct == nil {
		return ErrNilProto
	}
	g.BucketIndex = pbAct.GetBucketIndex()
	g.Generation = pbAct.GetGeneration()
	g.Nonce = pbAct.GetNonce()
	g.Deadline = pbAct.GetDeadline()
	g.Op = pbAct.GetOp()
	g.Recipient = append([]byte(nil), pbAct.GetRecipient()...)
	g.NewOwner = append([]byte(nil), pbAct.GetNewOwner()...)
	g.CandidateName = pbAct.GetCandidateName()
	g.ExpectedCandidateID = append([]byte(nil), pbAct.GetExpectedCandidateID()...)
	g.Duration = pbAct.GetDuration()
	g.AutoStake = pbAct.GetAutoStake()
	g.ClearGuard = pbAct.GetClearGuard()
	g.Sig = append([]byte(nil), pbAct.GetSig()...)
	g.NewGuard = nil
	if pbAct.GetNewGuard() != nil {
		g.NewGuard = &bucketguard.NewGuard{
			AlgID:     pbAct.GetNewGuard().GetAlgID(),
			PublicKey: append([]byte(nil), pbAct.GetNewGuard().GetPublicKey()...),
			AcceptSig: append([]byte(nil), pbAct.GetNewGuard().GetAcceptSig()...),
		}
	}
	return nil
}

// IntrinsicGas returns the canonical intrinsic gas.
func (g *GuardedBucketOp) IntrinsicGas() (uint64, error) {
	return g.GuardedBucketOp.IntrinsicGas(), nil
}

// SanityCheck performs stateless validation.
func (g *GuardedBucketOp) SanityCheck() error {
	if err := g.Validate(); err != nil {
		return errors.Wrap(ErrInvalidAct, err.Error())
	}
	return nil
}
