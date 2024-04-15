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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
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

func newFactoryWorkingSet(t testing.TB) *workingSet {
	r := require.New(t)
	sf, err := NewFactory(DefaultConfig, db.NewMemKVStore())
	r.NoError(err)

	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.Default,
	)
	r.NoError(sf.Start(ctx))
	// defer r.NoError(sf.Stop(ctx))

	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	r.NoError(err)
	return ws
}

func newStateDBWorkingSet(t testing.TB) *workingSet {
	r := require.New(t)
	sf, err := NewStateDB(DefaultConfig, db.NewMemKVStore())
	r.NoError(err)

	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		genesis.Default,
	)
	r.NoError(sf.Start(ctx))
	// defer r.NoError(sf.Stop(ctx))

	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	r.NoError(err)
	return ws
}

func TestWorkingSet_ReadWriteView(t *testing.T) {
	var (
		r   = require.New(t)
		set = []*workingSet{
			newFactoryWorkingSet(t),
			newStateDBWorkingSet(t),
		}
		tests = []struct{ key, val string }{
			{"key1", "value1"},
			{"key2", "value2"},
			{"key3", "value3"},
			{"key4", "value4"},
		}
	)
	for _, ws := range set {
		for _, test := range tests {
			val, err := ws.ReadView(test.key)
			r.Equal(protocol.ErrNoName, errors.Cause(err))
			r.Equal(val, nil)
			// write view into workingSet
			r.NoError(ws.WriteView(test.key, test.val))
		}

		// read view and compare result
		for _, test := range tests {
			val, err := ws.ReadView(test.key)
			r.NoError(err)
			r.Equal(test.val, val)
		}

		// overwrite
		newVal := "testvalue"
		r.NoError(ws.WriteView(tests[0].key, newVal))
		val, err := ws.ReadView(tests[0].key)
		r.NoError(err)
		r.Equal(newVal, val)
	}
}

func TestWorkingSet_Dock(t *testing.T) {
	var (
		r   = require.New(t)
		set = []*workingSet{
			newFactoryWorkingSet(t),
			newStateDBWorkingSet(t),
		}
		tests = []struct {
			name, key string
			val       state.Serializer
		}{
			{
				"ns", "test1", &testString{"v1"},
			},
			{
				"ns", "test2", &testString{"v2"},
			},
			{
				"vs", "test3", &testString{"v3"},
			},
			{
				"ts", "test4", &testString{"v4"},
			},
		}
	)
	for _, ws := range set {
		// test empty dock
		ts := &testString{}
		for _, e := range tests {
			r.False(ws.ProtocolDirty(e.name))
			r.Equal(protocol.ErrNoName, ws.Unload(e.name, e.key, ts))
		}

		// populate the dock, and verify existence
		for _, e := range tests {
			r.NoError(ws.Load(e.name, e.key, e.val))
			r.True(ws.ProtocolDirty(e.name))
			r.NoError(ws.Unload(e.name, e.key, ts))
			r.Equal(e.val, ts)
			// test key that does not exist
			r.Equal(protocol.ErrNoName, ws.Unload(e.name, "notexist", ts))
		}

		// overwrite
		v5 := &testString{"v5"}
		r.NoError(ws.Load(tests[1].name, tests[1].key, v5))
		r.NoError(ws.Unload(tests[1].name, tests[1].key, ts))
		r.Equal(v5, ts)

		// add a new one
		v6 := &testString{"v6"}
		r.NoError(ws.Load("as", "test6", v6))
		r.True(ws.ProtocolDirty("as"))
		r.NoError(ws.Unload("as", "test6", ts))
		r.Equal(v6, ts)

		ws.Reset()
		for _, e := range tests {
			r.False(ws.ProtocolDirty(e.name))
		}
		r.False(ws.ProtocolDirty("as"))
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
		f1, _          = NewFactory(cfg, db.NewMemKVStore(), RegistryOption(registry))
		f2, _          = NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
		factories      = []Factory{f1, f2}
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
	require.NoError(f1.Start(ctx))
	require.NoError(f2.Start(ctx))
	defer func() {
		require.NoError(f1.Stop(ctx))
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
	cfg.Genesis.QuebecBlockHeight = 1 // enable validate system action
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = "100000000"
	registry := protocol.NewRegistry()
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	require.NoError(rewarding.NewProtocol(cfg.Genesis.Rewarding).Register(registry))
	var (
		f1, _     = NewFactory(cfg, db.NewMemKVStore(), RegistryOption(registry))
		f2, _     = NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
		factories = []Factory{f1, f2}
	)

	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f1.Start(ctx))
	require.NoError(f2.Start(ctx))
	defer func() {
		require.NoError(f1.Stop(ctx))
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
		actions := []*action.SealedEnvelope{makeTransferAction(t, 1)}
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
		actions := []*action.SealedEnvelope{makeRewardAction(t, 28), makeTransferAction(t, 1)}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
	t.Run("correct system action", func(t *testing.T) {
		digestHash, err := hash.HexStringToHash256("ade24a5c647b5af34c4e74fe0d8f1fa410f6fb115f8fc2d39e45ca2f895de9ca")
		require.NoError(err)
		receiptRoot, err := hash.HexStringToHash256("a59bd06fe4d2bb537895f170dec1f9213045cb13480e4941f1abdc8d13b16fae")
		require.NoError(err)
		actions := []*action.SealedEnvelope{makeTransferAction(t, 1), makeRewardAction(t, 28)}
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
		actions := []*action.SealedEnvelope{makeTransferAction(t, 1), makeRewardAction(t, 27)}
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
		actions := []*action.SealedEnvelope{makeTransferAction(t, 1), makeRewardAction(t, 28), makeRewardAction(t, 28)}
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
		actions := []*action.SealedEnvelope{makeTransferAction(t, 1), rewardAct}
		for _, f := range factories {
			block := makeBlock(t, hash.ZeroHash256, receiptRoot, digestHash, actions...)
			require.ErrorIs(f.Validate(zctx, block), errInvalidSystemActionLayout)
		}
	})
}

func makeTransferAction(t *testing.T, nonce uint64) *action.SealedEnvelope {
	tsf, err := action.NewTransfer(
		uint64(nonce),
		big.NewInt(1),
		identityset.Address(29).String(),
		nil,
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	require.NoError(t, err)
	eb := action.EnvelopeBuilder{}
	evlp := eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(nonce).
		SetChainID(1).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(28))
	require.NoError(t, err)
	return sevlp
}

func makeRewardAction(t *testing.T, signer int) *action.SealedEnvelope {
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(action.BlockReward).SetHeight(1).Build()
	eb2 := action.EnvelopeBuilder{}
	evlp := eb2.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
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
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}
