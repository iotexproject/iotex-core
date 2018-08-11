// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/testutil"
)

const testTriePath = "trie.test"

func TestEmptyTrie(t *testing.T) {
	require := require.New(t)

	tr, err := NewTrie(db.NewMemKVStore(), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	require.Equal(tr.RootHash(), EmptyRoot)
	require.Nil(tr.Stop(context.Background()))
}

func Test2Roots(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	// first trie
	tr, err := NewTrie(db.NewBoltDB(testTriePath, nil), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	require.Nil(tr.Upsert(cat, testV[2]))
	v, err := tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], v)
	require.Nil(tr.Upsert(car, testV[1]))
	v, err = tr.Get(car)
	require.Nil(err)
	require.Equal(testV[1], v)
	require.Nil(tr.Upsert(egg, testV[4]))
	v, err = tr.Get(egg)
	require.Nil(err)
	require.Equal(testV[4], v)
	root := tr.RootHash()
	require.Nil(tr.Stop(context.Background()))

	// second trie
	tr1, err := NewTrie(tr.TrieDB(), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr1.Start(context.Background()))
	require.Nil(tr1.Upsert(dog, testV[3]))
	v, err = tr1.Get(dog)
	require.Nil(err)
	require.Equal(testV[3], v)
	require.Nil(tr1.Upsert(ham, testV[0]))
	v, err = tr1.Get(ham)
	require.Nil(err)
	require.Equal(testV[0], v)
	require.Nil(tr1.Upsert(fox, testV[5]))
	v, err = tr1.Get(fox)
	require.Nil(err)
	require.Equal(testV[5], v)
	root1 := tr1.RootHash()
	require.NotEqual(root, root1)
	require.Nil(tr1.Stop(context.Background()))

	// start first trie again
	require.Nil(tr.Start(context.Background()))
	v, err = tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], v)
	v, err = tr.Get(car)
	require.Nil(err)
	require.Equal(testV[1], v)
	v, err = tr.Get(egg)
	require.Nil(err)
	require.Equal(testV[4], v)
	// does not contain dog
	v, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))

	// re-create second trie from its root
	tr2, err := NewTrie(tr.TrieDB(), "test", root1)
	require.Nil(err)
	require.Nil(tr2.Start(context.Background()))
	v, err = tr2.Get(dog)
	require.Nil(err)
	require.Equal(testV[3], v)
	v, err = tr2.Get(ham)
	require.Nil(err)
	require.Equal(testV[0], v)
	v, err = tr2.Get(fox)
	require.Nil(err)
	require.Equal(testV[5], v)
	// does not contain cat
	v, err = tr2.Get(cat)
	require.Equal(ErrNotExist, errors.Cause(err))
	require.Nil(tr.Stop(context.Background()))
	require.Nil(tr2.Stop(context.Background()))
}

func TestInsert(t *testing.T) {
	require := require.New(t)

	tr, err := newTrie(db.NewMemKVStore(), "", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	root := EmptyRoot
	require.Equal(uint64(1), tr.numBranch)
	// query non-existing entry
	ptr, match, err := tr.query(cat)
	require.NotNil(ptr)
	require.Equal(0, match)
	require.NotNil(err)
	tr.clear()
	// this splits the root B
	logger.Info().Msg("Put[cat]")
	err = tr.Upsert(cat, testV[2])
	require.Nil(err)
	catRoot := tr.RootHash()
	require.NotEqual(catRoot, root)
	root = catRoot
	require.Equal(uint64(1), tr.numLeaf)
	b, err := tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], b)

	// this splits L --> E + B + 2L
	logger.Info().Msg("Put[rat]")
	err = tr.Upsert(rat, []byte("rat"))
	require.Nil(err)
	ratRoot := tr.RootHash()
	require.NotEqual(ratRoot, root)
	root = ratRoot
	require.Equal(uint64(2), tr.numBranch)
	require.Equal(uint64(1), tr.numExt)
	require.Equal(uint64(3), tr.numLeaf)
	b, err = tr.Get(rat)
	require.Nil(err)
	require.Equal([]byte("rat"), b)

	// this adds another L to B
	logger.Info().Msg("Put[car]")
	err = tr.Upsert(car, testV[1])
	require.Nil(err)
	newRoot := tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(car)
	require.Nil(err)
	require.Equal(testV[1], b)
	// delete car
	logger.Info().Msg("Del[car]")
	err = tr.Delete(car)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	require.Equal(newRoot, ratRoot)
	root = newRoot

	// this splits E (with match = 3, div = 3)
	logger.Info().Msg("Put[dog]")
	err = tr.Upsert(dog, testV[3])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	// Get returns "dog" now
	b, err = tr.Get(dog)
	require.Nil(err)
	require.Equal(testV[3], b)
	logger.Info().Msg("Del[dog]")
	err = tr.Delete(dog)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// this splits E (with match = 0, div = 2)
	logger.Info().Msg("Put[egg]")
	err = tr.Upsert(egg, testV[4])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.Nil(err)
	require.Equal(testV[4], b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	require.Nil(err)
	eggRoot := tr.RootHash()
	require.NotEqual(eggRoot, root)
	root = eggRoot

	// this splits E (with match = 4, div = 1)
	logger.Info().Msg("Put[egg]")
	err = tr.Upsert(egg, testV[4])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.Nil(err)
	require.Equal(testV[4], b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	require.Equal(newRoot, eggRoot)
	root = newRoot

	// insert 'ham' 'fox' 'cow'
	logger.Info().Msg("Put[ham]")
	err = tr.Upsert(ham, testV[0])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ham)
	require.Nil(err)
	require.Equal(testV[0], b)
	logger.Info().Msg("Put[fox]")
	err = tr.Upsert(fox, testV[5])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(fox)
	require.Nil(err)
	require.Equal(testV[5], b)
	logger.Info().Msg("Put[cow]")
	err = tr.Upsert(cow, testV[6])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(cow)
	require.Nil(err)
	require.Equal(testV[6], b)

	// delete fox ham cow
	logger.Info().Msg("Del[fox]")
	err = tr.Delete(fox)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	logger.Info().Msg("Del[ham]")
	err = tr.Delete(ham)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	logger.Info().Msg("Del[cow]")
	err = tr.Delete(cow)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// this adds another path to root B
	logger.Info().Msg("Put[ant]")
	err = tr.Upsert(ant, testV[7])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ant)
	require.Nil(err)
	require.Equal(testV[7], b)
	logger.Info().Msg("Del[ant]")
	err = tr.Delete(ant)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// delete "rat"
	logger.Info().Msg("Del[rat]")
	err = tr.Delete(rat)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	b, err = tr.Get(rat)
	require.NotNil(err)
	require.Equal([]byte(nil), b)
	// delete "cat"
	logger.Info().Msg("Del[cat]")
	err = tr.Delete(cat)
	require.Nil(err)
	require.Equal(EmptyRoot, tr.RootHash())
	require.Equal(uint64(1), tr.numEntry)
	require.Nil(tr.Stop(context.Background()))
}

func TestBatchCommit(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	tr, err := NewTrie(db.NewBoltDB(testTriePath, nil), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	// insert 3 entries
	require.Nil(tr.Upsert(cat, testV[2]))
	require.Nil(tr.Upsert(car, testV[1]))
	require.Nil(tr.Upsert(egg, testV[4]))
	root := tr.RootHash()
	// start batch mode
	tr.EnableBatch()
	require.Nil(tr.Upsert(dog, testV[3]))
	v, _ := tr.Get(dog)
	require.Equal(testV[3], v)
	require.Nil(tr.Upsert(ham, testV[0]))
	v, _ = tr.Get(ham)
	require.Equal(testV[0], v)
	require.Nil(tr.Upsert(fox, testV[5]))
	v, _ = tr.Get(fox)
	require.Equal(testV[5], v)
	require.NotEqual(root, tr.RootHash())
	// close w/o commit and reopen
	require.Nil(tr.Stop(context.Background()))
	tr, err = NewTrie(db.NewBoltDB(testTriePath, nil), "test", root)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	v, _ = tr.Get(cat)
	require.Equal(testV[2], v)
	v, _ = tr.Get(car)
	require.Equal(testV[1], v)
	v, _ = tr.Get(egg)
	require.Equal(testV[4], v)
	// entries not committed won't exist
	_, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))

	// start batch mode
	tr.EnableBatch()
	require.Nil(tr.Upsert(dog, testV[3]))
	require.Nil(tr.Upsert(ham, testV[0]))
	require.Nil(tr.Upsert(fox, testV[5]))
	root = tr.RootHash()
	// commit and reopen
	require.Nil(tr.Commit())
	require.Nil(tr.Stop(context.Background()))
	tr, err = NewTrie(db.NewBoltDB(testTriePath, nil), "test", root)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	// all entries should exist now
	v, _ = tr.Get(cat)
	require.Equal(testV[2], v)
	v, _ = tr.Get(car)
	require.Equal(testV[1], v)
	v, _ = tr.Get(egg)
	require.Equal(testV[4], v)
	v, _ = tr.Get(dog)
	require.Equal(testV[3], v)
	v, _ = tr.Get(ham)
	require.Equal(testV[0], v)
	v, _ = tr.Get(fox)
	require.Equal(testV[5], v)
	require.Nil(tr.Stop(context.Background()))
}

func Test2kEntries(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	tr, err := NewTrie(db.NewBoltDB(testTriePath, nil), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	tr.EnableBatch()
	root := EmptyRoot
	seed := time.Now().Nanosecond()
	// insert 1k entries
	var k [32]byte
	k[0] = byte(seed)
	for i := 0; i < 1<<11; i++ {
		k = blake2b.Sum256(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:8]); err == nil {
			continue
		}
		logger.Info().Hex("key", k[:8]).Msg("Put --")
		require.Nil(tr.Upsert(k[:8], v))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:8])
		require.Nil(err)
		require.Equal(v, b)
		// update <k, v>
		v = testV[7-k[0]&7]
		require.Nil(tr.Upsert(k[:8], v))
		b, err = tr.Get(k[:8])
		require.Nil(err)
		require.Equal(v, b)
	}
	// delete 1k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := blake2b.Sum256(d[:])
	d2 := blake2b.Sum256(d1[:])
	d3 := blake2b.Sum256(d2[:])
	d = d3
	for i := 0; i < 1<<11-3; i++ {
		d = blake2b.Sum256(d[:])
		logger.Info().Hex("key", d[:8]).Msg("Del --")
		require.Nil(tr.Delete(d[:8]))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:8])
		require.Equal(ErrNotExist, errors.Cause(err))
	}
	require.Nil(tr.Delete(d1[:8]))
	require.Nil(tr.Delete(d2[:8]))
	require.Nil(tr.Delete(d3[:8]))
	require.Nil(tr.Commit())
	// trie should fallback to empty
	require.Equal(EmptyRoot, tr.RootHash())
	require.Nil(tr.Stop(context.Background()))
}

func TestPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestPressure in short mode.")
	}

	require := require.New(t)

	tr, err := NewTrie(db.NewMemKVStore(), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	root := EmptyRoot
	seed := time.Now().Nanosecond()
	// insert 64k entries
	var k [32]byte
	k[0] = byte(seed)
	for i := 0; i < 1<<16; i++ {
		k = blake2b.Sum256(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:8]); err == nil {
			continue
		}
		logger.Info().Hex("key", k[:8]).Msg("Put --")
		require.Nil(tr.Upsert(k[:8], v))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:8])
		require.Nil(err)
		require.Equal(v, b)
	}
	// delete 64k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := blake2b.Sum256(d[:])
	d2 := blake2b.Sum256(d1[:])
	d3 := blake2b.Sum256(d2[:])
	d = d3
	for i := 0; i < 1<<16-3; i++ {
		d = blake2b.Sum256(d[:])
		logger.Info().Hex("key", d[:8]).Msg("Del --")
		require.Nil(tr.Delete(d[:8]))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:8])
		require.Equal(ErrNotExist, errors.Cause(err))
	}
	require.Nil(tr.Delete(d1[:8]))
	require.Nil(tr.Delete(d2[:8]))
	require.Nil(tr.Delete(d3[:8]))
	// trie should fallback to empty
	require.Equal(EmptyRoot, tr.RootHash())
	require.Nil(tr.Stop(context.Background()))
}

func TestQuery(t *testing.T) {
	require := require.New(t)

	tr, err := newTrie(db.NewMemKVStore(), "", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	require.Equal(uint64(1), tr.numBranch)
	// key length > 0
	ptr, match, err := tr.query(cat)
	require.NotNil(ptr)
	require.Equal(0, match)
	require.NotNil(err)
	// key length == 0
	ptr, match, err = tr.query([]byte{})
	require.Equal(tr.root, ptr)
	require.Equal(0, match)
	require.Nil(err)
	require.Nil(tr.Stop(context.Background()))
}
