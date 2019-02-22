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

func compareStrings(actual []string, expected []string) bool {
	act := make(map[string]bool)
	for i := 0; i < len(actual); i++ {
		act[actual[i]] = true
	}

	for i := 0; i < len(expected); i++ {
		if _, ok := act[expected[i]]; ok {
			delete(act, expected[i])
		} else {
			return false
		}
	}
	return len(act) == 0
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
	sHash := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())

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
	tHash := byteutil.BytesTo20B(testaddress.Addrinfo["bravo"].Bytes())

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
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testCandidates(sf, t, true)
}

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

func testCandidates(sf Factory, t *testing.T, checkStateRoot bool) {

	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey
	c := testaddress.Addrinfo["charlie"].String()
	priKeyC := testaddress.Keyinfo["charlie"].PriKey
	d := testaddress.Addrinfo["delta"].String()
	priKeyD := testaddress.Keyinfo["delta"].PriKey
	e := testaddress.Addrinfo["echo"].String()
	priKeyE := testaddress.Keyinfo["echo"].PriKey
	f := testaddress.Addrinfo["foxtrot"].String()
	priKeyF := testaddress.Keyinfo["foxtrot"].PriKey

	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
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
	_, err = accountutil.LoadOrCreateAccount(ws, c, big.NewInt(300))
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, d, big.NewInt(100))
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, e, big.NewInt(100))
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, f, big.NewInt(300))
	require.NoError(t, err)
	// a:100(0) b:200(0) c:300(0)
	tx1, err := action.NewTransfer(uint64(1), big.NewInt(10), b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetDestinationAddress(b).SetAction(tx1).Build()
	selp1, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	tx2, err := action.NewTransfer(uint64(2), big.NewInt(20), c, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetNonce(2).SetDestinationAddress(c).SetAction(tx2).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	gasLimit := uint64(1000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})
	newRoot, _, err := ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	balanceB, err := sf.Balance(b)
	require.NoError(t, err)
	require.Equal(t, balanceB, big.NewInt(210))
	balanceC, err := sf.Balance(c)
	require.NoError(t, err)
	require.Equal(t, balanceC, big.NewInt(320))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))
	// a:70 b:210 c:320

	vote, err := action.NewVote(0, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	zeroGasLimit := uint64(0)
	zctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &zeroGasLimit,
		})
	_, _, err = ws.RunActions(zctx, 0, []action.SealedEnvelope{selp})
	require.NotNil(t, err)
	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(0)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":70"}))
	// a(a):70(+0=70) b:210 c:320

	vote2, err := action.NewVote(0, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote2).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)
	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(1)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":70", b + ":210"}))
	// a(a):70(+0=70) b(b):210(+0=210) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote3, err := action.NewVote(1, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote3).SetNonce(1).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(2)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx3, err := action.NewTransfer(uint64(2), big.NewInt(20), a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx3).SetNonce(2).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(3)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):90(0) b(b):190(+90=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx4, err := action.NewTransfer(uint64(2), big.NewInt(20), b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx4).SetNonce(2).
		SetDestinationAddress(b).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(4)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote4, err := action.NewVote(1, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(1).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 5, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":210", b + ":70"}))
	// a(b):70(210) b(a):210(70) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote5, err := action.NewVote(2, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(2).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 6, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote6, err := action.NewVote(3, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(3).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 7, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx5, err := action.NewTransfer(uint64(2), big.NewInt(20), a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx5).SetNonce(2).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 8, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":300"}))
	// a(b):90(0) b(b):210(+90=300) !c:300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote7, err := action.NewVote(0, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote7).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 9, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":300"}))
	// a(b):90(300) b(b):210(+90=300) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote8, err := action.NewVote(4, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote8).SetNonce(4).
		SetDestinationAddress(c).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 10, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":90"}))
	// a(b):90(300) b(c):210(90) !c(a):300

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote9, err := action.NewVote(1, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote9).SetNonce(1).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 11, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote10, err := action.NewVote(0, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote10).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 12, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote11, err := action.NewVote(1, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote11).SetNonce(1).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 13, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", d + ":100"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510) d(d): 100(100)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote12, err := action.NewVote(2, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote12).SetNonce(2).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 14, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote13, err := action.NewVote(2, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote13).SetNonce(2).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 15, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":210", d + ":300"}))
	// a(b):90(100) b(c):210(90) c(d):300(210) d(a): 100(300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote14, err := action.NewVote(3, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote14).SetNonce(3).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 16, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx6, err := action.NewTransfer(uint64(1), big.NewInt(200), e, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx6).SetNonce(1).
		SetDestinationAddress(e).Build()
	selp1, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	tx7, err := action.NewTransfer(uint64(2), big.NewInt(200), e, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx7).SetNonce(2).
		SetDestinationAddress(e).Build()
	selp2, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 17, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", a + ":100"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) !e:500

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote15, err := action.NewVote(0, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote15).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyE)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 18, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote16, err := action.NewVote(0, f, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote16).SetNonce(0).
		SetDestinationAddress(f).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 19, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{f + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500) f(f):300(+0=300)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote17, err := action.NewVote(0, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote17).SetNonce(0).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp1, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	vote18, err := action.NewVote(1, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote18).SetNonce(1).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp2, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	_, err = ws.RunAction(ctx, selp1)
	require.NoError(t, err)
	_, err = ws.RunAction(ctx, selp2)
	require.NoError(t, err)
	newRoot = ws.UpdateBlockLevelInfo(20)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{d + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(300) e(e):500(+0=500) f(d):300(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx8, err := action.NewTransfer(uint64(1), big.NewInt(200), b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx8).SetNonce(1).
		SetDestinationAddress(b).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 21, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":310", e + ":500"}))
	// a(b):90(100) b(c):210(90) c(c):100(+210=310) d(a): 100(100) e(e):500(+0=500) f(d):100(0)
	//fmt.Printf("%v \n", voteForm(sf.candidatesBuffer()))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx9, err := action.NewTransfer(uint64(1), big.NewInt(10), a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx9).SetNonce(1).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 22, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", e + ":500"}))
	// a(b):100(100) b(c):200(100) c(c):100(+200=300) d(a): 100(100) e(e):500(+0=500) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	tx10, err := action.NewTransfer(uint64(1), big.NewInt(300), d, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx10).SetNonce(1).
		SetDestinationAddress(d).Build()
	selp, err = action.Sign(elp, priKeyE)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 23, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(23), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", a + ":400"}))
	// a(b):100(400) b(c):200(100) c(c):100(+200=300) d(a): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote19, err := action.NewVote(0, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote19).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp1, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	vote20, err := action.NewVote(3, b, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote20).SetNonce(3).
		SetDestinationAddress(b).SetGasLimit(100000).Build()
	selp2, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 24, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	require.NoError(t, sf.Commit(ws))
	h, err = sf.Height()
	require.Equal(t, uint64(24), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", b + ":500"}))
	// a(b):100(0) b(c):200(500) c(c):100(+200=300) d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote21, err := action.NewVote(4, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote21).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 25, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, hash.ZeroHash256, newRoot)
	}
	root := newRoot
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(25), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)
	vote22, err := action.NewVote(4, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote22).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	newRoot, _, err = ws.RunActions(ctx, 26, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	if checkStateRoot {
		require.NotEqual(t, newRoot, root)
	}
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	require.Equal(t, uint64(26), h)
	require.NoError(t, err)
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)
	stateA, err := accountutil.LoadOrCreateAccount(ws, a, big.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, stateA.Balance, big.NewInt(100))
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
	cfg.Chain.NumCandidates = 2
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
	require.NoError(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":100"}))
	// a(a):100(+0=100) b:200 c:300

	//test AccountState() & State()
	var testAccount state.Account
	accountA, err := sf.AccountState(a)
	require.NoError(t, err)
	sHash := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
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
	cfg.Chain.NumCandidates = 2
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
	nonce, err := sf.Nonce(a)
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
	require.NoError(t, sf.Commit(ws))
	nonce, err = sf.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), nonce)
}

func TestUnvote(t *testing.T) {
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.DB.DbPath = testTriePath
	f, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testUnvote(f, t)
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
	elp := bd.SetAction(vote1).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	gasLimit := uint64(10000000)
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: &gasLimit,
		})
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))
	h, _ := sf.Height()
	cand, _ := sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote2, err := action.NewVote(0, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote2).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":100"}))

	vote3, err := action.NewVote(0, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote3).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{}))

	vote4, err := action.NewVote(0, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(0).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp1, err := action.Sign(elp, priKeyB)
	require.NoError(t, err)

	vote5, err := action.NewVote(0, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(0).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp2, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	vote6, err := action.NewVote(0, "", uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(0).
		SetDestinationAddress("").SetGasLimit(20000).Build()
	selp3, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2, selp3})
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))
	h, _ = sf.Height()
	cand, _ = sf.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{b + ":200"}))
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
	_, _, err = ws.RunActions(context.Background(), 1, nil)
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
	hashA := byteutil.BytesTo20B(testaddress.Addrinfo["alfa"].Bytes())
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
			elp := bd.SetNonce(nonces[senderIdx]).SetDestinationAddress(receiver).SetAction(tx).Build()
			selp := action.FakeSeal(elp, pubKeys[senderIdx])
			acts = append(acts, selp)
		}
		b.StartTimer()
		zctx := protocol.WithRunActionsCtx(context.Background(),
			protocol.RunActionsCtx{
				Producer: testaddress.Addrinfo["producer"],
				GasLimit: &gasLimit,
			})
		_, _, err = ws.RunActions(zctx, uint64(n), acts)
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
