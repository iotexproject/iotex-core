// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const testStateDBPath = "stateDB.test"

func TestSDBCandidates(t *testing.T) {
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)
	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testCandidates(sdb, t, false)
}

func TestSDBUnvote(t *testing.T) {
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)
	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testUnvote(sdb, t)
}

func TestSDBState(t *testing.T) {
	// Create a dummy iotex address
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	sdb.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(t, sdb.Start(context.Background()))
	defer func() {
		require.NoError(t, sdb.Stop(context.Background()))
	}()
	ws, err := sdb.NewWorkingSet()
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)

	// a:100(0)

	vote, err := action.NewVote(0, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(vote).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sdb.Commit(ws))
	h, _ := sdb.Height()
	cand, _ := sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":100"}))
	// a(a):100(+0=100) b:200 c:300

	//test AccountState() & State()
	var testAccount state.Account
	accountA, err := sdb.AccountState(a)
	require.NoError(t, err)
	sHash := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
	err = sdb.State(sHash, &testAccount)
	require.NoError(t, err)
	require.Equal(t, accountA, &testAccount)
	require.Equal(t, big.NewInt(100), accountA.Balance)
	require.True(t, accountA.IsCandidate)
	require.Equal(t, a, accountA.Votee)
	require.Equal(t, big.NewInt(100), accountA.VotingWeight)
}

func TestSDBLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	db, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	testLoadStoreHeight(db, t)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath

	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)
	db, err := NewStateDB(cfg, InMemStateDBOption())
	require.NoError(err)

	testLoadStoreHeight(db, t)
}

func TestNonce(t *testing.T) {
	// Create two dummy iotex address
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	sdb.AddActionHandlers(account.NewProtocol(), account.NewProtocol())
	require.NoError(t, sdb.Start(context.Background()))
	defer func() {
		require.NoError(t, sdb.Stop(context.Background()))
	}()
	ws, err := sdb.NewWorkingSet()
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)

	tx, err := action.NewTransfer(0, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tx).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	nonce, err := sdb.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, uint64(0), nonce)

	tx, err = action.NewTransfer(1, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx).SetNonce(1).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sdb.Commit(ws))
	nonce, err = sdb.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), nonce)

}

func BenchmarkSDBInMemRunAction(b *testing.B) {
	cfg := config.Default
	sdb, err := NewStateDB(cfg, InMemStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
}

func BenchmarkSDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), testStateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sdb, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}
