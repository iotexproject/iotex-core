// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const testTriePath = "trie.test"
const testStateDBPath = "stateDB.test"

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func voteForm(height uint64, cs []*state.Candidate) []string {
	r := make([]string, len(cs))
	for i := 0; i < len(cs); i++ {
		r[i] = cs[i].Address + ":" + strconv.FormatInt(cs[i].Votes.Int64(), 10)
	}
	return r
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	defer func() {
		require.NoError(sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	testSnapshot(ws, t)
}

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

func testSnapshot(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	addr := testaddress.Addrinfo["alfa"].String()
	_, err := accountutil.LoadOrCreateAccount(ws, addr, big.NewInt(5))
	require.NoError(err)
	sHash := hash.BytesToHash160(testaddress.Addrinfo["alfa"].Bytes())

	s, err := accountutil.LoadAccount(ws, sHash)
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
	_, err = accountutil.LoadOrCreateAccount(ws, addr, big.NewInt(7))
	require.NoError(err)
	tHash := hash.BytesToHash160(testaddress.Addrinfo["bravo"].Bytes())

	s, err = accountutil.LoadAccount(ws, tHash)
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

func TestCandidates(t *testing.T) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(t, err)
	testCandidates(sf, t)
}

func TestSDBCandidates(t *testing.T) {
	cfg := config.Default
	sdb, err := NewStateDB(cfg, InMemStateDBOption())
	require.NoError(t, err)
	testCandidates(sdb, t)
}

func testCandidates(sf Factory, t *testing.T) {
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, candidatesutil.LoadAndAddCandidates(ws, 1, identityset.Address(0).String()))
	require.NoError(t, candidatesutil.LoadAndUpdateCandidates(ws, 1, identityset.Address(0).String(), big.NewInt(0)))
	require.NoError(t, candidatesutil.LoadAndAddCandidates(ws, 1, identityset.Address(1).String()))
	require.NoError(t, candidatesutil.LoadAndUpdateCandidates(ws, 1, identityset.Address(1).String(), big.NewInt(1)))
	require.NoError(t, sf.Commit(ws))

	candidates, err := sf.CandidatesByHeight(1)
	require.NoError(t, err)
	require.Equal(t, 2, len(candidates))
	assert.Equal(t, candidates[0].Address, identityset.Address(1).String())
	assert.Equal(t, candidates[0].Votes, big.NewInt(1))
	assert.Equal(t, candidates[1].Address, identityset.Address(0).String())
	assert.Equal(t, candidates[1].Votes, big.NewInt(0))
}

func TestState(t *testing.T) {
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testState(sf, t)
}

func TestSDBState(t *testing.T) {
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testState(sdb, t)
}

func testState(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(t, sf.Start(context.Background()))
	defer func() {
		require.NoError(t, sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)

	// a:100(0)

	vote, err := action.NewVote(0, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(vote).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	gasLimit := uint64(1000000)
	raCtx := protocol.RunActionsCtx{
		Producer: testaddress.Addrinfo["producer"],
		GasLimit: gasLimit,
	}

	_, err = ws.RunAction(raCtx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.Equal(t, voteForm(h, cand), []string{a + ":100"})
	// a(a):100(+0=100) b:200 c:300

	//test AccountState() & State()
	var testAccount state.Account
	accountA, err := sf.AccountState(a)
	require.NoError(t, err)
	sHash := hash.BytesToHash160(testaddress.Addrinfo["alfa"].Bytes())
	err = sf.State(sHash, &testAccount)
	require.NoError(t, err)
	require.Equal(t, accountA, &testAccount)
	require.Equal(t, big.NewInt(100), accountA.Balance)
	require.True(t, accountA.IsCandidate)
	require.Equal(t, a, accountA.Votee)
	require.Equal(t, big.NewInt(100), accountA.VotingWeight)
}

func TestNonce(t *testing.T) {
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testNonce(sf, t)
}
func TestSDBNonce(t *testing.T) {
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)

	testNonce(sdb, t)
}

func testNonce(sf Factory, t *testing.T) {
	// Create two dummy iotex address
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()

	sf.AddActionHandlers(account.NewProtocol(), account.NewProtocol())
	require.NoError(t, sf.Start(context.Background()))
	defer func() {
		require.NoError(t, sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)

	tx, err := action.NewTransfer(0, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tx).SetNonce(0).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	gasLimit := uint64(1000000)
	raCtx := protocol.RunActionsCtx{
		Producer: testaddress.Addrinfo["producer"],
		GasLimit: gasLimit,
	}

	_, err = ws.RunAction(raCtx, selp)
	require.NoError(t, err)
	nonce, err := sf.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, uint64(0), nonce)

	tx, err = action.NewTransfer(1, big.NewInt(2), b, nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx).SetNonce(1).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunAction(raCtx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sf.Commit(ws))
	nonce, err = sf.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), nonce)
}

func TestUnvote(t *testing.T) {
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	f, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testUnvote(f, t)
}

func TestSDBUnvote(t *testing.T) {
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testUnvote(sdb, t)
}

func testUnvote(sf Factory, t *testing.T) {
	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey

	sf.AddActionHandlers(vote.NewProtocol(nil))
	require.NoError(t, sf.Start(context.Background()))
	defer func() {
		require.NoError(t, sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(t, err)

	vote1, err := action.NewVote(0, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(vote1).SetNonce(0).SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	gasLimit := uint64(10000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.Equal(t, voteForm(h, cand), []string{})

	vote2, err := action.NewVote(0, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote2).SetNonce(0).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.Equal(t, voteForm(h, cand), []string{a + ":100"})

	vote3, err := action.NewVote(0, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote3).SetNonce(0).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.Equal(t, voteForm(h, cand), []string{})

	vote4, err := action.NewVote(0, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(0).SetGasLimit(20000).Build()
	selp1, err := action.Sign(elp, priKeyB)
	require.NoError(t, err)

	vote5, err := action.NewVote(0, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(0).SetGasLimit(20000).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	vote6, err := action.NewVote(0, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(0).SetGasLimit(20000).Build()
	selp3, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2, selp3})
	require.Nil(t, err)
	require.Nil(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.Equal(t, voteForm(h, cand), []string{b + ":200"})
}

func TestLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, DefaultTrieOption())
	require.NoError(err)

	testLoadStoreHeight(statefactory, t)
}

func TestLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	statefactory, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(err)
	testLoadStoreHeight(statefactory, t)
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

func testLoadStoreHeight(sf Factory, t *testing.T) {
	require := require.New(t)
	require.NoError(sf.Start(context.Background()))
	defer func() {
		require.NoError(sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	dao := ws.GetDB()
	require.NoError(dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)))
	height, err := sf.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	require.NoError(dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(10)))
	height, err = sf.Height()
	require.NoError(err)
	require.Equal(uint64(10), height)
}

func TestFactory_RootHashByHeight(t *testing.T) {
	cfg := config.Default
	ctx := context.Background()
	sf, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	defer func() {
		require.NoError(t, sf.Stop(ctx))
	}()

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = ws.RunActions(context.Background(), 1, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	rootHash, err := sf.RootHashByHeight(1)
	require.NoError(t, err)
	require.NotEqual(t, hash.ZeroHash256, rootHash)
}

func TestRunActions(t *testing.T) {
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(t, err)
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	testRunActions(ws, t)
}

func TestSTXRunActions(t *testing.T) {
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	testRunActions(ws, t)
}

func testRunActions(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.GetDB().Start(context.Background()))
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey
	_, err := accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(err)

	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), b, nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	tx2, err := action.NewTransfer(uint64(1), big.NewInt(20), a, nil, uint64(0), big.NewInt(0))
	require.NoError(err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyB)
	require.NoError(err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{selp1, selp2})
	require.NoError(err)
	rootHash1 := ws.UpdateBlockLevelInfo(1)
	require.NoError(ws.Commit())

	rootHash2 := ws.RootHash()
	require.Equal(rootHash1, rootHash2)
	h := ws.Height()
	require.Equal(uint64(1), h)
}

func TestCachedBatch(t *testing.T) {
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(t, err)
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	testCachedBatch(ws, t, false)
}

func TestSTXCachedBatch(t *testing.T) {
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	testCachedBatch(ws, t, true)
}

func testCachedBatch(ws WorkingSet, t *testing.T, chechCachedBatchHash bool) {
	require := require.New(t)
	hash1 := ws.Digest()
	if chechCachedBatchHash {
		require.NotEqual(hash.ZeroHash256, hash1)
	}

	// test PutState()
	hashA := hash.BytesToHash160(testaddress.Addrinfo["alfa"].Bytes())
	accountA := state.EmptyAccount()
	accountA.Balance = big.NewInt(70)
	accountA.VotingWeight = big.NewInt(70)
	err := ws.PutState(hashA, accountA)
	require.NoError(err)
	hash2 := ws.Digest()
	if chechCachedBatchHash {
		require.NotEqual(hash1, hash2)
	}

	// test State()
	testAccount := state.EmptyAccount()
	err = ws.State(hashA, &testAccount)
	require.NoError(err)
	require.Equal(accountA, testAccount)

	// test DelState()
	err = ws.DelState(hashA)
	require.NoError(err)
	hash3 := ws.Digest()
	if chechCachedBatchHash {
		require.NotEqual(hash2, hash3)
	}

	// can't state account "alfa" anymore
	err = ws.State(hashA, &testAccount)
	require.Error(err)
}

func TestGetDB(t *testing.T) {
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(t, err)
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	testGetDB(ws, t)
}

func TestSTXGetDB(t *testing.T) {
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol()})
	testGetDB(ws, t)
}

func testGetDB(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	memDB := db.NewMemKVStore()
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.GetDB().Start(context.Background()))
	require.Equal(memDB, ws.GetDB())
}

func BenchmarkInMemRunAction(b *testing.B) {
	cfg := config.Default
	sf, err := NewFactory(cfg, InMemTrieOption())
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sf, b)
}

func BenchmarkDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), testTriePath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}

	cfg := config.Default
	cfg.DB.DbPath = tp
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	if err != nil {
		b.Fatal(err)
	}
	benchRunAction(sf, b)

	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
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

func benchRunAction(sf Factory, b *testing.B) {
	// set up
	accounts := []string{
		testaddress.Addrinfo["alfa"].String(),
		testaddress.Addrinfo["bravo"].String(),
		testaddress.Addrinfo["charlie"].String(),
		testaddress.Addrinfo["delta"].String(),
		testaddress.Addrinfo["echo"].String(),
		testaddress.Addrinfo["foxtrot"].String(),
	}
	pubKeys := []keypair.PublicKey{
		testaddress.Keyinfo["alfa"].PubKey,
		testaddress.Keyinfo["bravo"].PubKey,
		testaddress.Keyinfo["charlie"].PubKey,
		testaddress.Keyinfo["delta"].PubKey,
		testaddress.Keyinfo["echo"].PubKey,
		testaddress.Keyinfo["foxtrot"].PubKey,
	}
	nonces := make([]uint64, len(accounts))

	sf.AddActionHandlers(account.NewProtocol())
	if err := sf.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	defer func() {
		defer func() {
			if err := sf.Stop(context.Background()); err != nil {
				b.Fatal(err)
			}

		}()
	}()

	ws, err := sf.NewWorkingSet()
	if err != nil {
		b.Fatal(err)
	}
	for _, acc := range accounts {
		_, err = accountutil.LoadOrCreateAccount(ws, acc, big.NewInt(int64(b.N*100)))
		if err != nil {
			b.Fatal(err)
		}
	}
	if err := sf.Commit(ws); err != nil {
		b.Fatal(err)
	}
	gasLimit := testutil.TestGasLimit * 100000

	for n := 0; n < b.N; n++ {
		ws, err := sf.NewWorkingSet()
		if err != nil {
			b.Fatal(err)
		}

		// put 500 actions together to run
		b.StopTimer()
		total := 500
		acts := make([]action.SealedEnvelope, 0, total)
		for numActs := 0; numActs < total; numActs++ {
			senderIdx := rand.Int() % len(accounts)

			var chainIDBytes [4]byte
			enc.MachineEndian.PutUint32(chainIDBytes[:], 1)
			payload := []byte(randStringRunes(20))
			receiverAddr, err := address.FromBytes(payload)
			if err != nil {
				b.Fatal(err)
			}
			receiver := receiverAddr.String()
			nonces[senderIdx] += nonces[senderIdx]
			tx, err := action.NewTransfer(nonces[senderIdx], big.NewInt(1), receiver, nil, uint64(0), big.NewInt(0))
			if err != nil {
				b.Fatal(err)
			}
			bd := &action.EnvelopeBuilder{}
			elp := bd.SetNonce(nonces[senderIdx]).SetAction(tx).Build()
			selp := action.FakeSeal(elp, pubKeys[senderIdx])
			acts = append(acts, selp)
		}
		b.StartTimer()
		zctx := protocol.WithRunActionsCtx(context.Background(),
			protocol.RunActionsCtx{
				Producer: testaddress.Addrinfo["producer"],
				GasLimit: gasLimit,
			})
		_, err = ws.RunActions(zctx, uint64(n), acts)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := sf.Commit(ws); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
