// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package signer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// acvpVectors are the NIST ACVP SLH-DSA-SHA2-192s vectors in
// ../testdata (see its README). Byte fields are hex.
type acvpVectors struct {
	ParameterSet string `json:"parameterSet"`
	KeyGen       []struct {
		TcID   int    `json:"tcId"`
		SkSeed string `json:"skSeed"`
		SkPrf  string `json:"skPrf"`
		PkSeed string `json:"pkSeed"`
		Pk     string `json:"pk"`
	} `json:"keyGen"`
	SigGen []struct {
		TcID      int    `json:"tcId"`
		Sk        string `json:"sk"`
		Pk        string `json:"pk"`
		Message   string `json:"message"`
		Context   string `json:"context"`
		Signature string `json:"signature"`
	} `json:"sigGen"`
}

func loadACVP(t *testing.T) *acvpVectors {
	t.Helper()
	f, err := os.Open("../testdata/acvp-slhdsa-sha2-192s.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var v acvpVectors
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v.ParameterSet != "SLH-DSA-SHA2-192s" || len(v.KeyGen) == 0 || len(v.SigGen) == 0 {
		t.Fatalf("unexpected vectors: %s, %d keyGen, %d sigGen", v.ParameterSet, len(v.KeyGen), len(v.SigGen))
	}
	return &v
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestACVPKeyGen checks NewKeyFromSeeds, which QSB-KDF-v1 feeds (IIP-65
// §12.1), against the NIST ACVP keyGen vectors.
func TestACVPKeyGen(t *testing.T) {
	for _, tc := range loadACVP(t).KeyGen {
		seeds := append(append(unhex(t, tc.SkSeed), unhex(t, tc.SkPrf)...), unhex(t, tc.PkSeed)...)
		k, err := NewKeyFromSeeds(pqcrypto.AlgSLHDSA192s, seeds)
		if err != nil {
			t.Fatalf("tc %d: %v", tc.TcID, err)
		}
		if got := k.PublicKey().Bytes(); !bytes.Equal(got, unhex(t, tc.Pk)) {
			t.Errorf("tc %d: public key %x, want %s", tc.TcID, got, tc.Pk)
		}
	}
}

// TestACVPSigGen checks pure-mode signing with a context string, as the
// guard transcripts are signed (IIP-65 §7), against the NIST ACVP sigGen
// vectors for deterministic signing. The private key is rebuilt from the
// first three n-byte parts of the vector's sk, SK.seed || SK.prf || PK.seed.
func TestACVPSigGen(t *testing.T) {
	alg := pqcrypto.AlgSLHDSA192s
	for _, tc := range loadACVP(t).SigGen {
		sk := unhex(t, tc.Sk)
		k, err := NewKeyFromSeeds(alg, sk[:SeedSize(alg)])
		if err != nil {
			t.Fatalf("tc %d: %v", tc.TcID, err)
		}
		if !bytes.Equal(k.PublicKey().Bytes(), unhex(t, tc.Pk)) {
			t.Fatalf("tc %d: the key rebuilt from sk does not match pk", tc.TcID)
		}
		ctx, msg := unhex(t, tc.Context), unhex(t, tc.Message)
		sig, err := k.SignDeterministic(ctx, msg)
		if err != nil {
			t.Fatalf("tc %d: %v", tc.TcID, err)
		}
		if !bytes.Equal(sig, unhex(t, tc.Signature)) {
			t.Errorf("tc %d: signature differs from the vector", tc.TcID)
		}
		if !k.PublicKey().Verify(ctx, msg, sig) {
			t.Errorf("tc %d: signature does not verify", tc.TcID)
		}
	}
}
