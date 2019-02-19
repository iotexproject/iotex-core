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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const testStateDBPath = "stateDB.test"

func TestSDBCandidates(t *testing.T) {
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
	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	sdb.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(t, sdb.Start(context.Background()))
	ws, err := sdb.NewWorkingSet()
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, b, big.NewInt(200))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, c, big.NewInt(300))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, d, big.NewInt(100))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, e, big.NewInt(100))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, f, big.NewInt(300))
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
	_, _, err = ws.RunActions(ctx, 0, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	balanceA, err := sdb.Balance(a)
	require.NoError(t, err)
	require.Equal(t, balanceA, big.NewInt(70))
	balanceB, err := sdb.Balance(b)
	require.NoError(t, err)
	require.Equal(t, balanceB, big.NewInt(210))
	balanceC, err := sdb.Balance(c)
	require.NoError(t, err)
	require.Equal(t, balanceC, big.NewInt(320))

	//test Nonce()
	nonceA, err := sdb.Nonce(a)
	require.NoError(t, err)
	require.Equal(t, nonceA, uint64(2))
	nonceB, err := sdb.Nonce(b)
	require.NoError(t, err)
	require.Equal(t, nonceB, uint64(0))

	h, _ := sdb.Height()
	cand, _ := sdb.CandidatesByHeight(h)
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

	// Test running an action in zero gas limit context.
	_, _, err = ws.RunActions(zctx, 0, []action.SealedEnvelope{selp})
	require.Error(t, err)
	_, err = ws.RunAction(ctx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
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
	_ = ws.UpdateBlockLevelInfo(1)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":70", b + ":210"}))
	// a(a):70(+0=70) b(b):210(+0=210) !c:320

	// test RootHash() & RootHashByHeight()
	rootHash, err := sdb.RootHashByHeight(h)
	require.NoError(t, err)
	require.Equal(t, hash.ZeroHash256, rootHash, sdb.RootHash())

	ws, err = sdb.NewWorkingSet()
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
	_ = ws.UpdateBlockLevelInfo(2)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sdb.NewWorkingSet()
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
	_ = ws.UpdateBlockLevelInfo(3)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):90(0) b(b):190(+90=280) !c:320

	ws, err = sdb.NewWorkingSet()
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
	_ = ws.UpdateBlockLevelInfo(4)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote4, err := action.NewVote(1, a, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote4).SetNonce(1).
		SetDestinationAddress(a).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 5, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":210", b + ":70"}))
	// a(b):70(210) b(a):210(70) !c:320

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote5, err := action.NewVote(2, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote5).SetNonce(2).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 6, []action.SealedEnvelope{selp})
	require.NoError(t, err)

	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote6, err := action.NewVote(3, b, uint64(20000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote6).SetNonce(3).
		SetDestinationAddress(b).SetGasLimit(20000).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 7, []action.SealedEnvelope{selp})
	require.NoError(t, err)

	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":280"}))
	// a(b):70(0) b(b):210(+70=280) !c:320

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	tx5, err := action.NewTransfer(uint64(2), big.NewInt(20), a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx5).SetNonce(2).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 8, []action.SealedEnvelope{selp})
	require.NoError(t, err)

	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":0", b + ":300"}))
	// a(b):90(0) b(b):210(+90=300) !c:300

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote7, err := action.NewVote(0, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote7).SetNonce(0).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 9, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":300"}))
	// a(b):90(300) b(b):210(+90=300) !c(a):300

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote8, err := action.NewVote(4, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote8).SetNonce(4).
		SetDestinationAddress(c).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 10, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{a + ":300", b + ":90"}))
	// a(b):90(300) b(c):210(90) !c(a):300

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote9, err := action.NewVote(1, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote9).SetNonce(1).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 11, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote10, err := action.NewVote(0, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote10).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 12, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", b + ":90"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote11, err := action.NewVote(1, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote11).SetNonce(1).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 13, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", d + ":100"}))
	// a(b):90(0) b(c):210(90) c(c):300(+210=510) d(d): 100(100)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote12, err := action.NewVote(2, a, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote12).SetNonce(2).
		SetDestinationAddress(a).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyD)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 14, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote13, err := action.NewVote(2, d, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote13).SetNonce(2).
		SetDestinationAddress(d).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 15, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":210", d + ":300"}))
	// a(b):90(100) b(c):210(90) c(d):300(210) d(a): 100(300)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote14, err := action.NewVote(3, c, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote14).SetNonce(3).
		SetDestinationAddress(c).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 16, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":510", a + ":100"}))
	// a(b):90(100) b(c):210(90) c(c):300(+210=510) d(a): 100(0)

	ws, err = sdb.NewWorkingSet()
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

	_, _, err = ws.RunActions(ctx, 17, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", a + ":100"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) !e:500

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote15, err := action.NewVote(0, e, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote15).SetNonce(0).
		SetDestinationAddress(e).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyE)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 18, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":110", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote16, err := action.NewVote(0, f, uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote16).SetNonce(0).
		SetDestinationAddress(f).SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 19, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{f + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(0) e(e):500(+0=500) f(f):300(+0=300)

	ws, err = sdb.NewWorkingSet()
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
	_ = ws.UpdateBlockLevelInfo(20)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{d + ":300", e + ":500"}))
	// a(b):90(100) b(c):10(90) c(c):100(+10=110) d(a): 100(300) e(e):500(+0=500) f(d):300(0)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	tx8, err := action.NewTransfer(uint64(1), big.NewInt(200), b, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx8).SetNonce(1).
		SetDestinationAddress(b).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 21, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":310", e + ":500"}))
	// a(b):90(100) b(c):210(90) c(c):100(+210=310) d(a): 100(100) e(e):500(+0=500) f(d):100(0)
	//fmt.Printf("%v \n", voteForm(sdb.candidatesBuffer()))

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	tx9, err := action.NewTransfer(uint64(1), big.NewInt(10), a, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx9).SetNonce(1).
		SetDestinationAddress(a).Build()
	selp, err = action.Sign(elp, priKeyB)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 22, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", e + ":500"}))
	// a(b):100(100) b(c):200(100) c(c):100(+200=300) d(a): 100(100) e(e):500(+0=500) f(d):100(0)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	tx10, err := action.NewTransfer(uint64(1), big.NewInt(300), d, nil, uint64(0), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(tx10).SetNonce(1).
		SetDestinationAddress(d).Build()
	selp, err = action.Sign(elp, priKeyE)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 23, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, err = sdb.Height()
	require.Equal(t, uint64(23), h)
	require.NoError(t, err)
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", a + ":400"}))
	// a(b):100(400) b(c):200(100) c(c):100(+200=300) d(a): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sdb.NewWorkingSet()
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

	_, _, err = ws.RunActions(ctx, 24, []action.SealedEnvelope{selp1, selp2})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, err = sdb.Height()
	require.Equal(t, uint64(24), h)
	require.NoError(t, err)
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{c + ":300", b + ":500"}))
	// a(b):100(0) b(c):200(500) c(c):100(+200=300) d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote21, err := action.NewVote(4, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote21).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyC)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 25, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	require.Equal(t, uint64(25), h)
	require.NoError(t, err)
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)

	ws, err = sdb.NewWorkingSet()
	require.NoError(t, err)
	vote22, err := action.NewVote(4, "", uint64(100000), big.NewInt(0))
	require.NoError(t, err)

	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote22).SetNonce(4).
		SetDestinationAddress("").SetGasLimit(100000).Build()
	selp, err = action.Sign(elp, priKeyF)
	require.NoError(t, err)

	_, _, err = ws.RunActions(ctx, 26, []action.SealedEnvelope{selp})
	require.NoError(t, err)
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	require.Equal(t, uint64(26), h)
	require.NoError(t, err)
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{e + ":200", b + ":500"}))
	// a(b):100(0) b(c):200(500) [c(c):100(+200=300)] d(b): 400(100) e(e):200(+0=200) f(d):100(0)
	stateA, err := util.LoadOrCreateAccount(ws, a, big.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, stateA.Balance, big.NewInt(100))
}

func TestSDBUnvote(t *testing.T) {
	// Create three dummy iotex addresses
	a := testaddress.Addrinfo["alfa"].String()
	priKeyA := testaddress.Keyinfo["alfa"].PriKey
	b := testaddress.Addrinfo["bravo"].String()
	priKeyB := testaddress.Keyinfo["bravo"].PriKey

	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)

	cfg := config.Default
	cfg.Chain.NumCandidates = 2
	cfg.Chain.TrieDBPath = testStateDBPath
	db, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	sdb, ok := db.(*stateDB)
	require.True(t, ok)
	sdb.AddActionHandlers(vote.NewProtocol(nil))
	require.NoError(t, sdb.Start(context.Background()))

	ws, err := sdb.NewWorkingSet()
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)
	_, err = util.LoadOrCreateAccount(ws, b, big.NewInt(200))
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
	require.NoError(t, sdb.Commit(ws))
	h, _ := sdb.Height()
	cand, _ := sdb.CandidatesByHeight(h)
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
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
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
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
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
	require.NoError(t, sdb.Commit(ws))
	h, _ = sdb.Height()
	cand, _ = sdb.CandidatesByHeight(h)
	require.True(t, compareStrings(voteForm(h, cand), []string{b + ":200"}))
}

func TestSDBState(t *testing.T) {
	// Create three dummy iotex addresses
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
	require.NoError(db.Start(context.Background()))

	sdb := db.(*stateDB)

	require.NoError(sdb.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)))
	height, err := sdb.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	require.NoError(sdb.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(10)))
	height, err = sdb.Height()
	require.NoError(err)
	require.Equal(uint64(10), height)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testStateDBPath

	testutil.CleanupPath(t, testStateDBPath)
	defer testutil.CleanupPath(t, testStateDBPath)
	db, err := NewStateDB(cfg, InMemStateDBOption())
	require.NoError(err)
	require.NoError(db.Start(context.Background()))

	sdb := db.(*stateDB)

	require.NoError(sdb.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)))
	height, err := sdb.Height()
	require.NoError(err)
	require.Equal(uint64(0), height)

	require.NoError(sdb.dao.Put(AccountKVNameSpace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(10)))
	height, err = sdb.Height()
	require.NoError(err)
	require.Equal(uint64(10), height)
}

func BenchmarkSDBInMemRunAction(b *testing.B) {
	benchSDBRunAction(true, config.Config{}, b)
}

func BenchmarkSDBRunAction(b *testing.B) {
	tp := filepath.Join(os.TempDir(), testStateDBPath)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
	cfg := config.Default
	cfg.Chain.TrieDBPath = tp
	benchSDBRunAction(false, cfg, b)
	if fileutil.FileExists(tp) && os.RemoveAll(tp) != nil {
		b.Error("Fail to remove testDB file")
	}
}

func benchSDBRunAction(isInMem bool, cfg config.Config, b *testing.B) {
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
	var sdb Factory
	var err error
	if isInMem {
		sdb, err = NewStateDB(cfg, InMemStateDBOption())
		if err != nil {
			b.Fatal(err)
		}
	} else {
		sdb, err = NewStateDB(cfg, DefaultStateDBOption())
		if err != nil {
			b.Fatal(err)
		}
	}
	sdb.AddActionHandlers(account.NewProtocol())
	if err := sdb.Start(context.Background()); err != nil {
		b.Fatal(err)
	}
	ws, err := sdb.NewWorkingSet()
	if err != nil {
		b.Fatal(err)
	}
	for _, acc := range accounts {
		_, err = util.LoadOrCreateAccount(ws, acc, big.NewInt(int64(b.N*100)))
		if err != nil {
			b.Fatal(err)
		}
	}
	if err := sdb.Commit(ws); err != nil {
		b.Fatal(err)
	}
	gasLimit := testutil.TestGasLimit
	for n := 0; n < b.N; n++ {
		ws, err := sdb.NewWorkingSet()
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
			nonces[senderIdx]++
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
		ctx := protocol.WithRunActionsCtx(context.Background(),
			protocol.RunActionsCtx{
				Producer: testaddress.Addrinfo["producer"],
				GasLimit: &gasLimit,
			})
		_, _, err = ws.RunActions(ctx, uint64(n), acts)
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		if err := sdb.Commit(ws); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
