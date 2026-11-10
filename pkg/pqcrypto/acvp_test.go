// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package pqcrypto

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// acvpVectors are the NIST ACVP SLH-DSA-SHA2-192s vectors in testdata (see
// testdata/README.md). Byte fields are hex.
type acvpVectors struct {
	ParameterSet string `json:"parameterSet"`
	SigVer       []struct {
		TcID       int    `json:"tcId"`
		Pk         string `json:"pk"`
		Message    string `json:"message"`
		Context    string `json:"context"`
		Signature  string `json:"signature"`
		TestPassed bool   `json:"testPassed"`
		Reason     string `json:"reason"`
	} `json:"sigVer"`
}

func loadACVP(t *testing.T, path string, v any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestACVPSigVer checks verification, the only SLH-DSA operation consensus
// runs (IIP-65 §2), against the NIST ACVP sigVer vectors for the external,
// pure interface: valid signatures verify, and modified messages, modified
// signature parts (R, SIG_FORS, SIG_HT) and wrong-length signatures do not.
func TestACVPSigVer(t *testing.T) {
	var v acvpVectors
	loadACVP(t, "testdata/acvp-slhdsa-sha2-192s.json", &v)
	if v.ParameterSet != "SLH-DSA-SHA2-192s" || len(v.SigVer) == 0 {
		t.Fatalf("unexpected vectors: %s, %d sigVer cases", v.ParameterSet, len(v.SigVer))
	}
	var pass, fail int
	for _, tc := range v.SigVer {
		pk, err := ParsePublicKey(AlgSLHDSA192s, unhex(t, tc.Pk))
		if err != nil {
			t.Fatalf("tc %d: %v", tc.TcID, err)
		}
		got := pk.Verify(unhex(t, tc.Context), unhex(t, tc.Message), unhex(t, tc.Signature))
		if got != tc.TestPassed {
			t.Errorf("tc %d (%s): verify = %v, want %v", tc.TcID, tc.Reason, got, tc.TestPassed)
		}
		if tc.TestPassed {
			pass++
		} else {
			fail++
		}
	}
	if pass == 0 || fail == 0 {
		t.Fatalf("want valid and invalid cases, got %d and %d", pass, fail)
	}
}
