// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"encoding/hex"
	"testing"

	"github.com/tyler-smith/go-bip39"
)

// Vectors from docs/qsb-spec-and-vectors.md §9 (wallet profile QSB-KDF-v1).
// The first 24 bytes of each public key are PK.seed = okm[48:72], which can
// be checked with HKDF alone.
var qsbTestSeed, _ = hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
	"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f")

const qsbTestMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func TestDeriveQSBKeyVectors(t *testing.T) {
	for _, tc := range []struct {
		name         string
		seed         []byte
		vault, index uint32
		pub          string
	}{
		{"seed 0/0", qsbTestSeed, 0, 0, "19cefae20e485beb73cd3baf3c84b9e452018c51850faac1e419836bc96f267a5913d095b8d796eecf68f4ca43eed03f"},
		{"seed 3/2", qsbTestSeed, 3, 2, "4384023dc938bad1b2397fe831a94bd6215e8cfad1c7a2074147d24e68cc0c0f9f13216c66e524af9d77972dcf6e87c5"},
		{"beyond the scan window", qsbTestSeed, 0, DefaultQSBScanWindow, "953096e87852f078eb9e759930ffe901aa1d956c6275d3d5feaf66e629287fb976157c6263dbd4350a4263f9acc3311b"},
		{"max key index", qsbTestSeed, 0, 0xffffffff, "758637538cc8ae78652d57d9b200c86bafb63c3735574f3713ab8d545e03293a93d9a9af340663f38d05211234002e31"},
		{"mnemonic", bip39.NewSeed(qsbTestMnemonic, ""), 0, 0, "b06841df29502ec67c3160c530f7b7d26e0a76838cced6e77b545a16fb3d435ce5bac69926bc389a83d32a7c6f2876ee"},
		{"mnemonic with passphrase", bip39.NewSeed(qsbTestMnemonic, "TREZOR"), 0, 0, "34fbd17dc298c306ca1d49a7cc9624a201bb7e132434b190201d121d4fbd886a422c45c509e0d143edb350a1e1b1f417"},
	} {
		k, err := DeriveQSBKey(tc.seed, tc.vault, tc.index)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(k.PublicKey().Bytes()); got != tc.pub {
			t.Errorf("%s: pub = %s, want %s", tc.name, got, tc.pub)
		}
	}
}

func TestRecoverQSBKeyIndex(t *testing.T) {
	target, err := DeriveQSBKey(qsbTestSeed, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	idx, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 3, 0, 4, target.PublicKey())
	if err != nil || !ok || idx != 2 {
		t.Fatalf("recover = (%d, %v, %v), want (2, true, nil)", idx, ok, err)
	}
	if _, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 3, 3, 4, target.PublicKey()); err != nil || ok {
		t.Fatalf("recover outside the range = (%v, %v), want (false, nil)", ok, err)
	}
	if _, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 0, 0, 4, target.PublicKey()); err != nil || ok {
		t.Fatalf("recover on wrong vault = (%v, %v), want (false, nil)", ok, err)
	}
	// The range may end at the largest key index without wrapping around.
	if _, ok, err := RecoverQSBKeyIndex(qsbTestSeed, 3, 0xffffffff, 0xffffffff, target.PublicKey()); err != nil || ok {
		t.Fatalf("recover at max index = (%v, %v), want (false, nil)", ok, err)
	}
}
