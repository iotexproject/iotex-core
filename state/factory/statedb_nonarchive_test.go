// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
)

// TestWorkingSetAtHeightNonArchive verifies that on a non-archive node a
// historical-height state read is rejected with ErrNotSupported instead of
// silently returning latest-tip state (issue #4916), while a read at the tip
// still works. Root cause: daoRetrofitter.atHeight ignores the height and
// always returns the latest KV store, so without this guard eth_getBalance at
// an old block returns the current balance.
func TestWorkingSetAtHeightNonArchive(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	sdb, err := NewStateDB(cfg, db.NewMemKVStore(), SkipBlockValidationStateDBOption())
	r.NoError(err)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	r.NoError(sdb.Start(ctx))
	defer func() { r.NoError(sdb.Stop(ctx)) }()

	s := sdb.(*stateDB)
	r.Nil(s.erigonDB, "test assumes a non-archive (no erigonDB) statedb")

	// simulate a chain that has advanced to height 100
	s.mutex.Lock()
	s.currentChainHeight = 100
	s.mutex.Unlock()

	// latest (height == tip) is served normally
	wsTip, err := sdb.WorkingSetAtHeight(ctx, 100)
	r.NoError(err)
	r.NotNil(wsTip)
	wsTip.Close()

	// any past height — including an explicit genesis (0) — is rejected, not
	// silently answered from latest state
	for _, h := range []uint64{0, 1, 50, 99} {
		_, err = sdb.WorkingSetAtHeight(ctx, h)
		r.Error(err)
		r.True(errors.Is(err, ErrNotSupported), "height %d should be ErrNotSupported, got %v", h, err)
		r.Contains(err.Error(), "non-archive node")
	}

	// when the tip is still genesis (0), height 0 is the tip and is served
	s.mutex.Lock()
	s.currentChainHeight = 0
	s.mutex.Unlock()
	wsGenesis, err := sdb.WorkingSetAtHeight(ctx, 0)
	r.NoError(err)
	r.NotNil(wsGenesis)
	wsGenesis.Close()
}
