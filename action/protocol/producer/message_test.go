// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disset epoch rewarded. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDonateToProducerFund(t *testing.T) {
	b := DonateToProducerFundBuilder{}
	s1 := b.SetAmount(big.NewInt(1)).
		SetData([]byte{2}).
		Build()
	proto := s1.Proto()
	s2 := DonateToProducerFund{}
	s2.LoadProto(proto)
	assert.Equal(t, s1.Amount(), s2.Amount())
	assert.Equal(t, s2.Data(), s2.Data())
}

func TestClaimFromProducerFund(t *testing.T) {
	b := ClaimFromProducerFundBuilder{}
	s1 := b.SetAmount(big.NewInt(1)).
		SetData([]byte{2}).
		Build()
	proto := s1.Proto()
	s2 := ClaimFromProducerFund{}
	s2.LoadProto(proto)
	assert.Equal(t, s1.Amount(), s2.Amount())
	assert.Equal(t, s2.Data(), s2.Data())
}

func TestSetBlockReward(t *testing.T) {
	b := SetBlockRewardBuilder{}
	s1 := b.SetAmount(big.NewInt(1)).
		SetData([]byte{2}).
		Build()
	proto := s1.Proto()
	s2 := SetBlockReward{}
	s2.LoadProto(proto)
	assert.Equal(t, s1.Amount(), s2.Amount())
	assert.Equal(t, s2.Data(), s2.Data())
}

func TestSetEpochReward(t *testing.T) {
	b := SetEpochRewardBuilder{}
	s1 := b.SetAmount(big.NewInt(1)).
		SetData([]byte{2}).
		Build()
	proto := s1.Proto()
	s2 := SetEpochReward{}
	s2.LoadProto(proto)
	assert.Equal(t, s1.Amount(), s2.Amount())
	assert.Equal(t, s2.Data(), s2.Data())
}
