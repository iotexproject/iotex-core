// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// checkComplete verifies a fully signed request, also after a JSON round
// trip, and its action's stateless checks.
func checkComplete(t *testing.T, req *Request) {
	r := require.New(t)
	r.True(req.Complete())
	r.NoError(Verify(req))
	out := roundTrip(t, req)
	r.NoError(Verify(out))
	call, accept := Signatures(out)
	r.NotEqual(SigInvalid, call)
	r.NotEqual(SigInvalid, accept)
	a, err := Payload(out)
	r.NoError(err)
	r.NoError(a.SanityCheck())
	gas, err := a.IntrinsicGas()
	r.NoError(err)
	r.Greater(gas, uint64(10_000))
	data, err := a.EthData()
	r.NoError(err)
	r.NotEmpty(data)
}

func TestSignOps(t *testing.T) {
	a0, a1, _ := keys(t)
	for _, o := range []bucketguard.Op{bucketguard.OpUnstake, bucketguard.OpWithdraw,
		bucketguard.OpChangeCandidate, bucketguard.OpRestake} {
		t.Run(o.String(), func(t *testing.T) {
			t.Parallel()
			r := require.New(t)
			req, err := NewOpRequest(testState(t), testParams(t, o))
			r.NoError(err)
			r.False(req.Complete())
			r.NoError(Verify(req), "an unsigned request has no invalid signature")
			_, err = Payload(req)
			r.Error(err)
			// Only the current guard key signs CALL.
			r.ErrorContains(SignCall(req, a1), "not the bucket's current guard key")
			r.ErrorContains(SignAccept(req, a1), "installs no guard key")
			r.NoError(SignCall(req, a0))
			checkComplete(t, req)

			// Any change to a signed field breaks CALL.
			tampered := roundTrip(t, req)
			tampered.Op.Deadline++
			r.ErrorIs(Verify(tampered), bucketguard.ErrGuardSignature)
			tampered = roundTrip(t, req)
			tampered.Bucket.Owner = identityset.Address(9)
			r.ErrorIs(Verify(tampered), bucketguard.ErrGuardSignature)
			tampered = roundTrip(t, req)
			tampered.Op.Sig[0] ^= 1
			call, _ := Signatures(tampered)
			r.Equal(SigInvalid, call)
		})
	}
	t.Run("TRANSFER clear", func(t *testing.T) {
		t.Parallel()
		r := require.New(t)
		req, err := NewOpRequest(testState(t), &OpParams{
			Op: bucketguard.OpTransfer, Deadline: 1100, NewOwner: identityset.Address(3), ClearGuard: true,
		})
		r.NoError(err)
		r.False(req.NeedsAccept())
		r.NoError(SignCall(req, a0))
		checkComplete(t, req)
		r.Contains(Summary(req).String(), "clearGuard removes the guard")
	})
	t.Run("ROTATE pending", func(t *testing.T) {
		t.Parallel()
		r := require.New(t)
		req, err := NewOpRequest(testState(t), &OpParams{Op: bucketguard.OpRotate, Deadline: 1100})
		r.NoError(err)
		r.True(req.IncomingPending())
		r.ErrorContains(SignCall(req, a0), "pending")
		// The incoming key's ACCEPT fills in the key, then CALL covers it.
		r.NoError(SignAccept(req, a1))
		r.False(req.IncomingPending())
		req = roundTrip(t, req)
		r.ErrorContains(req.SetIncomingKey(a0.PublicKey()), "different key")
		r.NoError(SignCall(req, a0))
		checkComplete(t, req)
		_, accept := Signatures(req)
		r.Equal(SigValid, accept)
	})
}

func reparse(r *Request) (*Request, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var out Request
	return &out, json.Unmarshal(b, &out)
}

// TestSignTransferTwoVaults signs a TRANSFER to a key of another vault, with
// ACCEPT first and with CALL first; neither signature covers the other.
func TestSignTransferTwoVaults(t *testing.T) {
	r := require.New(t)
	a0, _, b0 := keys(t)
	s := testState(t)

	type result struct {
		req *Request
		err error
	}
	acceptFirst, callFirst := make(chan result, 1), make(chan result, 1)
	go func() {
		// New owner's device: ACCEPT; then the file goes to the current owner.
		p := testParams(t, bucketguard.OpTransfer)
		req, err := NewOpRequest(s, p)
		if err == nil {
			err = SignAccept(req, b0)
		}
		if err == nil {
			req, err = reparse(req)
		}
		if err == nil {
			if call, _ := Signatures(req); call != SigMissing {
				t.Errorf("CALL = %s, want missing", call)
			}
			err = SignCall(req, a0)
		}
		acceptFirst <- result{req, err}
	}()
	go func() {
		p := testParams(t, bucketguard.OpTransfer)
		p.Deadline++ // a different CALL
		req, err := NewOpRequest(s, p)
		if err == nil {
			err = SignCall(req, a0)
		}
		if err == nil {
			req, err = reparse(req)
		}
		if err == nil {
			// The new owner checks the existing CALL before accepting.
			if err = Verify(req); err == nil {
				err = SignAccept(req, b0)
			}
		}
		callFirst <- result{req, err}
	}()
	ra, rc := <-acceptFirst, <-callFirst
	r.NoError(ra.err)
	r.NoError(rc.err)
	checkComplete(t, ra.req)
	checkComplete(t, rc.req)
	r.Equal(identityset.Address(3).String(), Summary(ra.req).ResultingOwner)

	// An ACCEPT is bound to its CALL: swapping it into another CALL fails.
	swapped := roundTrip(t, rc.req)
	swapped.Op.NewGuard.AcceptSig = bytes.Clone(ra.req.Op.NewGuard.AcceptSig)
	r.ErrorIs(Verify(swapped), bucketguard.ErrGuardSignature)
	_, accept := Signatures(swapped)
	r.Equal(SigInvalid, accept)
	// ACCEPT names the resulting owner.
	other := roundTrip(t, ra.req)
	other.Op.NewOwner = identityset.Address(4).Bytes()
	_, accept = Signatures(other)
	r.Equal(SigInvalid, accept)
	// The vault-A key cannot accept for the vault-B key of the request.
	unsigned, err := NewOpRequest(s, testParams(t, bucketguard.OpTransfer))
	r.NoError(err)
	r.ErrorContains(SignAccept(unsigned, a0), "different key")
}

func TestSignBind(t *testing.T) {
	r := require.New(t)
	a0, a1, _ := keys(t)
	s := testState(t)
	s.Guard = &bucketguard.Guard{Generation: 2, Nonce: 1} // a tombstone
	req, err := NewBindRequest(s, nil)
	r.NoError(err)
	r.True(req.IncomingPending())
	r.ErrorContains(SignCall(req, a0), "no CALL")
	r.NoError(SignAccept(req, a0))
	checkComplete(t, req)
	m := Summary(req)
	r.Equal("BIND", m.Op)
	r.Contains(m.String(), "irreversible")

	// ACCEPT of a bind commits to the next generation.
	stale := roundTrip(t, req)
	stale.Guard.Generation = 1
	r.ErrorIs(Verify(stale), bucketguard.ErrGuardSignature)
	r.ErrorContains(roundTrip(t, req).SetIncomingKey(a1.PublicKey()), "different key")
}

func TestSummary(t *testing.T) {
	r := require.New(t)
	s := testState(t)
	p := testParams(t, bucketguard.OpChangeCandidate)
	p.Deadline = s.Height + MaxSafeDeadlineBlocks + 1
	req, err := NewOpRequest(s, p)
	r.NoError(err)
	m := Summary(req)
	r.Equal("testnet", m.Network)
	r.Equal("CHANGE_CANDIDATE", m.Op)
	r.Equal(SigMissing, m.Call)
	r.Equal(SigNotNeeded, m.Accept)
	r.False(m.Complete)
	text := m.String()
	for _, want := range []string{"CHANGE_CANDIDATE on bucket 7", "testnet (chain 2", "1234.5 IOTX",
		"candidateName:", "bot4", "expectedCandidateID:", "generation/nonce:", "3/5", "bearer authorization",
		"blocks (≈ 12h0m0s) after the read height"} {
		r.Contains(text, want)
	}
	r.False(strings.Contains(text, "ACCEPT:"))
	b, err := json.Marshal(m)
	r.NoError(err)
	r.Contains(string(b), `"expectedCandidateID":"`+identityset.Address(4).String())

	r.Equal("1 IOTX", FormatRau("1000000000000000000"))
	r.Equal("0.000000000000000001 IOTX", FormatRau("1"))
	r.Equal("x", FormatRau("x"))
}
