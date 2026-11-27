// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

// SetIncomingKey fills in a pending incoming key (bind, ROTATE, TRANSFER with
// a new guard). Once set, the key is covered by CALL and cannot change; it
// may only be set again to the same key.
func (r *Request) SetIncomingKey(pub *pqcrypto.PublicKey) error {
	if !r.NeedsAccept() {
		return errors.New("the request installs no guard key")
	}
	alg, cur := r.incoming()
	if len(cur) > 0 {
		if alg != pub.Alg() || !bytes.Equal(cur, pub.Bytes()) {
			return errors.New("the incoming key of the request is a different key")
		}
		return nil
	}
	if r.CallSigned() {
		return errors.New("CALL is signed; the incoming key cannot be set")
	}
	if r.Kind == KindBind {
		r.Bind.AlgID, r.Bind.PublicKey = pub.Alg(), pub.Bytes()
	} else {
		r.Op.NewGuard.AlgID, r.Op.NewGuard.PublicKey = pub.Alg(), pub.Bytes()
	}
	return nil
}

func (r *Request) acceptMessage() ([]byte, error) {
	alg, pub := r.incoming()
	if len(pub) == 0 {
		return nil, errors.New("the incoming key is pending")
	}
	owner, err := r.ResultingOwner()
	if err != nil {
		return nil, err
	}
	var h [bucketguard.CallHashLen]byte
	if r.Kind == KindOp {
		h, _ = r.CallHash()
	}
	return bucketguard.EncodeAccept(r.Domain(), r.Bucket.Index, r.Guard.Generation+1, owner, alg, pub, h), nil
}

// SignAccept signs ACCEPT with the incoming key k, filling in a pending
// incoming key while CALL is unsigned. The signature is verified before it is
// stored.
func SignAccept(r *Request, k *signer.PrivateKey) error {
	if err := r.SetIncomingKey(k.PublicKey()); err != nil {
		return err
	}
	msg, err := r.acceptMessage()
	if err != nil {
		return err
	}
	sig, err := k.Sign([]byte(bucketguard.AcceptContext), msg)
	if err != nil {
		return err
	}
	if !k.PublicKey().Verify([]byte(bucketguard.AcceptContext), msg, sig) {
		return errors.New("ACCEPT signature does not verify")
	}
	if r.Kind == KindBind {
		r.Bind.AcceptSig = sig
	} else {
		r.Op.NewGuard.AcceptSig = sig
	}
	return nil
}

// SignCall signs CALL with k, which must be the bucket's current guard key.
// The signature is verified before it is stored.
func SignCall(r *Request, k *signer.PrivateKey) error {
	if r.Kind != KindOp {
		return errors.New("a bind has no CALL")
	}
	if r.IncomingPending() {
		return errors.New("the incoming key is pending; set it before signing CALL")
	}
	cur, err := r.CurrentGuard()
	if err != nil {
		return err
	}
	if !cur.Equal(k.PublicKey()) {
		return errors.New("the signing key is not the bucket's current guard key")
	}
	msg := bucketguard.EncodeCall(r.Domain(), r.Bucket.Owner, r.Op)
	sig, err := k.Sign([]byte(bucketguard.CallContext), msg)
	if err != nil {
		return err
	}
	if !cur.Verify([]byte(bucketguard.CallContext), msg, sig) {
		return errors.New("CALL signature does not verify")
	}
	r.Op.Sig = sig
	return nil
}

// SigStatus is the state of one signature of a request.
type SigStatus string

// Signature states.
const (
	SigNotNeeded SigStatus = "not needed"
	SigMissing   SigStatus = "missing"
	SigValid     SigStatus = "valid"
	SigInvalid   SigStatus = "INVALID"
)

// Signatures reports the state of CALL and ACCEPT, verifying each present one.
func Signatures(r *Request) (call, accept SigStatus) {
	call, accept = SigNotNeeded, SigNotNeeded
	if r.Kind == KindOp {
		call = SigMissing
		if r.CallSigned() {
			call = SigInvalid
			if pk, err := r.CurrentGuard(); err == nil &&
				pk.Verify([]byte(bucketguard.CallContext), bucketguard.EncodeCall(r.Domain(), r.Bucket.Owner, r.Op), r.Op.Sig) {
				call = SigValid
			}
		}
	}
	if r.NeedsAccept() {
		accept = SigMissing
		if r.AcceptSigned() {
			accept = SigInvalid
			pk, err := r.IncomingKey()
			msg, merr := r.acceptMessage()
			if err == nil && merr == nil && pk != nil && pk.Verify([]byte(bucketguard.AcceptContext), msg, r.acceptSig()) {
				accept = SigValid
			}
		}
	}
	return call, accept
}

// Verify checks every present signature. When the request is complete it also
// runs the node's checks that need no state: bucketguard.VerifyOp against the
// guard of the request, and the action's SanityCheck.
func Verify(r *Request) error {
	call, accept := Signatures(r)
	if call == SigInvalid {
		return fmt.Errorf("%w: CALL", bucketguard.ErrGuardSignature)
	}
	if accept == SigInvalid {
		return fmt.Errorf("%w: ACCEPT", bucketguard.ErrGuardSignature)
	}
	if !r.Complete() {
		return nil
	}
	if r.Kind == KindBind {
		return action.NewBindBucketGuard(r.Bind).SanityCheck()
	}
	g := r.Guard
	if _, err := bucketguard.VerifyOp(bucketguard.ParamsV1(), r.Domain(), r.Bucket.Owner, &g, r.Op); err != nil {
		return err
	}
	return action.NewGuardedBucketOp(r.Op).SanityCheck()
}

// Action is the action a complete request is submitted as.
type Action interface {
	action.EthCompatibleAction
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}

// Payload returns the action of a complete request.
func Payload(r *Request) (Action, error) {
	if !r.Complete() {
		return nil, errors.New("the request is not fully signed")
	}
	if r.Kind == KindBind {
		return action.NewBindBucketGuard(r.Bind), nil
	}
	return action.NewGuardedBucketOp(r.Op), nil
}
