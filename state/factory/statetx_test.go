// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/require"
)

func TestSDBSnapshot(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)
	require.NoError(sdb.Start(context.Background()))
	addr := testaddress.Addrinfo["alfa"].String()
	ws, err := sdb.NewWorkingSet()
	require.NoError(err)
	_, err = util.LoadOrCreateAccount(ws, addr, big.NewInt(5))
	require.NoError(err)
	sHash := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())

	s, err := util.LoadAccount(ws, sHash)
	require.NoError(err)
	require.Equal(big.NewInt(5), s.Balance)
	s0 := ws.Snapshot()
	require.Zero(s0)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(10), s.Balance)
	require.NoError(ws.PutState(sHash, s))
	s1 := ws.Snapshot()
	require.Equal(1, s1)
	s.Balance.Add(s.Balance, big.NewInt(5))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.PutState(sHash, s))
	// add another account
	addr = testaddress.Addrinfo["bravo"].String()
	_, err = util.LoadOrCreateAccount(ws, addr, big.NewInt(7))
	require.NoError(err)
	tHash := byteutil.BytesTo20B(testaddress.Addrinfo["bravo"].Bytes())

	s, err = util.LoadAccount(ws, tHash)
	require.NoError(err)
	require.Equal(big.NewInt(7), s.Balance)
	s2 := ws.Snapshot()
	require.Equal(2, s2)
	require.NoError(s.AddBalance(big.NewInt(6)))
	require.Equal(big.NewInt(13), s.Balance)
	require.NoError(ws.PutState(tHash, s))

	require.NoError(ws.Revert(s2))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(15), s.Balance)
	require.NoError(ws.State(tHash, s))
	require.Equal(big.NewInt(7), s.Balance)
	require.NoError(ws.Revert(s1))
	require.NoError(ws.State(sHash, s))
	require.Equal(big.NewInt(10), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
	require.NoError(ws.Revert(s0))
	require.NoError(ws.State(sHash, s))

	require.Equal(big.NewInt(5), s.Balance)
	require.Equal(state.ErrStateNotExist, errors.Cause(ws.State(tHash, s)))
}

func TestRunActions(t *testing.T) {
	require := require.New(t)
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	require.Equal(uint64(0), ws.Version())
	ws.dao.Start(context.Background())
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey
	_, err := util.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(err)
	_, err = util.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(err)

	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), b, nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetDestinationAddress(b).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tx2, err := action.NewTransfer(uint64(1), big.NewInt(20), a, nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetDestinationAddress(b).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})
	_, _, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{selp1, selp2})
	require.NoError(err)
	require.NoError(ws.Commit())
	hash1 := ws.UpdateBlockLevelInfo(1)
	hash2 := ws.RootHash()
	require.Equal(hash.ZeroHash256, hash1, hash2)
	h := ws.Height()
	require.Equal(uint64(1), h)
}

func TestCachedBatch(t *testing.T) {
	require := require.New(t)
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	hashA := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
	accountA := state.EmptyAccount()
	accountA.Balance = big.NewInt(70)
	accountA.VotingWeight = big.NewInt(70)
	hash1 := ws.Digest()
	require.NotEqual(hash.ZeroHash256, hash1)
	err := ws.PutState(hashA, accountA)
	require.NoError(err)
	hash2 := ws.Digest()
	require.NotEqual(hash1, hash2)
	testAccount := state.EmptyAccount()
	err = ws.State(hashA, &testAccount)
	require.NoError(err)
	require.Equal(accountA, testAccount)
	err = ws.DelState(hashA)
	require.NoError(err)
	hash3 := ws.Digest()
	require.NotEqual(hash2, hash3)
	err = ws.State(hashA, &testAccount)
	require.Error(err)
}

func TestFetDB(t *testing.T) {
	require := require.New(t)
	memDB := db.NewMemKVStore()
	ws := newStateTX(0, memDB, []protocol.ActionHandler{account.NewProtocol()})
	require.Equal(uint64(0), ws.Version())
	ws.dao.Start(context.Background())
	require.Equal(memDB, ws.GetDB())
}
