// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"encoding/hex"
	"testing"
)

// Vectors from docs/qsb-spec-and-vectors.md §9 (wallet profile QSB-KDF-v1).
var qsbTestSeed, _ = hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
	"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f")

func TestDeriveQSBKeyVectors(t *testing.T) {
	for _, tc := range []struct {
		account, index uint32
		pub            string
	}{
		{0, 0, "c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a"},
		{3, 2, "90d6316353ed3740537e3697662900b4f5e21aafeef9e6706df36603691ed44ae7aaf5d06edf82b68587becd64e17a9c"},
	} {
		k, err := DeriveQSBKey(qsbTestSeed, tc.account, tc.index)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(k.PublicKey().Bytes()); got != tc.pub {
			t.Fatalf("account %d index %d: pub = %s, want %s", tc.account, tc.index, got, tc.pub)
		}
	}
	if _, err := DeriveQSBKey(qsbTestSeed, 0, MaxQSBKeyIndex+1); err == nil {
		t.Fatal("index above MaxQSBKeyIndex accepted")
	}
}

func TestRecoverQSBKeyIndex(t *testing.T) {
	target, err := DeriveQSBKey(qsbTestSeed, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	idx, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 3, 4, target.PublicKey())
	if err != nil || !ok || idx != 2 {
		t.Fatalf("recover = (%d, %v, %v), want (2, true, nil)", idx, ok, err)
	}
	if _, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 0, 1, target.PublicKey()); err != nil || ok {
		t.Fatalf("recover on wrong account = (%v, %v), want (false, nil)", ok, err)
	}
}
