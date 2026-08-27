// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestTestnetGrant runs a scheduled balance grant on a live chain across two
// nodes: one mints the blocks, the other independently validates and commits
// them. The second node is the point of the test -- it recomputes the delta
// state digest from its own working set, so if the grant did not reproduce
// identically there, ValidateBlock would fail with ErrDeltaStateMismatch
// instead of the balances lining up.
func TestTestnetGrant(t *testing.T) {
	var (
		sender   = identityset.Address(10)
		senderSK = identityset.PrivateKey(10)
		// an address with no genesis balance, and no role in the genesis
		// delegate set -- what a replacement delegate owner key looks like
		fresh = mustNoErr(address.FromHex("0x00000000000000000000000000000000000c0ffe"))
		// an address that already holds a genesis balance, to prove the grant
		// adds rather than overwrites
		funded      = identityset.Address(1)
		fundedInit  = unit.ConvertIotxToRau(100000000)
		grantHeight = uint64(3)
		freshGrant  = unit.ConvertIotxToRau(1200100) // min self-stake plus the registration fee
		fundedGrant = unit.ConvertIotxToRau(5)
		gasLimit    = uint64(1000000)
	)

	newCfg := func(r *require.Assertions, withGrant bool) config.Config {
		cfg := initCfg(r)
		// A grant is refused on mainnet's chain ID and EVM network ID, so a
		// test that schedules one has to declare a testnet identity.
		cfg.Chain.ID = 2
		cfg.Chain.EVMNetworkID = 4690
		cfg.Network.Port = testutil.RandomPort()
		cfg.API.GRPCPort = testutil.RandomPort()
		cfg.API.HTTPPort = testutil.RandomPort()
		cfg.API.WebSocketPort = 0
		cfg.Genesis.InitBalanceMap[sender.String()] = unit.ConvertIotxToRau(10000).String()
		cfg.Genesis.InitBalanceMap[funded.String()] = fundedInit.String()
		if withGrant {
			cfg.Genesis.Account.TestnetGrants = []genesis.TestnetGrant{{
				Height: grantHeight,
				Recipients: []genesis.GrantRecipient{
					{Address: fresh.String(), Amount: freshGrant.String()},
					{Address: funded.String(), Amount: fundedGrant.String()},
				},
			}}
		}
		return cfg
	}

	// a plain self-transfer per block, purely to have something to mint
	newTransfer := func(chainID uint32, nonce uint64) *actionWithTime {
		tx := action.NewLegacyTx(chainID, nonce, gasLimit, big.NewInt(unit.Qev))
		elp := action.NewEnvelope(tx, action.NewTransfer(big.NewInt(1), sender.String(), nil))
		return &actionWithTime{mustNoErr(action.Sign(elp, senderSK)), time.Now()}
	}

	t.Run("both nodes carry the grant", func(t *testing.T) {
		r := require.New(t)
		cfgA, cfgB := newCfg(r, true), newCfg(r, true)
		// the two nodes differ only in their local paths and ports -- the
		// genesis, including the grant, has to be identical to agree
		r.Equal(cfgA.Genesis.Hash(), cfgB.Genesis.Hash())

		producer := newE2ETest(t, cfgA)
		defer producer.teardown()
		validator := newE2ETest(t, cfgB)
		defer validator.teardown()

		var (
			ctx              = context.Background()
			bcA              = producer.cs.Blockchain()
			apA              = producer.cs.ActionPool()
			bcB              = validator.cs.Blockchain()
			balanceA         = balanceReader(r, producer.cs.StateFactory(), cfgA.Genesis)
			balanceB         = balanceReader(r, validator.cs.StateFactory(), cfgB.Genesis)
			fundedAfterGrant = new(big.Int).Add(fundedInit, fundedGrant).String()
		)

		for h := uint64(1); h <= 5; h++ {
			tx := newTransfer(cfgA.Chain.ID, producer.nonceMgr.pop(sender.String()))
			_, _, blk, err := addOneTx(ctx, apA, bcA, tx)
			r.NoErrorf(err, "failed to mint block %d", h)
			r.Equal(h, blk.Height())

			// the second node reproduces the block from scratch; ValidateBlock
			// is what compares its own digest against the one in the header
			r.NoErrorf(bcB.ValidateBlock(blk), "node B failed to validate block %d", h)
			r.NoErrorf(bcB.CommitBlock(blk), "node B failed to commit block %d", h)

			wantFresh, wantFunded := "0", fundedInit.String()
			if h >= grantHeight {
				wantFresh, wantFunded = freshGrant.String(), fundedAfterGrant
			}
			for _, b := range []func(address.Address) string{balanceA, balanceB} {
				r.Equalf(wantFresh, b(fresh), "unexpected fresh balance at height %d", h)
				r.Equalf(wantFunded, b(funded), "unexpected funded balance at height %d", h)
			}
		}
	})

	// The operational failure mode, pinned so it stays the loud one. A node
	// that did not get the updated genesis file stays on the same p2p network
	// -- the grant is not part of the genesis hash -- and follows the chain
	// normally right up to the activation height, where its own digest no
	// longer matches the block header and it stops. It does not silently fork.
	t.Run("node missing the grant rejects the block", func(t *testing.T) {
		r := require.New(t)
		cfgA, cfgB := newCfg(r, true), newCfg(r, false)
		r.Equal(cfgA.Genesis.Hash(), cfgB.Genesis.Hash())

		producer := newE2ETest(t, cfgA)
		defer producer.teardown()
		stale := newE2ETest(t, cfgB)
		defer stale.teardown()

		var (
			ctx = context.Background()
			bcA = producer.cs.Blockchain()
			apA = producer.cs.ActionPool()
			bcB = stale.cs.Blockchain()
		)

		for h := uint64(1); h <= grantHeight; h++ {
			tx := newTransfer(cfgA.Chain.ID, producer.nonceMgr.pop(sender.String()))
			_, _, blk, err := addOneTx(ctx, apA, bcA, tx)
			r.NoErrorf(err, "failed to mint block %d", h)

			err = bcB.ValidateBlock(blk)
			if h < grantHeight {
				r.NoErrorf(err, "stale node should still follow block %d", h)
				r.NoError(bcB.CommitBlock(blk))
				continue
			}
			r.ErrorIs(err, block.ErrDeltaStateMismatch)
		}
	})
}

func balanceReader(r *require.Assertions, sf factory.Factory, g genesis.Genesis) func(address.Address) string {
	ctx := genesis.WithGenesisContext(context.Background(), g)
	return func(addr address.Address) string {
		acct, err := accountutil.AccountState(ctx, sf, addr)
		r.NoError(err)
		return acct.Balance.String()
	}
}
