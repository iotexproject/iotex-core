// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	triePath    = "trie.test"
	stateDBPath = "stateDB.test"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestSnapshot(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

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
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testStateDBPath := testTrieFile.Name()

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
	addr := identityset.Address(28).String()
	_, err := accountutil.LoadOrCreateAccount(ws, addr, big.NewInt(5))
	require.NoError(err)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())

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
	addr = identityset.Address(29).String()
	_, err = accountutil.LoadOrCreateAccount(ws, addr, big.NewInt(7))
	require.NoError(err)
	tHash := hash.BytesToHash160(identityset.Address(29).Bytes())

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
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testState(sf, t)
}

func TestSDBState(t *testing.T) {
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)
	testState(sdb, t)
}

func testState(sf Factory, t *testing.T) {
	// Create a dummy iotex address
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	sf.AddActionHandlers(account.NewProtocol(0))
	require.NoError(t, sf.Start(context.Background()))
	defer func() {
		require.NoError(t, sf.Stop(context.Background()))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100))
	require.NoError(t, err)

	tsf, err := action.NewTransfer(1, big.NewInt(10), identityset.Address(31).String(), nil, uint64(20000), big.NewInt(0))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(20000).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(t, err)
	gasLimit := uint64(1000000)
	raCtx := protocol.RunActionsCtx{
		Producer: identityset.Address(27),
		GasLimit: gasLimit,
	}

	_, err = ws.RunAction(raCtx, selp)
	require.NoError(t, err)
	_ = ws.UpdateBlockLevelInfo(0)
	require.NoError(t, sf.Commit(ws))

	//test AccountState() & State()
	var testAccount state.Account
	accountA, err := sf.AccountState(a)
	require.NoError(t, err)
	sHash := hash.BytesToHash160(identityset.Address(28).Bytes())
	err = sf.State(sHash, &testAccount)
	require.NoError(t, err)
	require.Equal(t, accountA, &testAccount)
	require.Equal(t, big.NewInt(90), accountA.Balance)
}

func TestNonce(t *testing.T) {
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.DB.DbPath = testTriePath
	sf, err := NewFactory(cfg, PrecreatedTrieDBOption(db.NewOnDiskDB(cfg.DB)))
	require.NoError(t, err)
	testNonce(sf, t)
}
func TestSDBNonce(t *testing.T) {
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	sdb, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(t, err)

	testNonce(sdb, t)
}

func testNonce(sf Factory, t *testing.T) {
	// Create two dummy iotex address
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()

	sf.AddActionHandlers(account.NewProtocol(0), account.NewProtocol(0))
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
		Producer: identityset.Address(27),
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

func TestLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, DefaultTrieOption())
	require.NoError(err)

	testLoadStoreHeight(statefactory, t)
}

func TestLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	statefactory, err := NewFactory(cfg, InMemTrieOption())
	require.NoError(err)
	testLoadStoreHeight(statefactory, t)
}

func TestSDBLoadStoreHeight(t *testing.T) {
	require := require.New(t)
	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
	db, err := NewStateDB(cfg, DefaultStateDBOption())
	require.NoError(err)

	testLoadStoreHeight(db, t)
}

func TestSDBLoadStoreHeightInMem(t *testing.T) {
	require := require.New(t)

	testDBFile, _ := ioutil.TempFile(os.TempDir(), stateDBPath)
	testDBPath := testDBFile.Name()
	cfg := config.Default
	cfg.Chain.TrieDBPath = testDBPath
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
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol(0)})
	testRunActions(ws, t)
}

func testRunActions(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.GetDB().Start(context.Background()))
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()
	priKeyB := identityset.PrivateKey(29)
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
			Producer: identityset.Address(27),
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
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol(0)})
	testCachedBatch(ws, t, true)
}

func testCachedBatch(ws WorkingSet, t *testing.T, chechCachedBatchHash bool) {
	require := require.New(t)
	hash1 := ws.Digest()
	if chechCachedBatchHash {
		require.NotEqual(hash.ZeroHash256, hash1)
	}

	// test PutState()
	hashA := hash.BytesToHash160(identityset.Address(28).Bytes())
	accountA := state.EmptyAccount()
	accountA.Balance = big.NewInt(70)
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
	ws := newStateTX(0, db.NewMemKVStore(), []protocol.ActionHandler{account.NewProtocol(0)})
	testGetDB(ws, t)
}

func testGetDB(ws WorkingSet, t *testing.T) {
	require := require.New(t)
	memDB := db.NewMemKVStore()
	require.Equal(uint64(0), ws.Version())
	require.NoError(ws.GetDB().Start(context.Background()))
	require.Equal(memDB, ws.GetDB())
}

func TestDeleteAndPutSameKey(t *testing.T) {
	testDeleteAndPutSameKey := func(t *testing.T, ws WorkingSet) {
		key := hash.Hash160b([]byte("test"))
		acc := state.Account{
			Nonce: 1,
		}
		require.NoError(t, ws.PutState(key, acc))
		require.NoError(t, ws.DelState(key))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(ws.State(key, &acc)))
		require.Equal(t, state.ErrStateNotExist, errors.Cause(ws.State(hash.Hash160b([]byte("other")), &acc)))
	}
	t.Run("workingSet", func(t *testing.T) {
		sf, err := NewFactory(config.Default, InMemTrieOption())
		require.NoError(t, err)
		ws, err := sf.NewWorkingSet()
		require.NoError(t, err)
		testDeleteAndPutSameKey(t, ws)
	})
	t.Run("stateTx", func(t *testing.T) {
		ws := newStateTX(0, db.NewMemKVStore(), nil)
		testDeleteAndPutSameKey(t, ws)
	})
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
	tp := filepath.Join(os.TempDir(), triePath)
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
	tp := filepath.Join(os.TempDir(), stateDBPath)
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
		identityset.Address(28).String(),
		identityset.Address(29).String(),
		identityset.Address(30).String(),
		identityset.Address(31).String(),
		identityset.Address(32).String(),
		identityset.Address(33).String(),
	}
	pubKeys := []crypto.PublicKey{
		identityset.PrivateKey(28).PublicKey(),
		identityset.PrivateKey(29).PublicKey(),
		identityset.PrivateKey(30).PublicKey(),
		identityset.PrivateKey(31).PublicKey(),
		identityset.PrivateKey(32).PublicKey(),
		identityset.PrivateKey(33).PublicKey(),
	}
	nonces := make([]uint64, len(accounts))

	sf.AddActionHandlers(account.NewProtocol(0))
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
				Producer: identityset.Address(27),
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
