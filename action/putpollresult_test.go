// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutPollResult(t *testing.T) {
	candidates := state.CandidateList{}
	pk := testaddress.Keyinfo["echo"].PubKey
	addr, err := address.FromBytes(pk.Hash())
	assert.NoError(t, err)
	candidates = append(candidates, &state.Candidate{
		Address: addr.String(),
		Votes:   big.NewInt(1000),
	})
	r := NewPutPollResult(1, 10001, candidates)
	assert.Equal(t, uint64(1), r.Nonce())
	igas, err := r.IntrinsicGas()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), igas)
	cost, err := r.Cost()
	assert.NoError(t, err)
	assert.Equal(t, 0, big.NewInt(0).Cmp(cost))
	pb := r.Proto()
	assert.NotNil(t, pb)
	clone := &PutPollResult{}
	assert.NoError(t, clone.LoadProto(pb))
	assert.Equal(t, uint64(10001), clone.Height())

}
