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
	ws, err := sdb.NewWorkingSet()
	require.NoError(err)
	testSnapshot(ws, t)
}

func TestRunActions(t *testing.T) {
	require := require.New(t)
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.dao.Start(context.Background()))
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
	rootHash1 := ws.UpdateBlockLevelInfo(1)
	require.NoError(ws.Commit())

	rootHash2 := ws.RootHash()
	require.Equal(hash.ZeroHash256, rootHash1, rootHash2)
	h := ws.Height()
	require.Equal(uint64(1), h)
}

func TestCachedBatch(t *testing.T) {
	require := require.New(t)
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})

	hash1 := ws.Digest()
	require.NotEqual(hash.ZeroHash256, hash1)

	// test PutState()
	hashA := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
	accountA := state.EmptyAccount()
	accountA.Balance = big.NewInt(70)
	accountA.VotingWeight = big.NewInt(70)
	err := ws.PutState(hashA, accountA)
	require.NoError(err)
	hash2 := ws.Digest()
	require.NotEqual(hash1, hash2)

	// test State()
	testAccount := state.EmptyAccount()
	err = ws.State(hashA, &testAccount)
	require.NoError(err)
	require.Equal(accountA, testAccount)

	// test DelState()
	err = ws.DelState(hashA)
	require.NoError(err)
	hash3 := ws.Digest()
	require.NotEqual(hash2, hash3)

	// can't state account "alfa" anymore
	err = ws.State(hashA, &testAccount)
	require.Error(err)
}

func TestGetDB(t *testing.T) {
	require := require.New(t)
	memDB := db.NewMemKVStore()
	ws := newStateTX(0, memDB, []protocol.ActionHandler{account.NewProtocol()})
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.dao.Start(context.Background()))
	require.Equal(memDB, ws.GetDB())
}
