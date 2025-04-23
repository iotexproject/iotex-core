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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

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

func (v mockView) Clone() protocol.View {
	return v
}

func (v mockView) Snapshot() int {
	return 0
}

func (v mockView) Revert(int) error {
	return nil
}

func (v mockView) Commit(context.Context, protocol.StateReader) error {
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
	for _, f := range factories {
		for _, test := range tests {
			require.Equal(test.err, errors.Cause(f.Validate(zctx, test.block)))
		}
	}
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
