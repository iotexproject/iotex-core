// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestFinalizeCapturesStateDiffOnlyWithCallback(t *testing.T) {
	r := require.New(t)
	cfg := Config{Chain: blockchain.DefaultConfig, Genesis: genesis.TestDefault()}
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(protocol.NewRegistry()))
	r.NoError(err)
	ctx := protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), cfg.Genesis), protocol.BlockCtx{BlockHeight: 1})
	ctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{}))
	r.NoError(f.Start(ctx))
	defer func() { r.NoError(f.Stop(ctx)) }()

	finalized := func() *workingSet {
		ws, err := f.(workingSetCreator).newWorkingSet(ctx, 1)
		r.NoError(err)
		acct, err := accountutil.LoadOrCreateAccount(ws, identityset.Address(1))
		r.NoError(err)
		r.NoError(accountutil.StoreAccount(ws, identityset.Address(1), acct))
		r.NoError(ws.finalize(ctx))
		return ws
	}

	r.Empty(finalized().stateDiffEntries, "nothing receives the diff, so nothing is copied")

	r.True(SetDiffCallback(f, func(uint64, []WriteQueueEntry, []byte) {}))
	ws := finalized()
	r.NotEmpty(ws.stateDiffEntries)
	r.NotEmpty(ws.stateDiffDigest)

	r.True(SetDiffCallback(f, nil))
	r.Empty(finalized().stateDiffEntries)
}
