// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

type (
	testString struct {
		s string
	}

	workingSetCreator interface {
		newWorkingSet(context.Context, uint64) (*workingSet, error)
	}
)

func (s testString) Serialize() ([]byte, error) {
	return []byte(s.s), nil
}

func (s *testString) Deserialize(v []byte) error {
	s.s = string(v)
	return nil
}

func newStateDBWorkingSet(t testing.TB) *workingSet {
	r := require.New(t)
	sf, err := NewStateDB(DefaultConfig, db.NewMemKVStore())
	r.NoError(err)

	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.TestDefault(),
	)
	r.NoError(sf.Start(ctx))
	// defer r.NoError(sf.Stop(ctx))

	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	r.NoError(err)
	return ws
}

type mockView string

func (v mockView) Fork() protocol.View {
	return v
}

func (v mockView) Snapshot() int {
	return 0
}

func (v mockView) Revert(int) error {
	return nil
}

func (v mockView) Commit(context.Context, protocol.StateManager) error {
	return nil
}

func TestWorkingSet_ReadWriteView(t *testing.T) {
	var (
		r   = require.New(t)
		set = []*workingSet{
			newStateDBWorkingSet(t),
		}
		tests = map[string]mockView{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}
	)
	for _, ws := range set {
		for key, oval := range tests {
			val, err := ws.ReadView(key)
			r.Equal(protocol.ErrNoName, errors.Cause(err))
			r.Equal(val, nil)
			// write view into workingSet
			r.NoError(ws.WriteView(key, oval))
		}

		// read view and compare result
		for key, oval := range tests {
			val, err := ws.ReadView(key)
			r.NoError(err)
			r.Equal(oval, val)
		}

		// overwrite
		var newVal mockView = "testvalue"
		r.NoError(ws.WriteView("key1", newVal))
		val, err := ws.ReadView("key1")
		r.NoError(err)
		r.Equal(newVal, val)
	}
}

func TestWorkingSet_ValidateBlock(t *testing.T) {
	require := require.New(t)
	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
	var (
		f2, _          = NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
		factories      = []Factory{f2}
		digestHash, _  = hash.HexStringToHash256("43f69c954ea0138917d69a01f7ba47da74c99cb2c6229f5969a7f0bf53efb775")
		receiptRoot, _ = hash.HexStringToHash256("b8aaff4d845664a7a3f341f677365dafcdae0ae99a7fea821c7cc42c320acefe")
		tests          = []struct {
			block *block.Block
			err   error
		}{
			{
				makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, makeTransferAction(t, 1)),
				nil,
			},
			{
				makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, makeTransferAction(t, 3)),
				action.ErrNonceTooHigh,
			},
			{
				makeBlock(t, hash.ZeroHash256, hash.Hash256b([]byte("test")), digestHash, makeTransferAction(t, 1)),
				block.ErrReceiptRootMismatch,
			},
			{
				makeBlock(t, hash.ZeroHash256, receiptRoot, hash.Hash256b([]byte("test")), makeTransferAction(t, 1)),
				block.ErrDeltaStateMismatch,
			},
		}
	)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f2.Start(ctx))
	defer func() {
		require.NoError(f2.Stop(ctx))
	}()

	zctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	zctx = genesis.WithGenesisContext(zctx, cfg.Genesis)
	zctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(zctx, protocol.BlockchainCtx{
		ChainID: 1,
	}))
	zctx = protocol.WithFeatureWithHeightCtx(zctx)
	for _, f := range factories {
		for _, test := range tests {
			require.Equal(test.err, errors.Cause(f.Validate(zctx, test.block)))
		}
	}
}

// TestWorkingSet_ValidateBlock_RecoversPanic verifies that a panic raised while
// processing a proposed block's actions is recovered and surfaced as an error
// (the block is rejected) instead of crashing the validating node.
func TestWorkingSet_ValidateBlock_RecoversPanic(t *testing.T) {
	require := require.New(t)
	registry := protocol.NewRegistry()
	// a deposit-gas hook that panics stands in for any action handler that
	// panics while processing a proposed (untrusted) block
	panicDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		panic("injected panic while processing action")
	}
	require.NoError(account.NewProtocol(panicDeposit).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	require.NoError(err)

	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f.Start(startCtx))
	defer func() {
		require.NoError(f.Stop(startCtx))
	}()

	receiptRoot, _ := hash.HexStringToHash256("b8aaff4d845664a7a3f341f677365dafcdae0ae99a7fea821c7cc42c320acefe")
	digestHash, _ := hash.HexStringToHash256("43f69c954ea0138917d69a01f7ba47da74c99cb2c6229f5969a7f0bf53efb775")
	blk := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, makeTransferAction(t, 1))

	zctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	zctx = genesis.WithGenesisContext(zctx, cfg.Genesis)
	zctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(zctx, protocol.BlockchainCtx{
		ChainID: 1,
	}))
	zctx = protocol.WithFeatureWithHeightCtx(zctx)

	// the panic must be recovered and surfaced as an error, not crash the process
	var validateErr error
	require.NotPanics(func() { validateErr = f.Validate(zctx, blk) })
	require.Error(validateErr)
	require.Contains(validateErr.Error(), "recovered from panic")
}

// TestWorkingSet_Mint_RecoversActionPanic verifies that a panic raised while running a
// single pending action during minting is recovered into an error (the draft is discarded,
// same as today) and that the offending sender is evicted from the pool — so the next mint
// attempt no longer sees the poison action, turning what would otherwise be a persistent
// block-production stall into a one-time lost draft.
func TestWorkingSet_Mint_RecoversActionPanic(t *testing.T) {
	require := require.New(t)
	registry := protocol.NewRegistry()
	panicDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		panic("injected panic while running action during mint")
	}
	require.NoError(account.NewProtocol(panicDeposit).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	require.NoError(err)

	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f.Start(startCtx))
	defer func() {
		require.NoError(f.Stop(startCtx))
	}()

	selp := makeTransferAction(t, 1)
	sender := identityset.Address(28)

	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		sender.String(): {selp},
	}).Times(1)
	ap.EXPECT().DeleteAction(gomock.Any()).Do(func(addr address.Address) {
		require.Equal(sender.String(), addr.String())
	}).Times(1)

	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	// the panic must be recovered and surfaced as an error, not crash the mint goroutine,
	// and the poison action's sender must be evicted from the pool (see ap.EXPECT().DeleteAction above)
	var mintErr error
	require.NotPanics(func() {
		_, mintErr = f.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.Error(mintErr)
	require.Contains(mintErr.Error(), "recovered from panic")
}

func TestWorkingSet_ValidateBlock_SystemAction(t *testing.T) {
	require := require.New(t)
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.VanuatuBlockHeight = 1 // enable validate system action
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	require.NoError(rewarding.NewProtocol(cfg.Genesis.Rewarding).Register(registry))
	var (
		f2, _     = NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
		factories = []Factory{f2}
	)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f2.Start(ctx))
	defer func() {
		require.NoError(f2.Stop(ctx))
	}()

	zctx := protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: uint64(1),
		Producer:    identityset.Address(28),
		GasLimit:    testutil.TestGasLimit * 100000,
	})
	zctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(zctx, protocol.BlockchainCtx{
		ChainID: 1,
	}))
	zctx = protocol.WithFeatureWithHeightCtx(zctx)

	t.Run("missing system action", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("8f9b7694c325a4f4b0065cd382f8af0a4e913113a4ce7ef1ac899f96158c74f4")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("f04673451e31386a8fddfcf7750665bfcf33f239f6c4919430bb11a144e1aa95")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 0)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
	t.Run("system action not on tail", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("8f9b7694c325a4f4b0065cd382f8af0a4e913113a4ce7ef1ac899f96158c74f4")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("f04673451e31386a8fddfcf7750665bfcf33f239f6c4919430bb11a144e1aa95")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeRewardAction(t, 28), makeTransferAction(t, 0)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
	t.Run("correct system action", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("da051302d6e0b433d54225892789ce24dd634b1c17a6fa443a8a8cab27e2c586")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("afd544c5cf1b4b88216504a3b08d535314470adf6e45c68f9d0bb9e5c3699948")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 0), makeRewardAction(t, 28)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), nil)
		}
	})
	t.Run("wrong reward action signer", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("ade24a5c647b5af34c4e74fe0d8f1fa410f6fb115f8fc2d39e45ca2f895de9ca")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("a59bd06fe4d2bb537895f170dec1f9213045cb13480e4941f1abdc8d13b16fae")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 0), makeRewardAction(t, 27)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorContains(f.Validate(zctx, block), "Only producer could create reward")
		}
	})
	t.Run("postiche system action", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("ade24a5c647b5af34c4e74fe0d8f1fa410f6fb115f8fc2d39e45ca2f895de9ca")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("a59bd06fe4d2bb537895f170dec1f9213045cb13480e4941f1abdc8d13b16fae")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 0), makeRewardAction(t, 28), makeRewardAction(t, 28)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
	t.Run("inconsistent system action", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("8f9b7694c325a4f4b0065cd382f8af0a4e913113a4ce7ef1ac899f96158c74f4")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("f04673451e31386a8fddfcf7750665bfcf33f239f6c4919430bb11a144e1aa95")
		require.NoError(err)
		rewardAct := makeRewardAction(t, 28)
		rewardAct.SetNonce(2)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 0), rewardAct}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
}

// TestWorkingSet_ValidateBlock_HeaderGasFields checks that a validating node
// re-derives the header's gasUsed and blobGasUsed from the receipts it produced
// while executing the block, and rejects a header whose values disagree.
//
// The check is gated: before the fork height a header carrying gas fields that
// do not match its receipts must still be accepted, so that already-committed
// blocks keep replaying.
func TestWorkingSet_ValidateBlock_HeaderGasFields(t *testing.T) {
	tests := []struct {
		name string
		// gateHeight is the Zanzibar Gamma height the check is wired to;
		// math.MaxUint64 is the shipped default, i.e. the check is off
		gateHeight   uint64
		gasUsedDelta uint64
		blobGasUsed  uint64
		expectedErr  error
	}{
		{"pre-fork gas used mismatch accepted", math.MaxUint64, 1, 0, nil},
		{"pre-fork blob gas used mismatch accepted", math.MaxUint64, 0, params.BlobTxBlobGasPerBlob, nil},
		{"post-fork matching header accepted", 1, 0, 0, nil},
		{"post-fork gas used mismatch rejected", 1, 1, 0, block.ErrGasUsedMismatch},
		{"post-fork blob gas used mismatch rejected", 1, 0, params.BlobTxBlobGasPerBlob, block.ErrBlobGasUsedMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			cfg := gasFieldTestConfig(test.gateHeight)
			minter, ctx := newGasFieldTestFactory(t, cfg)
			validator, _ := newGasFieldTestFactory(t, cfg)

			ctrl := gomock.NewController(t)
			ap := mock_actpool.NewMockActPool(ctrl)
			ap.EXPECT().BundlePool().Return(nil).Times(1)
			ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
				identityset.Address(28).String(): {makeGasFieldTransferAction(t, 0)},
			}).Times(1)

			blk, err := minter.Mint(ctx, ap, identityset.PrivateKey(27))
			require.NoError(err)
			// the proposer must have put a non-zero gas used in the header,
			// otherwise the mismatch below would be indistinguishable from the
			// zero value
			require.NotZero(blk.GasUsed())
			require.Zero(blk.BlobGasUsed())

			tampered := rebuildWithGasFields(t, blk, blk.GasUsed()+test.gasUsedDelta, test.blobGasUsed)
			err = validator.Validate(ctx, tampered)
			if test.expectedErr == nil {
				require.NoError(err)
			} else {
				require.ErrorIs(err, test.expectedErr)
			}
		})
	}
}

// gasFieldTestConfig returns a config where every fork but the one gating the
// header gas check is active at height 1, so the two cases differ only by that
// gate.
func gasFieldTestConfig(gateHeight uint64) Config {
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.YapBetaBlockHeight = 1
	// All three at one height: the check rides Zanzibar Gamma, and a chain
	// that has activated none of the family carries them equal. Setting the
	// later ones alone would bake a partial-family genesis into the test.
	cfg.Genesis.ZanzibarBlockHeight = gateHeight
	cfg.Genesis.ZanzibarBetaBlockHeight = gateHeight
	cfg.Genesis.ZanzibarGammaBlockHeight = gateHeight
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000000000000000"
	return cfg
}

func newGasFieldTestFactory(t *testing.T, cfg Config) (Factory, context.Context) {
	require := require.New(t)
	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	// A real KV store rather than db.NewMemKVStore(): committing a block range
	// scans, which the in-memory store does not support, and the fork-boundary
	// case has to commit one block to reach the next height.
	dbPath, err := testutil.PathOfTempFile("gas-field-statedb")
	require.NoError(err)
	t.Cleanup(func() { testutil.CleanupPath(dbPath) })
	chainCfg := cfg.Chain
	chainCfg.TrieDBPath = dbPath
	chainCfg.TrieDBPatchFile = ""
	cfg.Chain = chainCfg
	kv, err := db.CreateKVStoreWithCache(db.DefaultConfig, dbPath, cfg.Chain.StateDBCacheSize)
	require.NoError(err)
	f, err := NewStateDB(cfg, kv, RegistryStateDBOption(registry))
	require.NoError(err)
	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f.Start(startCtx))
	t.Cleanup(func() {
		require.NoError(f.Stop(startCtx))
	})

	return f, gasFieldCtx(cfg, protocol.TipInfo{})
}

// gasFieldCtx builds the context for the block that follows tip. Mint derives
// the height it is being asked for from the tip it is handed, so the
// fork-boundary case advances the tip to cross an activation height, and the
// feature context follows from the same height.
func gasFieldCtx(cfg Config, tip protocol.TipInfo) context.Context {
	height := tip.Height + 1
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight:    height,
		BlockTimeStamp: time.Unix(cfg.Genesis.Timestamp, 0).Add(time.Duration(height) * time.Second),
		Producer:       identityset.Address(27),
		GasLimit:       testutil.TestGasLimit * 100000,
		BaseFee:        big.NewInt(action.InitialBaseFee),
	})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{ChainID: 1, Tip: tip},
	)
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

func makeGasFieldTransferAction(t *testing.T, nonce uint64) *action.SealedEnvelope {
	tsf := action.NewTransfer(big.NewInt(1), identityset.Address(29).String(), nil)
	evlp := (&action.EnvelopeBuilder{}).
		SetAction(tsf).
		SetGasLimit(testutil.TestGasLimit).
		SetGasPrice(big.NewInt(action.InitialBaseFee)).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(28))
	require.NoError(t, err)
	return sevlp
}

// rebuildWithGasFields re-signs a copy of blk that carries the given gas fields
// and is otherwise identical, standing in for a header a proposer published
// with gas fields that do not follow from its receipts.
func rebuildWithGasFields(t *testing.T, blk *block.Block, gasUsed, blobGasUsed uint64) *block.Block {
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
		SetBlobGasUsed(blobGasUsed).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	return &tampered
}

func makeTransferAction(t *testing.T, nonce uint64) *action.SealedEnvelope {
	tsf := action.NewTransfer(big.NewInt(1), identityset.Address(29).String(), nil)
	evlp := (&action.EnvelopeBuilder{}).
		SetAction(tsf).
		SetGasLimit(testutil.TestGasLimit).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(28))
	require.NoError(t, err)
	return sevlp
}

func makeRewardAction(t *testing.T, signer int) *action.SealedEnvelope {
	grant := action.NewGrantReward(action.BlockReward, 1)
	eb2 := action.EnvelopeBuilder{}
	evlp := eb2.SetNonce(0).SetGasPrice(big.NewInt(0)).
		SetAction(grant).Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(signer))
	require.NoError(t, err)
	return sevlp
}

func makeBlock(t *testing.T, prevHash hash.Hash256, receiptRoot hash.Hash256, digest hash.Hash256, actions ...*action.SealedEnvelope) *block.Block {
	rap := block.RunnableActionsBuilder{}
	ra := rap.AddActions(actions...).Build()
	blk, err := block.NewBuilder(ra).
		SetHeight(1).
		SetTimestamp(time.Now()).
		SetVersion(1).
		SetReceiptRoot(receiptRoot).
		SetDeltaStateDigest(digest).
		SetPrevBlockHash(prevHash).
		SetBaseFee(big.NewInt(unit.Qev)).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}

// TestWorkingSet_ValidateBlock_HeaderGasFieldsForkBoundary walks the activation
// height itself rather than comparing a gate that is off against one that has
// been on all along. The same tampering is applied to the block at fork-1 and
// to the block at fork, against one config and one pair of factories, so the
// only thing that differs between the two outcomes is the height the block
// lands on.
func TestWorkingSet_ValidateBlock_HeaderGasFieldsForkBoundary(t *testing.T) {
	require := require.New(t)
	const forkHeight = uint64(2)
	cfg := gasFieldTestConfig(forkHeight)
	// Keep the earlier Zanzibar family members active at fork-1. This pins
	// the header check to Gamma rather than merely testing a family-wide gate.
	cfg.Genesis.ZanzibarBlockHeight = forkHeight - 1
	cfg.Genesis.ZanzibarBetaBlockHeight = forkHeight - 1
	minter, _ := newGasFieldTestFactory(t, cfg)
	validator, _ := newGasFieldTestFactory(t, cfg)

	// mintTamperValidate mints the block at height, hands the validator a copy
	// whose gasUsed does not follow from its receipts, and then commits the
	// untampered block on both factories so the next height can be minted.
	tip := protocol.TipInfo{}
	mintTamperValidate := func(height, nonce uint64) error {
		ctx := gasFieldCtx(cfg, tip)
		ctrl := gomock.NewController(t)
		ap := mock_actpool.NewMockActPool(ctrl)
		ap.EXPECT().BundlePool().Return(nil).Times(1)
		ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
			identityset.Address(28).String(): {makeGasFieldTransferAction(t, nonce)},
		}).Times(1)

		blk, err := minter.Mint(ctx, ap, identityset.PrivateKey(27))
		require.NoError(err)
		require.Equal(height, blk.Height())
		require.NotZero(blk.GasUsed(), "the mismatch below has to be distinguishable from a zero value")

		verr := validator.Validate(ctx, rebuildWithGasFields(t, blk, blk.GasUsed()+1, 0))

		require.NoError(minter.PutBlock(ctx, blk))
		require.NoError(validator.PutBlock(ctx, blk))
		// EIP-1559 header verification derives the next base fee from the
		// parent, so the tip has to carry the whole gas picture, not just the
		// height and hash.
		tip = protocol.TipInfo{
			Height:        blk.Height(),
			Hash:          blk.HashBlock(),
			Timestamp:     blk.Timestamp(),
			GasUsed:       blk.GasUsed(),
			BaseFee:       blk.BaseFee(),
			BlobGasUsed:   blk.BlobGasUsed(),
			ExcessBlobGas: blk.ExcessBlobGas(),
		}
		return verr
	}

	// At fork-1 the check is still off, so a header that disagrees with its
	// receipts has to be accepted: committed history must keep replaying.
	require.NoError(mintTamperValidate(forkHeight-1, 0))

	// At the activation height exactly, the same tampering is rejected.
	require.ErrorIs(mintTamperValidate(forkHeight, 1), block.ErrGasUsedMismatch)
}

// gasReportingProtocol is a post-action handler that rewrites the gas reported
// by the receipt of the action carrying a given nonce. It lets a test drive
// the remaining-block-gas bookkeeping with a receipt that reports more gas
// than the block has left.
type gasReportingProtocol struct {
	nonce       uint64
	gasConsumed uint64
}

func (p *gasReportingProtocol) Handle(context.Context, action.Envelope, protocol.StateManager) (*action.Receipt, error) {
	return nil, nil
}

func (p *gasReportingProtocol) HandleReceipt(_ context.Context, elp action.Envelope, _ protocol.StateManager, receipt *action.Receipt) error {
	if elp.Nonce() == p.nonce {
		receipt.GasConsumed = p.gasConsumed
	}
	return nil
}

func (p *gasReportingProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (p *gasReportingProtocol) Register(r *protocol.Registry) error {
	return r.Register(p.Name(), p)
}

func (p *gasReportingProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

func (p *gasReportingProtocol) Name() string { return "gasReporting" }

// TestWorkingSet_RemainingBlockGas covers what happens when a receipt reports
// more gas than the block has left. The remaining budget is an unsigned
// counter: before the fix height the subtraction is unchecked and wraps to a
// huge value, so the rest of the block is processed as if the budget were
// untouched; from the fix height on it saturates at zero and the next action
// is rejected for want of gas.
func TestWorkingSet_RemainingBlockGas(t *testing.T) {
	const blkGasLimit = uint64(15000)
	for _, tt := range []struct {
		name string
		// height from which the checked subtraction takes effect
		gateHeight uint64
		// whether the action following the over-reporting one is refused
		wantNextActionOutOfGas bool
	}{
		{
			name:                   "before fix height",
			gateHeight:             math.MaxUint64,
			wantNextActionOutOfGas: false,
		},
		{
			name:                   "from fix height",
			gateHeight:             1,
			wantNextActionOutOfGas: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			cfg := Config{
				Chain:   blockchain.DefaultConfig,
				Genesis: genesis.TestDefault(),
			}
			cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
			// the remaining budget is only carried across actions on the
			// post-Vanuatu processing path
			cfg.Genesis.VanuatuBlockHeight = 1
			// The corrections ride Zanzibar Gamma; a chain that has activated
			// none of the family carries the heights equal, so set all three
			// rather than leaving a partial-family genesis in a test.
			cfg.Genesis.ZanzibarBlockHeight = tt.gateHeight
			cfg.Genesis.ZanzibarBetaBlockHeight = tt.gateHeight
			cfg.Genesis.ZanzibarGammaBlockHeight = tt.gateHeight
			registry := protocol.NewRegistry()
			require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
			// makes the first transfer report one unit more gas than the block
			// has to give
			require.NoError((&gasReportingProtocol{nonce: 0, gasConsumed: blkGasLimit + 1}).Register(registry))
			f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
			require.NoError(err)

			startCtx := protocol.WithBlockCtx(
				genesis.WithGenesisContext(context.Background(), cfg.Genesis),
				protocol.BlockCtx{},
			)
			require.NoError(f.Start(startCtx))
			defer func() {
				require.NoError(f.Stop(startCtx))
			}()

			blk := makeBlock(t, hash.ZeroHash256, hash.ZeroHash256, hash.ZeroHash256,
				makeTransferAction(t, 0), makeTransferAction(t, 1))

			zctx := protocol.WithBlockCtx(context.Background(),
				protocol.BlockCtx{
					BlockHeight: uint64(1),
					Producer:    identityset.Address(27),
					GasLimit:    blkGasLimit,
				})
			zctx = genesis.WithGenesisContext(zctx, cfg.Genesis)
			zctx = protocol.WithFeatureCtx(protocol.WithBlockchainCtx(zctx, protocol.BlockchainCtx{
				ChainID: 1,
			}))
			zctx = protocol.WithFeatureWithHeightCtx(zctx)

			err = f.Validate(zctx, blk)
			require.Error(err)
			if tt.wantNextActionOutOfGas {
				require.ErrorIs(err, action.ErrGasLimit)
			} else {
				// the budget wrapped around, so the second action was run
				// anyway and the block only failed later, on the state digest
				require.NotErrorIs(err, action.ErrGasLimit)
				require.ErrorIs(err, block.ErrDeltaStateMismatch)
			}
		})
	}
}

// viewTrace records what a protocol view was asked to do. Forks share it, so a
// test can watch a view that only ever exists inside Mint.
type viewTrace struct {
	mu  sync.Mutex
	ops []string
}

func (tr *viewTrace) record(op string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.ops = append(tr.ops, op)
}

func (tr *viewTrace) snapshot() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return append([]string(nil), tr.ops...)
}

// countingView is a minimal protocol view: an integer every handled action
// bumps, with the snapshot and revert semantics the working set relies on.
type countingView struct {
	n         int
	snapshots []int
	trace     *viewTrace
}

func (v *countingView) Fork() protocol.View {
	// The fork shares the trace on purpose: it is the only handle the test has
	// on the copy the working set actually mutates.
	return &countingView{n: v.n, snapshots: append([]int(nil), v.snapshots...), trace: v.trace}
}

func (v *countingView) Snapshot() int {
	v.snapshots = append(v.snapshots, v.n)
	v.trace.record("snapshot")
	return len(v.snapshots) - 1
}

func (v *countingView) Revert(i int) error {
	if i < 0 || i >= len(v.snapshots) {
		return errors.Errorf("invalid snapshot index %d", i)
	}
	v.n = v.snapshots[i]
	v.snapshots = v.snapshots[:i]
	v.trace.record("revert")
	return nil
}

func (v *countingView) Commit(context.Context, protocol.StateManager) error { return nil }

// viewBumpProtocol mutates its view for every transfer it sees and fails on one
// designated recipient, which is how the bundle below is made to give up after
// it has already changed a view.
type viewBumpProtocol struct {
	poison string
	trace  *viewTrace
}

func (p *viewBumpProtocol) Name() string { return "viewbump" }

func (p *viewBumpProtocol) Start(context.Context, protocol.StateReader) (protocol.View, error) {
	return &countingView{trace: p.trace}, nil
}

func (p *viewBumpProtocol) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	tsf, ok := elp.Action().(*action.Transfer)
	if !ok {
		return nil, nil
	}
	v, err := sm.ReadView(p.Name())
	if err != nil {
		return nil, err
	}
	cv, ok := v.(*countingView)
	if !ok {
		return nil, errors.Errorf("unexpected view type %T", v)
	}
	cv.n++
	cv.trace.record("bump")
	if tsf.Recipient() == p.poison {
		return nil, errors.New("injected failure inside bundle")
	}
	// Leave the receipt to the account protocol.
	return nil, nil
}

func (p *viewBumpProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (p *viewBumpProtocol) Register(r *protocol.Registry) error { return r.Register(p.Name(), p) }

func (p *viewBumpProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

func bundleTransfer(t *testing.T, nonce uint64, recipient string) *action.SealedEnvelope {
	tsf := action.NewTransfer(big.NewInt(1), recipient, nil)
	evlp := (&action.EnvelopeBuilder{}).
		SetAction(tsf).
		SetGasLimit(testutil.TestGasLimit).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(28))
	require.NoError(t, err)
	return sevlp
}

// TestWorkingSet_Mint_BundleAbortRevertsView pins the rollback a bundle takes
// when it gives up part way through. The store snapshot alone leaves protocol
// views carrying the changes of a bundle that was dropped, and the block the
// proposer goes on to build then reflects actions it did not include. Reverting
// through the working set puts both back.
func TestWorkingSet_Mint_BundleAbortRevertsView(t *testing.T) {
	require := require.New(t)
	trace := &viewTrace{}
	registry := protocol.NewRegistry()
	// Registered ahead of the account protocol: runAction walks the registry in
	// registration order and stops at the first handler that returns a receipt,
	// so this one has to see the action before account settles it.
	poison := identityset.Address(30).String()
	require.NoError((&viewBumpProtocol{poison: poison, trace: trace}).Register(registry))
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))

	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000000000000000"
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	require.NoError(err)

	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f.Start(startCtx))
	defer func() { require.NoError(f.Stop(startCtx)) }()

	// A bundle whose second action fails, after the first has already moved the
	// view. Bundles only accept transfers, executions and tx containers.
	bundle := action.NewBundle()
	require.NoError(bundle.Add(
		bundleTransfer(t, 1, identityset.Address(29).String()),
		bundleTransfer(t, 2, poison),
	))
	bundle.SetTargetBlockHeight(1)

	bp := actpool.NewBundlePool(cfg.Genesis)
	require.NoError(bp.AddBundle(context.Background(), identityset.Address(28), "bundle-uuid", bundle))

	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().BundlePool().Return(bp).Times(1)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{}).Times(1)

	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	blk, err := f.Mint(ctx, ap, identityset.PrivateKey(27))
	require.NoError(err, "a failed bundle is skipped, it must not fail the mint")
	for _, selp := range blk.Actions {
		if tsf, ok := selp.Action().(*action.Transfer); ok {
			require.NotEqual(poison, tsf.Recipient(), "the dropped bundle must not reach the block")
		}
	}

	ops := trace.snapshot()
	require.Contains(ops, "bump", "the bundle must have reached the protocol")
	require.Contains(ops, "revert", "the dropped bundle must have rolled its view back")
	// The revert has to come after the mutations it undoes.
	require.Greater(indexOfLast(ops, "revert"), indexOfFirst(ops, "bump"))
}

func indexOfFirst(ops []string, want string) int {
	for i, op := range ops {
		if op == want {
			return i
		}
	}
	return -1
}

func indexOfLast(ops []string, want string) int {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i] == want {
			return i
		}
	}
	return -1
}
