// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// ChainState is what a request is prepared from: the network and one bucket
// with its guard record, read at Height.
type ChainState struct {
	ChainID     uint32
	GenesisHash [32]byte
	Height      uint64
	// HeightHash is the hash of block Height. An op is anchored to it.
	HeightHash  [32]byte
	BucketIndex uint64
	Owner       address.Address
	Candidate   string
	Amount      string
	// Guard is the bucket's guard record; nil means no record.
	Guard *bucketguard.Guard
}

// Domain returns the transcript domain of the chain.
func (s *ChainState) Domain() bucketguard.Domain {
	return bucketguard.NewDomain(s.ChainID, s.GenesisHash)
}

func (s *ChainState) bucketInfo() BucketInfo {
	return BucketInfo{
		Index:      s.BucketIndex,
		Owner:      s.Owner,
		Candidate:  s.Candidate,
		Amount:     s.Amount,
		ReadHeight: s.Height,
	}
}

// OpParams are the operation-specific inputs of a guarded op.
type OpParams struct {
	Op       bucketguard.Op
	Deadline uint64
	// Sponsor, when set, is the only account that may submit the op; nil
	// makes the signed op a bearer authorization.
	Sponsor address.Address
	// WITHDRAW
	Recipient address.Address
	// TRANSFER
	NewOwner   address.Address
	ClearGuard bool
	// CHANGE_CANDIDATE
	CandidateName       string
	ExpectedCandidateID address.Address
	// RESTAKE
	Duration  uint32
	AutoStake bool
	// TRANSFER without ClearGuard, and ROTATE: the incoming key. nil leaves
	// it pending, to be filled in when the request is signed.
	NewGuardKey *pqcrypto.PublicKey
}

func addrBytes(a address.Address) []byte {
	if a == nil {
		return nil
	}
	return a.Bytes()
}

// NewOpRequest prepares a guarded op on a guarded bucket, anchored to the
// block the state was read at (IIP-65 §6).
func NewOpRequest(s *ChainState, p *OpParams) (*Request, error) {
	if !s.Guard.IsGuarded() {
		return nil, fmt.Errorf("bucket %d is not guarded", s.BucketIndex)
	}
	if s.Owner == nil {
		return nil, errors.New("missing bucket owner")
	}
	if p.Deadline <= s.Height {
		return nil, fmt.Errorf("deadline %d is not after the read height %d", p.Deadline, s.Height)
	}
	if w := bucketguard.ParamsV1().MaxDeadlineWindow; p.Deadline-s.Height > w {
		return nil, fmt.Errorf("deadline %d is more than %d blocks after the anchor block %d", p.Deadline, w, s.Height)
	}
	op := &bucketguard.GuardedBucketOp{
		BucketIndex:         s.BucketIndex,
		Generation:          s.Guard.Generation,
		Nonce:               s.Guard.Nonce,
		Deadline:            p.Deadline,
		AnchorHeight:        s.Height,
		AnchorHash:          bytes.Clone(s.HeightHash[:]),
		Sponsor:             addrBytes(p.Sponsor),
		Op:                  p.Op,
		Recipient:           addrBytes(p.Recipient),
		NewOwner:            addrBytes(p.NewOwner),
		CandidateName:       p.CandidateName,
		ExpectedCandidateID: addrBytes(p.ExpectedCandidateID),
		Duration:            p.Duration,
		AutoStake:           p.AutoStake,
		ClearGuard:          p.ClearGuard,
	}
	if p.Op == bucketguard.OpRotate || (p.Op == bucketguard.OpTransfer && !p.ClearGuard) {
		op.NewGuard = &bucketguard.NewGuard{AlgID: pqcrypto.AlgSLHDSA192s}
		if k := p.NewGuardKey; k != nil {
			op.NewGuard.AlgID, op.NewGuard.PublicKey = k.Alg(), k.Bytes()
		}
	} else if p.NewGuardKey != nil {
		return nil, fmt.Errorf("%s does not install a new guard key", p.Op)
	}
	r := &Request{
		Kind:        KindOp,
		ChainID:     s.ChainID,
		GenesisHash: s.GenesisHash,
		Bucket:      s.bucketInfo(),
		Guard: bucketguard.Guard{
			AlgID:      s.Guard.AlgID,
			PublicKey:  bytes.Clone(s.Guard.PublicKey),
			Generation: s.Guard.Generation,
			Nonce:      s.Guard.Nonce,
		},
		Op: op,
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	return r, nil
}

// NewBindRequest prepares binding a guard to an unguarded bucket. key is the
// guard key to bind; nil leaves it pending.
func NewBindRequest(s *ChainState, key *pqcrypto.PublicKey) (*Request, error) {
	if s.Guard.IsGuarded() {
		return nil, fmt.Errorf("bucket %d is already guarded", s.BucketIndex)
	}
	if s.Owner == nil {
		return nil, errors.New("missing bucket owner")
	}
	r := &Request{
		Kind:        KindBind,
		ChainID:     s.ChainID,
		GenesisHash: s.GenesisHash,
		Bucket:      s.bucketInfo(),
		Bind:        &bucketguard.BindBucketGuard{BucketIndex: s.BucketIndex, AlgID: pqcrypto.AlgSLHDSA192s},
	}
	if s.Guard != nil {
		r.Guard.Generation, r.Guard.Nonce = s.Guard.Generation, s.Guard.Nonce
	}
	if key != nil {
		r.Bind.AlgID, r.Bind.PublicKey = key.Alg(), key.Bytes()
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	return r, nil
}
