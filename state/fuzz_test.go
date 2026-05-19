// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"testing"
)

// FuzzAccountDeserialize asserts that Account.Deserialize never panics on
// arbitrary input bytes. The seed corpus covers legacy and zero-nonce
// account types, varying balance widths, and contract accounts with code
// hashes.
//
// Account.FromProto contains two panic sites (unknown account type and
// unparseable balance string) that are exposed when fuzz mutations produce
// proto-valid-but-semantically-bad messages. Findings should be filed as
// follow-up fixes; the harness here is what catches them.
func FuzzAccountDeserialize(f *testing.F) {
	seeds := []*Account{
		{nonce: 0, Balance: big.NewInt(0), votingWeight: big.NewInt(0)},
		{nonce: 100, Balance: big.NewInt(1_000_000), votingWeight: big.NewInt(0)},
		{nonce: 1, Balance: new(big.Int).Lsh(big.NewInt(1), 200), votingWeight: big.NewInt(0)},
		{nonce: 5, Balance: big.NewInt(0), votingWeight: big.NewInt(42), isCandidate: true},
		{nonce: 0, Balance: big.NewInt(7), CodeHash: []byte("0123456789abcdef0123456789abcdef"), votingWeight: big.NewInt(0)},
	}
	for _, a := range seeds {
		if b, err := a.Serialize(); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Account.Deserialize panicked on %d bytes: %v", len(data), r)
			}
		}()
		var a Account
		_ = a.Deserialize(data)
	})
}

// FuzzCandidateListDeserialize asserts that CandidateList.Deserialize never
// panics on arbitrary input bytes.
func FuzzCandidateListDeserialize(f *testing.F) {
	seeds := []CandidateList{
		{},
		{{Address: "io1aaa", Votes: big.NewInt(1), RewardAddress: "io1bbb"}},
		{
			{Identity: "alice", Address: "io1aaa", Votes: big.NewInt(10), RewardAddress: "io1bbb"},
			{Identity: "bob", Address: "io1ccc", Votes: big.NewInt(20), RewardAddress: "io1ddd"},
		},
		{{Address: "io1xxx", Votes: big.NewInt(0), BLSPubKey: make([]byte, 48)}},
	}
	for _, l := range seeds {
		if b, err := l.Serialize(); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("CandidateList.Deserialize panicked on %d bytes: %v", len(data), r)
			}
		}()
		var l CandidateList
		_ = l.Deserialize(data)
	})
}
