// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// The two actions of the IIP draft "Quantum-Safe Staking Buckets". They are
// thin wrappers: the rules, the canonical size and the gas live in
// bucketguard. Gas is fixed to bucketguard.ParamsV1 until a later parameter
// version is activated by a fork.

var (
	_ EthCompatibleAction = (*BindBucketGuard)(nil)
	_ EthCompatibleAction = (*GuardedBucketOp)(nil)
)

// BindBucketGuard attaches a post-quantum guard key to a native bucket.
type BindBucketGuard struct {
	stake_common
	act bucketguard.BindBucketGuard
}

// NewBindBucketGuard wraps m.
func NewBindBucketGuard(m *bucketguard.BindBucketGuard) *BindBucketGuard {
	return &BindBucketGuard{act: *m}
}

// Guard returns the bucketguard form of the action.
func (a *BindBucketGuard) Guard() *bucketguard.BindBucketGuard { return &a.act }

// BucketIndex returns the bucket index.
func (a *BindBucketGuard) BucketIndex() uint64 { return a.act.BucketIndex }

// IntrinsicGas prices the action over its canonical size (IIP §8).
func (a *BindBucketGuard) IntrinsicGas() (uint64, error) {
	return a.act.IntrinsicGas(bucketguard.ParamsV1()), nil
}

// SanityCheck runs the stateless checks.
func (a *BindBucketGuard) SanityCheck() error {
	if err := a.act.Validate(bucketguard.ParamsV1()); err != nil {
		return errors.Wrap(ErrInvalidAct, err.Error())
	}
	return nil
}

// FillAction fills the action core.
func (a *BindBucketGuard) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_BindBucketGuard{BindBucketGuard: a.Proto()}
}

// Proto converts to protobuf.
func (a *BindBucketGuard) Proto() *iotextypes.BindBucketGuard {
	return &iotextypes.BindBucketGuard{
		BucketIndex: a.act.BucketIndex,
		AlgID:       uint32(a.act.AlgID),
		PublicKey:   bytes.Clone(a.act.PublicKey),
		AcceptSig:   bytes.Clone(a.act.AcceptSig),
	}
}

// LoadProto converts from protobuf.
func (a *BindBucketGuard) LoadProto(pb *iotextypes.BindBucketGuard) error {
	if pb == nil {
		return ErrNilProto
	}
	a.act = bucketguard.BindBucketGuard{
		BucketIndex: pb.GetBucketIndex(),
		AlgID:       pqcrypto.AlgID(pb.GetAlgID()),
		PublicKey:   bytes.Clone(pb.GetPublicKey()),
		AcceptSig:   bytes.Clone(pb.GetAcceptSig()),
	}
	return nil
}

// EthData returns the ABI-encoded data for the web3 entry.
func (a *BindBucketGuard) EthData() ([]byte, error) { return a.act.EthData() }

// NewBindBucketGuardFromABIBinary decodes a web3 bindBucketGuard call.
func NewBindBucketGuardFromABIBinary(data []byte) (*BindBucketGuard, error) {
	m, err := bucketguard.BindBucketGuardFromEthData(data)
	if err != nil {
		return nil, err
	}
	return NewBindBucketGuard(m), nil
}

// GuardedBucketOp performs one guard-authorized operation on a bucket. The
// sender is only a sponsor.
type GuardedBucketOp struct {
	stake_common
	act bucketguard.GuardedBucketOp
}

// NewGuardedBucketOp wraps m.
func NewGuardedBucketOp(m *bucketguard.GuardedBucketOp) *GuardedBucketOp {
	return &GuardedBucketOp{act: *m}
}

// Guard returns the bucketguard form of the action.
func (a *GuardedBucketOp) Guard() *bucketguard.GuardedBucketOp { return &a.act }

// BucketIndex returns the bucket index.
func (a *GuardedBucketOp) BucketIndex() uint64 { return a.act.BucketIndex }

// IntrinsicGas prices the action over its canonical size (IIP §8).
func (a *GuardedBucketOp) IntrinsicGas() (uint64, error) {
	return a.act.IntrinsicGas(bucketguard.ParamsV1()), nil
}

// SanityCheck runs the stateless guard checks. The staking business rules of
// the underlying operation are checked by the staking protocol.
func (a *GuardedBucketOp) SanityCheck() error {
	if err := a.act.Validate(bucketguard.ParamsV1()); err != nil {
		return errors.Wrap(ErrInvalidAct, err.Error())
	}
	return nil
}

// FillAction fills the action core.
func (a *GuardedBucketOp) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_GuardedBucketOp{GuardedBucketOp: a.Proto()}
}

// Proto converts to protobuf.
func (a *GuardedBucketOp) Proto() *iotextypes.GuardedBucketOp {
	m := &a.act
	pb := &iotextypes.GuardedBucketOp{
		BucketIndex:         m.BucketIndex,
		Generation:          m.Generation,
		Nonce:               m.Nonce,
		Deadline:            m.Deadline,
		Op:                  uint32(m.Op),
		Recipient:           bytes.Clone(m.Recipient),
		NewOwner:            bytes.Clone(m.NewOwner),
		CandidateName:       m.CandidateName,
		ExpectedCandidateID: bytes.Clone(m.ExpectedCandidateID),
		Duration:            m.Duration,
		AutoStake:           m.AutoStake,
		ClearGuard:          m.ClearGuard,
		Sig:                 bytes.Clone(m.Sig),
	}
	if m.NewGuard != nil {
		pb.NewGuard = &iotextypes.NewGuard{
			AlgID:     uint32(m.NewGuard.AlgID),
			PublicKey: bytes.Clone(m.NewGuard.PublicKey),
			AcceptSig: bytes.Clone(m.NewGuard.AcceptSig),
		}
	}
	return pb
}

// LoadProto converts from protobuf.
func (a *GuardedBucketOp) LoadProto(pb *iotextypes.GuardedBucketOp) error {
	if pb == nil {
		return ErrNilProto
	}
	a.act = bucketguard.GuardedBucketOp{
		BucketIndex:         pb.GetBucketIndex(),
		Generation:          pb.GetGeneration(),
		Nonce:               pb.GetNonce(),
		Deadline:            pb.GetDeadline(),
		Op:                  bucketguard.Op(pb.GetOp()),
		Recipient:           bytes.Clone(pb.GetRecipient()),
		NewOwner:            bytes.Clone(pb.GetNewOwner()),
		CandidateName:       pb.GetCandidateName(),
		ExpectedCandidateID: bytes.Clone(pb.GetExpectedCandidateID()),
		Duration:            pb.GetDuration(),
		AutoStake:           pb.GetAutoStake(),
		ClearGuard:          pb.GetClearGuard(),
		Sig:                 bytes.Clone(pb.GetSig()),
	}
	if ng := pb.GetNewGuard(); ng != nil {
		a.act.NewGuard = &bucketguard.NewGuard{
			AlgID:     pqcrypto.AlgID(ng.GetAlgID()),
			PublicKey: bytes.Clone(ng.GetPublicKey()),
			AcceptSig: bytes.Clone(ng.GetAcceptSig()),
		}
	}
	return nil
}

// EthData returns the ABI-encoded data for the web3 entry.
func (a *GuardedBucketOp) EthData() ([]byte, error) { return a.act.EthData() }

// NewGuardedBucketOpFromABIBinary decodes a web3 guarded* call.
func NewGuardedBucketOpFromABIBinary(data []byte) (*GuardedBucketOp, error) {
	m, err := bucketguard.GuardedBucketOpFromEthData(data)
	if err != nil {
		return nil, err
	}
	return NewGuardedBucketOp(m), nil
}
