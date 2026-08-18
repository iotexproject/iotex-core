// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
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

// TestWorkingSet_Mint_SkipsPanickingAction verifies that an action panicking
// while it is being run during block production does not stall the proposer:
// the panic is turned into an error, the offending sender is dropped from the
// action pool, and the next mint produces a block without it.
func TestWorkingSet_Mint_SkipsPanickingAction(t *testing.T) {
	require := require.New(t)
	var (
		badSender  = identityset.Address(28)
		goodSender = identityset.Address(29)
	)
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	cfg.Genesis.InitBalanceMap[badSender.String()] = "100000000000000000000"
	cfg.Genesis.InitBalanceMap[goodSender.String()] = "100000000000000000000"

	registry := protocol.NewRegistry()
	// a deposit-gas hook that panics for one particular sender stands in for any
	// action handler that panics on a specific action
	depositGas := func(ctx context.Context, _ protocol.StateManager, _ *big.Int, _ ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		if actCtx, ok := protocol.GetActionCtx(ctx); ok && actCtx.Caller.String() == badSender.String() {
			panic("injected panic while running action")
		}
		return nil, nil
	}
	require.NoError(account.NewProtocol(depositGas).Register(registry))

	sdb, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	require.NoError(err)
	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(sdb.Start(startCtx))
	defer func() {
		require.NoError(sdb.Stop(startCtx))
	}()

	badAct, err := action.SignedTransfer(goodSender.String(), identityset.PrivateKey(28), 1, big.NewInt(1), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	badHash, err := badAct.Hash()
	require.NoError(err)
	goodAct, err := action.SignedTransfer(badSender.String(), identityset.PrivateKey(29), 1, big.NewInt(1), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	goodHash, err := goodAct.Hash()
	require.NoError(err)

	pending := map[string][]*action.SealedEnvelope{
		badSender.String():  {badAct},
		goodSender.String(): {goodAct},
	}
	deleted := []string{}
	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().BundlePool().Return(nil).AnyTimes()
	ap.EXPECT().PendingActionMap().DoAndReturn(func() map[string][]*action.SealedEnvelope {
		snapshot := make(map[string][]*action.SealedEnvelope, len(pending))
		for sender, acts := range pending {
			snapshot[sender] = acts
		}
		return snapshot
	}).AnyTimes()
	ap.EXPECT().DeleteAction(gomock.Any()).Do(func(caller address.Address) {
		deleted = append(deleted, caller.String())
		delete(pending, caller.String())
	}).AnyTimes()

	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: 1,
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	// first round: the offending action aborts the draft and its sender is dropped
	var blk *block.Block
	require.NotPanics(func() {
		blk, err = sdb.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.Error(err)
	require.Contains(err.Error(), "panic while running action")
	require.Nil(blk)
	require.Equal([]string{badSender.String()}, deleted)

	// second round: block production resumes without the offending action
	require.NotPanics(func() {
		blk, err = sdb.Mint(ctx, ap, identityset.PrivateKey(27))
	})
	require.NoError(err)
	require.NotNil(blk)
	minted := make(map[hash.Hash256]bool)
	for _, act := range blk.Actions {
		h, err := act.Hash()
		require.NoError(err)
		minted[h] = true
	}
	require.True(minted[goodHash])
	require.False(minted[badHash])
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
