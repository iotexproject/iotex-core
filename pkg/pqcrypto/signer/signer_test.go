// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"bytes"
	"testing"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// TestSignIsHedged checks that Sign uses fresh randomness (IIP-65 §12.3):
// two signatures of one message differ and both verify, while
// SignDeterministic, kept for vectors, repeats.
func TestSignIsHedged(t *testing.T) {
	alg := pqcrypto.AlgSLHDSA192s
	k, err := NewKeyFromSeeds(alg, bytes.Repeat([]byte{7}, SeedSize(alg)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, msg := []byte("IoTeX-PQ-Bucket-Call-v1"), []byte("transcript")
	sign := func(f func(context, msg []byte) ([]byte, error)) []byte {
		t.Helper()
		sig, err := f(ctx, msg)
		if err != nil {
			t.Fatal(err)
		}
		if !k.PublicKey().Verify(ctx, msg, sig) {
			t.Fatal("signature does not verify")
		}
		return sig
	}
	if bytes.Equal(sign(k.Sign), sign(k.Sign)) {
		t.Error("Sign returned the same signature twice")
	}
	if !bytes.Equal(sign(k.SignDeterministic), sign(k.SignDeterministic)) {
		t.Error("SignDeterministic is not deterministic")
	}
}
