// Copyright (c) 2026 IoTeX Foundation
// This source code is provided as is and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestHeaderGasFieldsThroughBlockchainValidate drives the header gas check
// through blockchain.ValidateBlock, the entry point a node actually uses for a
// block it received, rather than through the state factory directly. It is the
// end of the chain the mismatch has to travel: workingSet.ValidateBlock ->
// stateDB.Validate -> block.Validator -> blockchain.ValidateBlock.
func TestHeaderGasFieldsThroughBlockchainValidate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		gateHeight uint64
		wantErr    error
	}{
		{"pre-fork mismatch accepted", math.MaxUint64, nil},
		{"post-fork mismatch rejected", 1, block.ErrGasUsedMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			bc, ap, producerKey, cfg := newHeaderGasChain(t, tc.gateHeight)

			sender := identityset.Address(28)
			tsf := action.NewTransfer(big.NewInt(1), identityset.Address(29).String(), nil)
			elp := (&action.EnvelopeBuilder{}).
				SetAction(tsf).
				SetNonce(0).
				SetGasLimit(100000).
				SetGasPrice(big.NewInt(action.InitialBaseFee)).
				SetChainID(cfg.Chain.ID).
				SetVersion(1).
				Build()
			selp, err := action.Sign(elp, identityset.PrivateKey(28))
			r.NoError(err)
			r.NoError(ap.Add(context.Background(), selp))
			r.NotNil(sender)

			blk, err := bc.MintNewBlock(testutil.TimestampNow())
			r.NoError(err)
			minted := false
			for _, act := range blk.Actions {
				if _, ok := act.Action().(*action.Transfer); ok {
					minted = true
				}
			}
			r.True(minted, "the transfer must have been minted into the block")
			r.NotZero(blk.GasUsed(), "the mismatch below has to be distinguishable from a zero value")

			// The untampered block is the control: whatever else the validator
			// checks, it has to pass before a gas-field verdict means anything.
			blk.Receipts = nil
			r.NoError(bc.ValidateBlock(blk))

			tampered := rebuildBlockWithGasUsed(t, blk, blk.GasUsed()+1, producerKey)
			err = bc.ValidateBlock(tampered)
			if tc.wantErr == nil {
				r.NoError(err)
			} else {
				r.ErrorIs(err, tc.wantErr)
			}
		})
	}
}

func newHeaderGasChain(t *testing.T, gateHeight uint64) (blockchain.Blockchain, actpool.ActPool, crypto.PrivateKey, config.Config) {
	r := require.New(t)
	cfg := config.Default
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000000000000000000"
	cfg.Genesis.YapBetaBlockHeight = 1
	// The check rides Zanzibar Gamma; a chain that has activated none of the
	// family carries them equal, so set all three rather than Gamma alone.
	cfg.Genesis.ZanzibarBlockHeight = gateHeight
	cfg.Genesis.ZanzibarBetaBlockHeight = gateHeight
	cfg.Genesis.ZanzibarGammaBlockHeight = gateHeight
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)

	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	r.NoError(rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs,
	).Register(registry))

	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	r.NoError(err)
	genericValidator := protocol.NewGenericValidator(sf, accountutil.AccountState)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	r.NoError(err)
	ap.AddActionEnvelopeValidators(genericValidator)

	store, err := filedao.NewFileDAOInMemForTest()
	r.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain, cfg.Genesis, dao, factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(sf, genericValidator)),
	)
	r.NotNil(bc)
	r.NoError(rewarding.NewProtocol(cfg.Genesis.Rewarding).Register(registry))
	r.NoError(execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime, nil).Register(registry))
	r.NoError(poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates).Register(registry))

	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	r.NoError(bc.Start(ctx))
	t.Cleanup(func() { r.NoError(bc.Stop(ctx)) })
	r.NoError(sf.Start(ctx))

	keys := cfg.Chain.ProducerPrivateKeys()
	r.NotEmpty(keys)
	return bc, ap, keys[0], cfg
}

// rebuildBlockWithGasUsed re-signs a copy of blk carrying a header gasUsed that
// does not follow from its receipts, standing in for a block a peer published.
func rebuildBlockWithGasUsed(t *testing.T, blk *block.Block, gasUsed uint64, key crypto.PrivateKey) *block.Block {
	tampered, err := block.NewBuilder(blk.RunnableActions()).
		SetVersion(blk.Version()).
		SetHeight(blk.Height()).
		SetTimestamp(blk.Timestamp()).
		SetPrevBlockHash(blk.PrevHash()).
		SetDeltaStateDigest(blk.DeltaStateDigest()).
		SetReceiptRoot(blk.ReceiptRoot()).
		SetLogsBloom(blk.LogsBloomfilter()).
		SetBaseFee(blk.BaseFee()).
		SetExcessBlobGas(blk.ExcessBlobGas()).
		SetGasUsed(gasUsed).
		SetBlobGasUsed(blk.BlobGasUsed()).
		SignAndBuild(key)
	require.NoError(t, err)
	return &tampered
}
