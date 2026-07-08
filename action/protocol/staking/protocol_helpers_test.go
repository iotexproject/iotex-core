// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func newProtocolForTest(t *testing.T) *Protocol {
	g := genesis.TestDefault()
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil, nil)
	require.NoError(t, err)
	return p
}

func TestFindProtocol(t *testing.T) {
	r := require.New(t)
	// nil registry
	r.Nil(FindProtocol(nil))
	// empty registry without staking registered
	r.Nil(FindProtocol(protocol.NewRegistry()))
	// registered protocol is returned by identity
	p := newProtocolForTest(t)
	reg := protocol.NewRegistry()
	r.NoError(p.Register(reg))
	got := FindProtocol(reg)
	r.NotNil(got)
	r.Same(p, got)
}

func TestProtocolAddr(t *testing.T) {
	r := require.New(t)
	p := newProtocolForTest(t)
	// ProtocolAddr must be deterministic and match the address NewProtocol derives
	r.True(address.Equal(ProtocolAddr(), ProtocolAddr()))
	r.Equal(p.addr.String(), ProtocolAddr().String())
}
