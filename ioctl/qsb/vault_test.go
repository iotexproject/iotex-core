// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/stretchr/testify/require"
	"github.com/tyler-smith/go-bip39"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

func init() {
	// Keep unit tests fast; real vaults use the standard parameters.
	scryptN, scryptP = keystore.LightScryptN, keystore.LightScryptP
}

// BIP-39 test vector, docs/qsb-spec-and-vectors.md §9.
const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func TestSeedVectors(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	for _, tc := range []struct {
		passphrase, seed, pub string
	}{
		{"", "408b285c123836004f4b8842c89324c1f01382450c0d439af345ba7fc49acf705489c6fc77dbd4e3dc1dd8cc6bc9f043db8ada1e243c4a0eafb290d399480840",
			"b06841df29502ec67c3160c530f7b7d26e0a76838cced6e77b545a16fb3d435ce5bac69926bc389a83d32a7c6f2876ee"},
		{"TREZOR", "bda85446c68413707090a52022edd26a1c9462295029f2e60cd7c4f2bbd3097170af7a4d73245cafa9c3cca8d561a7c3de6f5d4a10be8ed2a5e608d68f92fcc8",
			"34fbd17dc298c306ca1d49a7cc9624a201bb7e132434b190201d121d4fbd886a422c45c509e0d143edb350a1e1b1f417"},
	} {
		seed, err := Seed(testMnemonic, tc.passphrase)
		r.NoError(err)
		r.Equal(tc.seed, hex.EncodeToString(seed))
		k, err := NewKeyring(seed, 0).Key(0)
		r.NoError(err)
		r.Equal(tc.pub, hex.EncodeToString(k.PublicKey().Bytes()))
	}
}

func TestSeedNFKD(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	want, err := Seed(testMnemonic, "")
	r.NoError(err)
	// Ideographic spaces and repeated whitespace normalize to single spaces.
	got, err := Seed(strings.ReplaceAll(testMnemonic, " ", "\u3000 "), "")
	r.NoError(err)
	r.Equal(want, got)

	// A composed and a decomposed passphrase give the same seed, the one of
	// the NFKD form.
	const composed, decomposed = "caf\u00e9", "cafe\u0301"
	nfc, err := Seed(testMnemonic, composed)
	r.NoError(err)
	nfd, err := Seed(testMnemonic, decomposed)
	r.NoError(err)
	r.Equal(nfc, nfd)
	r.Equal(bip39.NewSeed(testMnemonic, decomposed), nfc)
	r.NotEqual(bip39.NewSeed(testMnemonic, composed), nfc)

	_, err = Seed(strings.Replace(testMnemonic, "art", "abandon", 1), "")
	r.ErrorContains(err, "invalid mnemonic")
}

func TestNewMnemonic(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	for _, n := range []int{12, 15, 18, 21, 24} {
		m, err := NewMnemonic(n)
		r.NoError(err)
		r.Len(strings.Fields(m), n)
		r.True(bip39.IsMnemonicValid(m))
	}
	for _, n := range []int{0, 11, 13, 27} {
		_, err := NewMnemonic(n)
		r.Error(err)
	}
}

func TestVaultRoundTrip(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	data, err := EncryptVault(" "+testMnemonic+" ", "TREZOR", "pw")
	r.NoError(err)
	r.NotContains(string(data), "abandon")
	var f map[string]any
	r.NoError(json.Unmarshal(data, &f))
	r.Equal(VaultType, f["type"])
	r.EqualValues(VaultVersion, f["version"])

	_, err = DecryptVault(data, "wrong")
	r.ErrorIs(err, ErrWrongPassword)

	v, err := DecryptVault(data, "pw")
	r.NoError(err)
	r.Equal(testMnemonic, v.Mnemonic)
	r.True(v.HasPassphrase)

	// A wrong passphrase is caught by the check value.
	_, err = v.Unlock("", 0)
	r.ErrorIs(err, ErrWrongPassphrase)
	_, err = v.Unlock("trezor", 0)
	r.ErrorIs(err, ErrWrongPassphrase)

	kr, err := v.Unlock("TREZOR", 0)
	r.NoError(err)
	k, err := kr.Key(0)
	r.NoError(err)
	r.Equal("34fbd17dc298c306", hex.EncodeToString(k.PublicKey().Bytes())[:16])
	kr.Zero()
	_, err = kr.Key(0)
	r.Error(err)

	// No passphrase: an unexpected one is rejected.
	data, err = EncryptVault(testMnemonic, "", "pw")
	r.NoError(err)
	v, err = DecryptVault(data, "pw")
	r.NoError(err)
	r.False(v.HasPassphrase)
	_, err = v.Unlock("x", 0)
	r.ErrorIs(err, ErrWrongPassphrase)
	_, err = v.Unlock("", 0)
	r.NoError(err)

	_, err = EncryptVault(testMnemonic, "", "")
	r.Error(err)
	_, err = DecryptVault([]byte(`{"type":"other","version":1,"crypto":{}}`), "pw")
	r.ErrorContains(err, "not a guard vault")
}

func TestKeyringFind(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	seed, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f")
	target, err := signer.DeriveQSBKey(seed, 3, 2)
	r.NoError(err)
	r.Equal("4384023dc938bad1", hex.EncodeToString(target.PublicKey().Bytes())[:16])

	kr := NewKeyring(seed, 3)
	i, ok, err := kr.Find(context.Background(), target.PublicKey(), 0, 4)
	r.NoError(err)
	r.True(ok)
	r.EqualValues(2, i)

	_, ok, err = kr.Find(context.Background(), target.PublicKey(), 3, 4)
	r.NoError(err)
	r.False(ok)
	_, ok, err = NewKeyring(seed, 0).Find(context.Background(), target.PublicKey(), 0, 4)
	r.NoError(err)
	r.False(ok)
	_, ok, err = kr.Find(context.Background(), target.PublicKey(), 0xffffffff, 0xffffffff)
	r.NoError(err)
	r.False(ok)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok, err = kr.Find(ctx, target.PublicKey(), 0, 1000)
	r.False(ok)
	r.ErrorIs(err, context.Canceled)
}

// BenchmarkKeyringKey measures one QSB-KDF-v1 key derivation (HKDF plus
// SLH-DSA-SHA2-192s key generation).
func BenchmarkKeyringKey(b *testing.B) {
	kr := NewKeyring(make([]byte, 64), 0)
	for i := 0; i < b.N; i++ {
		if _, err := kr.Key(uint32(i)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyringFind64 measures scanning the default op scan window.
func BenchmarkKeyringFind64(b *testing.B) {
	kr := NewKeyring(make([]byte, 64), 0)
	other, err := NewKeyring(make([]byte, 64), 1).Key(0)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		if _, _, err := kr.Find(context.Background(), other.PublicKey(), 0, 63); err != nil {
			b.Fatal(err)
		}
	}
}
