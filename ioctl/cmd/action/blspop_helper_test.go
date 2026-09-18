// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBlsPoPFlags_ClassifyForRegister exercises the three-option matrix
// at the register command, plus the rejection cases for partial input.
// The classifier is the only stateful piece between flag parsing and
// the cryptographic resolve path, so covering it explicitly catches
// regressions without having to spin up cobra / keystore plumbing.
func TestBlsPoPFlags_ClassifyForRegister(t *testing.T) {
	cases := []struct {
		name   string
		f      blsPoPFlags
		want   blsMode
		errMsg string
	}{
		{"all empty → auto-derive", blsPoPFlags{}, blsModeAutoDerive, ""},
		{"only --bls-priv-key → explicit key",
			blsPoPFlags{privKeyHex: "deadbeef"}, blsModeExplicitKey, ""},
		{"--bls-pubkey + --bls-pop → explicit PoP",
			blsPoPFlags{pubKeyHex: "abcd", popHex: "ef01"}, blsModeExplicitPoP, ""},
		{"only --bls-pubkey → error (incomplete)",
			blsPoPFlags{pubKeyHex: "abcd"}, 0, "must be specified together"},
		{"only --bls-pop → error (incomplete)",
			blsPoPFlags{popHex: "ef01"}, 0, "must be specified together"},
		{"--bls-priv-key + --bls-pubkey + --bls-pop → mutually exclusive",
			blsPoPFlags{privKeyHex: "deadbeef", pubKeyHex: "abcd", popHex: "ef01"},
			0, "mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			mode, err := tc.f.classifyForRegister()
			if tc.errMsg != "" {
				r.Error(err)
				r.Contains(err.Error(), tc.errMsg)
				return
			}
			r.NoError(err)
			r.Equal(tc.want, mode)
		})
	}
}

// TestBlsPoPFlags_ClassifyForUpdate covers the update-only Option 0
// (no BLS flags → don't touch BLS) plus the rejection branches that
// keep "rotate BLS" explicit so a name/operator-only update never
// silently rotates the producer key.
func TestBlsPoPFlags_ClassifyForUpdate(t *testing.T) {
	cases := []struct {
		name   string
		f      blsPoPFlags
		want   blsMode
		errMsg string
	}{
		{"all empty → Option 0 (no BLS)", blsPoPFlags{}, blsModeNone, ""},
		{"--bls-from-signer → auto-derive",
			blsPoPFlags{fromSigner: true}, blsModeAutoDerive, ""},
		{"--bls-priv-key → explicit key",
			blsPoPFlags{privKeyHex: "deadbeef"}, blsModeExplicitKey, ""},
		{"--bls-pubkey + --bls-pop → explicit PoP",
			blsPoPFlags{pubKeyHex: "abcd", popHex: "ef01"}, blsModeExplicitPoP, ""},
		{"only --bls-pubkey → error",
			blsPoPFlags{pubKeyHex: "abcd"}, 0, "must be specified together"},
		{"only --bls-pop → error",
			blsPoPFlags{popHex: "ef01"}, 0, "must be specified together"},
		{"--bls-priv-key + --bls-from-signer → mutually exclusive",
			blsPoPFlags{privKeyHex: "deadbeef", fromSigner: true},
			0, "mutually exclusive"},
		{"--bls-pubkey + --bls-pop + --bls-from-signer → mutually exclusive",
			blsPoPFlags{pubKeyHex: "abcd", popHex: "ef01", fromSigner: true},
			0, "mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			mode, err := tc.f.classifyForUpdate()
			if tc.errMsg != "" {
				r.Error(err)
				r.Contains(err.Error(), tc.errMsg)
				return
			}
			r.NoError(err)
			r.Equal(tc.want, mode)
		})
	}
}

// TestLoadBLSPrivKey covers the hex parse + length-validate path,
// including the 0x prefix accepting.
func TestLoadBLSPrivKey(t *testing.T) {
	r := require.New(t)

	// 32-byte hex (valid scalar — derived deterministically from sha256
	// in tests would be better, but for codec coverage any non-zero
	// 32B value the BLS key parser accepts will do).
	good := "11111111111111111111111111111111" + "11111111111111111111111111111111"
	sk, err := loadBLSPrivKey(good)
	r.NoError(err)
	r.NotNil(sk)
	sk.Zero()

	// 0x prefix accepted.
	sk, err = loadBLSPrivKey("0x" + good)
	r.NoError(err)
	r.NotNil(sk)
	sk.Zero()

	// Empty rejected.
	_, err = loadBLSPrivKey("")
	r.Error(err)

	// Non-hex rejected.
	_, err = loadBLSPrivKey("zzzzzz")
	r.Error(err)

	// Wrong length rejected.
	_, err = loadBLSPrivKey("dead")
	r.Error(err)
}
