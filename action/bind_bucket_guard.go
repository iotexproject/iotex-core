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

// BindBucketGuard attaches a quantum-safe guard to a native staking bucket.
type BindBucketGuard struct {
	bucketguard.BindBucketGuard
}

// NewBindBucketGuard returns a BindBucketGuard action.
func NewBindBucketGuard(bucketIndex uint64, algID uint32, publicKey, acceptSig []byte) *BindBucketGuard {
	return &BindBucketGuard{
		BindBucketGuard: bucketguard.BindBucketGuard{
			BucketIndex: bucketIndex,
			AlgID:       algID,
			PublicKey:   publicKey,
			AcceptSig:   acceptSig,
		},
	}
}

// Serialize returns a raw byte stream of the action.
func (b *BindBucketGuard) Serialize() []byte {
	return byteutil.Must(proto.Marshal(b.Proto()))
}

// FillAction fills the action core.
func (b *BindBucketGuard) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_BindBucketGuard{BindBucketGuard: b.Proto()}
}

// Proto converts to the protobuf action.
func (b *BindBucketGuard) Proto() *iotextypes.BindBucketGuard {
	return &iotextypes.BindBucketGuard{
		BucketIndex: b.BucketIndex,
		AlgID:       b.AlgID,
		PublicKey:   append([]byte(nil), b.PublicKey...),
		AcceptSig:   append([]byte(nil), b.AcceptSig...),
	}
}

// LoadProto converts a protobuf action.
func (b *BindBucketGuard) LoadProto(pbAct *iotextypes.BindBucketGuard) error {
	if pbAct == nil {
		return ErrNilProto
	}
	b.BucketIndex = pbAct.GetBucketIndex()
	b.AlgID = pbAct.GetAlgID()
	b.PublicKey = append([]byte(nil), pbAct.GetPublicKey()...)
	b.AcceptSig = append([]byte(nil), pbAct.GetAcceptSig()...)
	return nil
}

// IntrinsicGas returns the canonical intrinsic gas.
func (b *BindBucketGuard) IntrinsicGas() (uint64, error) {
	return b.BindBucketGuard.IntrinsicGas(), nil
}

// SanityCheck performs stateless validation.
func (b *BindBucketGuard) SanityCheck() error {
	if err := b.Validate(); err != nil {
		return errors.Wrap(ErrInvalidAct, err.Error())
	}
	return nil
}
