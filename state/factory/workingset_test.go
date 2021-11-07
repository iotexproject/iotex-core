// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type testString struct {
	s string
}

func (s testString) Serialize() ([]byte, error) {
	return []byte(s.s), nil
}

func (s *testString) Deserialize(v []byte) error {
	s.s = string(v)
	return nil
}

func newFactoryWorkingSet(t testing.TB) *workingSet {
	r := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
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
	sf, err := NewStateDB(config.Default, InMemStateDBOption())
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
	var (
		require    = require.New(t)
		f1, _      = NewFactory(config.Default, InMemTrieOption())
		f2, _      = NewStateDB(config.Default, InMemStateDBOption())
		factories  = []Factory{f1, f2}
		digestHash = hash.Hash256b([]byte{65, 99, 99, 111, 117, 110, 116, 99, 117, 114, 114,
			101, 110, 116, 72, 101, 105, 103, 104, 116, 1, 0, 0, 0, 0, 0, 0, 0})
		tests = []struct {
			block *block.Block
			err   error
		}{
			{makeBlock(t, 1, hash.ZeroHash256, digestHash), nil},
			{
				makeBlock(t, 3, hash.ZeroHash256, digestHash),
				action.ErrNonceTooHigh,
			},
			{
				makeBlock(t, 1, hash.Hash256b([]byte("test")), digestHash),
				block.ErrReceiptRootMismatch,
			},
			{
				makeBlock(t, 1, hash.ZeroHash256, hash.Hash256b([]byte("test"))),
				block.ErrDeltaStateMismatch,
			},
		}
	)
	gasLimit := testutil.TestGasLimit * 100000
	zctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	zctx = genesis.WithGenesisContext(zctx, genesis.Default)
	for _, f := range factories {
		for _, test := range tests {
			require.Equal(test.err, errors.Cause(f.Validate(zctx, test.block)))
		}
	}
}

func makeBlock(t *testing.T, nonce uint64, rootHash hash.Hash256, digest hash.Hash256) *block.Block {
	rand.Seed(time.Now().Unix())
	var sevlps []action.SealedEnvelope
	r := rand.Int()
	tsf, err := action.NewTransfer(
		uint64(r),
		unit.ConvertIotxToRau(1000+int64(r)),
		identityset.Address(r%identityset.Size()).String(),
		nil,
		20000+uint64(r),
		unit.ConvertIotxToRau(1+int64(r)),
	)
	require.NoError(t, err)
	eb := action.EnvelopeBuilder{}
	evlp := eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(nonce).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey((r+1)%identityset.Size()))
	require.NoError(t, err)
	sevlps = append(sevlps, sevlp)
	rap := block.RunnableActionsBuilder{}
	ra := rap.AddActions(sevlps...).Build()
	blk, err := block.NewBuilder(ra).
		SetHeight(1).
		SetTimestamp(time.Now()).
		SetVersion(1).
		SetReceiptRoot(rootHash).
		SetDeltaStateDigest(digest).
		SetPrevBlockHash(hash.Hash256b([]byte("test"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(t, err)
	return &blk
}
