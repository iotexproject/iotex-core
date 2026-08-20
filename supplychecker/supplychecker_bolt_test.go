// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package supplychecker

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestBoltBackedSupplyConservation exercises the L2 (per-block tracker) and L3
// (periodic observer) supply monitors against a REAL bolt-backed
// factory.Factory, closing the review gap that the in-memory fakeStateReader
// only ever inserts state.Account into the Account namespace (so it can never
// surface how the real serialized on-chain states / state-diff plumbing behave).
//
// With a real factory that supports Filter() (bolt), we verify that:
//   - the L2 tracker, wired through factory.StateDiffCallback, observes a block
//     of plain transfers and records a conserved (non-increasing) total;
//   - the L3 observer performs a full-namespace scan (Account + Rewarding) over
//     the real serialized states without panicking, and reads back the genesis
//     total supply.
func TestBoltBackedSupplyConservation(t *testing.T) {
	require := require.New(t)

	dbPath, err := testutil.PathOfTempFile("supplycheck")
	require.NoError(err)
	defer testutil.CleanupPath(dbPath)

	// --- Config + funded genesis ------------------------------------------------
	cfg := config.Default
	g := genesis.TestDefault()
	cfg.Chain.TrieDBPath = dbPath
	// Use v2 storage from genesis so the rewarding fund lives in the Rewarding
	// namespace (mirrors modern mainnet, which is far past Greenland).
	g.GreenlandBlockHeight = 0
	// genesis.TestDefault() funds indices < identityset.Size(); fund the producer
	// (27) and the two transfer accounts (28, 29) explicitly.
	g.InitBalanceMap[identityset.Address(27).String()] = "1000000000000000000000000000000"
	g.InitBalanceMap[identityset.Address(28).String()] = "100000000000000000000000000"
	g.InitBalanceMap[identityset.Address(29).String()] = "100000000000000000000000000"
	cfg.Genesis = g

	// --- Register protocols (account + rolldpos + rewarding) -------------------
	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	require.NoError(rolldpos.NewProtocol(
		g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs,
	).Register(registry))
	require.NoError(rewarding.NewProtocol(g.Rewarding).Register(registry))

	// --- Create the real-state factory -----------------------------------------
	kv, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sf, err := factory.NewStateDB(factory.GenerateConfig(cfg.Chain, cfg.Genesis), kv,
		factory.RegistryStateDBOption(registry), factory.SkipBlockValidationStateDBOption())
	require.NoError(err)

	// --- Wire the L2 per-block tracker into the real factory --------------------
	tracker := NewSupplyTrackerFromGenesis(cfg.Genesis)
	require.True(factory.AddDiffCallback(sf, tracker.OnBlockCommitted))

	// --- Start (mints genesis states) ------------------------------------------
	startCtx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	startCtx = protocol.WithFeatureWithHeightCtx(startCtx)
	startCtx = protocol.WithBlockchainCtx(startCtx, protocol.BlockchainCtx{ChainID: cfg.Chain.ID})
	require.NoError(sf.Start(startCtx))
	defer func() { require.NoError(sf.Stop(startCtx)) }()

	// At genesis (before any block) a full scan must already report Total == Cap.
	o := NewObserver(sf, cfg.Genesis, 0)
	res, err := o.Check(context.Background())
	require.NoError(err)
	require.Zero(res.Total.Cmp(res.Cap), "genesis total must equal cap on a real factory")
	require.Equal(0, res.Total.Cmp(res.Cap))

	// --- Commit one block with a plain NATIVE transfer ---------------------------
	priKeyA := identityset.PrivateKey(28)
	tsf := action.NewTransfer(big.NewInt(10), identityset.Address(29).String(), nil)
	bd := &action.EnvelopeBuilder{}
	// Mirror state/factory testCommit: no explicit gas fields, so transfer only
	// moves value between two funded accounts and conserves total supply.
	elp := bd.SetNonce(1).SetAction(tsf).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	blkCtx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight: 1,
		Producer:    identityset.Address(27),
		GasLimit:    1000000,
	})
	putCtx := genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(blkCtx, protocol.BlockchainCtx{
			Tip:     protocol.TipInfo{Height: 0, Hash: hash.ZeroHash256},
			ChainID: cfg.Chain.ID,
		}),
		cfg.Genesis,
	)
	putCtx = protocol.WithFeatureCtx(putCtx)
	putCtx = protocol.WithFeatureWithHeightCtx(putCtx)

	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	require.NoError(sf.PutBlock(putCtx, &blk))

	// --- Assert the L2 tracker observed a conserved block ------------------------
	require.Equal(uint64(1), tracker.Height(), "tracker must observe the committed block")
	require.Zero(tracker.RunningTotal().Cmp(res.Cap), "transfer must conserve total supply")

	// --- Assert the L3 observer still reconciles at height 1 ---------------------
	res1, err := o.Check(context.Background())
	require.NoError(err)
	require.Zero(res1.Total.Cmp(res.Cap), "transfer must not change total supply on a real factory")
	require.Zero(res1.Account.Cmp(res.Account), "sum of account balances unchanged by a transfer")
}

// TestBoltNamespaceScansSafe confirms the strict decoder tolerates the real
// serialized entries the Account namespace holds on a bolt factory (genuine
// accounts plus the per-block height key), never panicking.
func TestBoltNamespaceScansSafe(t *testing.T) {
	require := require.New(t)

	dbPath, err := testutil.PathOfTempFile("supplycheck-scan")
	require.NoError(err)
	defer testutil.CleanupPath(dbPath)

	cfg := config.Default
	g := genesis.TestDefault()
	cfg.Chain.TrieDBPath = dbPath
	g.GreenlandBlockHeight = 0
	g.InitBalanceMap[identityset.Address(28).String()] = "1000"
	cfg.Genesis = g

	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	require.NoError(rolldpos.NewProtocol(
		g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs,
	).Register(registry))
	require.NoError(rewarding.NewProtocol(g.Rewarding).Register(registry))

	kv, err := db.CreateKVStoreWithCache(db.DefaultConfig, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	sf, err := factory.NewStateDB(factory.GenerateConfig(cfg.Chain, cfg.Genesis), kv,
		factory.RegistryStateDBOption(registry), factory.SkipBlockValidationStateDBOption())
	require.NoError(err)

	startCtx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	startCtx = protocol.WithFeatureWithHeightCtx(startCtx)
	startCtx = protocol.WithBlockchainCtx(startCtx, protocol.BlockchainCtx{ChainID: cfg.Chain.ID})
	require.NoError(sf.Start(startCtx))
	defer func() { require.NoError(sf.Stop(startCtx)) }()

	// A full-namespace scan over the real Account namespace must not panic and
	// must not over-count: it must read every genuine account balance exactly.
	// The rewarding fund total must also be readable from its own namespace.
	fund, err := readFundTotal(sf)
	require.NoError(err)
	require.True(fund.Sign() > 0, "rewarding fund must be seeded at genesis")

	// At genesis R1 (account reservoir) == genesis cap minus the rewarding seed
	// (R2), with no staking pool yet (R3 == 0). This is an exact reconciliation on
	// real serialized states, not a loose bound.
	cap := genesisTotalSupply(cfg.Genesis)
	sum, _, err := sumPrimaryBalances(sf)
	require.NoError(err)
	require.Zero(sum.Cmp(new(big.Int).Sub(cap, fund)),
		"R1 must equal Cap - R2 at genesis on a real bolt factory")
}
