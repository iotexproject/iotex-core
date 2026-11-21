// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestPutPollResult(t *testing.T) {
	candidates := state.CandidateList{}
	pk := identityset.PrivateKey(32).PublicKey()
	addr := pk.Address()
	assert.NotNil(t, addr)
	candidates = append(candidates, &state.Candidate{
		Address: addr.String(),
		Votes:   big.NewInt(1000),
	})
	r := NewPutPollResult(10001, candidates)
	igas, err := r.IntrinsicGas()
	assert.NoError(t, err)
	assert.Zero(t, igas)
	elp := (&EnvelopeBuilder{}).SetNonce(1).SetGasLimit(uint64(100000)).
		SetAction(r).Build()
	cost, err := elp.Cost()
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(0), cost)
	pb := r.Proto()
	assert.NotNil(t, pb)
	clone := &PutPollResult{}
	assert.NoError(t, clone.LoadProto(pb))
	assert.Equal(t, r, clone)

}

// TestPutPollResultLoadProtoEmptyCandidates checks that a putPollResult with
// no candidate list (Candidates is nil) decodes to an empty candidate list.
func TestPutPollResultLoadProtoEmptyCandidates(t *testing.T) {
	require := require.New(t)
	for name, pb := range map[string]*iotextypes.PutPollResult{
		"empty":           {},
		"nil candidates":  {Height: 7},
		"empty cand list": {Height: 7, Candidates: &iotextypes.CandidateList{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := &PutPollResult{}
			require.NotPanics(func() {
				require.NoError(r.LoadProto(pb))
			})
			require.Empty(r.Candidates())
		})
	}
}
