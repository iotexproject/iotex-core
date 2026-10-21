// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package pqcrypto_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

func TestAlgID(t *testing.T) {
	a := pqcrypto.AlgSLHDSA192s
	if !a.Supported() || a.PublicKeySize() != 48 || a.SignatureSize() != 16224 || a.String() != "SLH-DSA-SHA2-192s" {
		t.Fatalf("unexpected spec for %v", a)
	}
	unknown := pqcrypto.AlgID(0x13)
	if unknown.Supported() || unknown.PublicKeySize() != 0 || unknown.SignatureSize() != 0 || unknown.String() != "alg(0x13)" {
		t.Fatalf("unexpected spec for unknown alg %v", unknown)
	}
}

func TestParsePublicKey(t *testing.T) {
	if _, err := pqcrypto.ParsePublicKey(0x13, make([]byte, 48)); !errors.Is(err, pqcrypto.ErrUnsupportedAlg) {
		t.Fatalf("want ErrUnsupportedAlg, got %v", err)
	}
	if _, err := pqcrypto.ParsePublicKey(pqcrypto.AlgSLHDSA192s, make([]byte, 47)); !errors.Is(err, pqcrypto.ErrInvalidPublicKey) {
		t.Fatalf("want ErrInvalidPublicKey, got %v", err)
	}
	raw := bytes.Repeat([]byte{7}, 48)
	pk, err := pqcrypto.ParsePublicKey(pqcrypto.AlgSLHDSA192s, raw)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 0 // the key must not alias the caller's buffer
	if pk.Bytes()[0] != 7 {
		t.Fatal("public key aliases input")
	}
	b := pk.Bytes()
	b[1] = 0
	if pk.Bytes()[1] != 7 {
		t.Fatal("Bytes returns internal buffer")
	}
}

func TestVerify(t *testing.T) {
	k, err := signer.NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, bytes.Repeat([]byte{1}, signer.SeedSize(pqcrypto.AlgSLHDSA192s)))
	if err != nil {
		t.Fatal(err)
	}
	pk := k.PublicKey()
	ctx, msg := []byte("TEST-CONTEXT"), []byte("message")
	sig, err := k.Sign(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	if !pk.Verify(ctx, msg, sig) {
		t.Fatal("valid signature rejected")
	}
	for name, tc := range map[string]struct{ ctx, msg, sig []byte }{
		"context":  {[]byte("OTHER-CONTEXT"), msg, sig},
		"message":  {ctx, []byte("massage"), sig},
		"sigBit":   {ctx, msg, flip(sig)},
		"sigShort": {ctx, msg, sig[:len(sig)-1]},
		"ctxLong":  {bytes.Repeat([]byte{'a'}, 256), msg, sig},
	} {
		if pk.Verify(tc.ctx, tc.msg, tc.sig) {
			t.Fatalf("%s: tampered input accepted", name)
		}
	}
}

func flip(b []byte) []byte {
	c := bytes.Clone(b)
	c[0] ^= 1
	return c
}

func TestTranscript(t *testing.T) {
	got := pqcrypto.NewTranscript("D").
		Uint8(0xaa).Bool(true).Bool(false).
		Uint32(0x01020304).Uint64(0x0102030405060708).
		Fixed([]byte{0xee, 0xff}).Bytes([]byte{0x11}).Bytes(nil).
		Message()
	want := "44" + "aa" + "01" + "00" + "01020304" + "0102030405060708" + "eeff" + "0000000111" + "00000000"
	if hex.EncodeToString(got) != want {
		t.Fatalf("transcript = %x, want %s", got, want)
	}
}
