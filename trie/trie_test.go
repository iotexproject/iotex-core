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

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/testutil"
)

var cfg = config.Default.DB

const testTriePath = "trie.test"

func TestEmptyTrie(t *testing.T) {
	require := require.New(t)

	_, err := NewTrie(nil, "test", EmptyRoot)
	require.Equal(ErrInvalidTrie, errors.Cause(err))

	_, err = NewTrieSharedBatch(db.NewMemKVStore(), nil, "test", EmptyRoot)
	require.Equal(ErrInvalidTrie, errors.Cause(err))

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
	tr, err := NewTrie(db.NewBoltDB(testTriePath, cfg), "test", EmptyRoot)
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
	require.Nil(tr.Commit())
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
	require.Nil(tr1.Commit())
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
	_, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))

	// create a new one and load second trie's root
	tr2, err := NewTrie(tr.TrieDB(), "test", EmptyRoot)
	require.NoError(err)
	require.NoError(tr2.Start(context.Background()))
	require.NoError(tr2.SetRoot(root1))
	require.Equal(root1, tr2.RootHash())
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
	_, err = tr2.Get(cat)
	require.Equal(ErrNotExist, errors.Cause(err))
	require.Nil(tr.Stop(context.Background()))
	require.Nil(tr2.Stop(context.Background()))
}

func TestInsert(t *testing.T) {
	require := require.New(t)

	tr, err := NewTrie(db.NewMemKVStore(), "", EmptyRoot)
	require.NotNil(tr)
	require.NoError(err)
	require.Nil(tr.Start(context.Background()))
	root := EmptyRoot
	// this adds one L to root R
	logger.Info().Msg("Put[cat]")
	err = tr.Upsert(cat, testV[2])
	require.Nil(err)
	catRoot := tr.RootHash()
	require.NotEqual(catRoot, root)
	root = catRoot

	// this splits L --> E + B + 2L (cat, rat)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat)
	 */
	logger.Info().Msg("Put[rat]")
	err = tr.Upsert(rat, []byte("rat"))
	require.Nil(err)
	ratRoot := tr.RootHash()
	require.NotEqual(ratRoot, root)
	require.NoError(tr.Commit())
	b, err := tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.Nil(err)
	require.Equal([]byte("rat"), b)

	// it's OK to upsert a key with same value again
	require.Nil(tr.Upsert(rat, []byte("rat")))
	require.Nil(tr.Upsert(cat, testV[2]))
	require.NoError(tr.Commit())
	b, err = tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.Nil(err)
	require.Equal([]byte("rat"), b)
	root = tr.RootHash()
	// root should keep the same since the value is same
	require.Equal(root, ratRoot)

	// this adds another L to B (car)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat, car)
	 */
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

	// this splits E --> B1 + E1 + L (match = 3, div = 3)
	/*
	 *  Root --1--> E --234--> B1 --5--> E1 --67--> B --> (cat, rat)
	 *                          | --6--> L --710--> dog
	 */
	logger.Info().Msg("Put[dog]")
	err = tr.Upsert(dog, testV[3])
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	require.NoError(tr.Commit())
	// Get returns "dog" now
	b, err = tr.Get(dog)
	require.Nil(err)
	require.Equal(testV[3], b)
	b, err = tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.Nil(err)
	require.Equal([]byte("rat"), b)
	_, err = tr.Get(car)
	require.Equal(ErrNotExist, errors.Cause(err))

	// this deletes 'dog' and turns B1 into another E2
	/*
	 *  Root --1--> E --234--> E2 --5--> E1 --67--> B --> (cat, rat)
	 */
	logger.Info().Msg("Del[dog]")
	err = tr.Delete(dog)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// this splits E1 --> B2 + E3 + L (match = 0, div = 2)
	/*
	 *  Root --1--> E --234--> E2 --5--> B2 --6--> E3 --7--> B --> (cat, rat)
	 *                                    | --8--> L --10--> egg
	 */
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
	require.NoError(tr.Commit())
	_, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(egg)
	require.Equal(ErrNotExist, errors.Cause(err))

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

	// delete fox rat cow
	logger.Info().Msg("Del[fox]")
	err = tr.Delete(fox)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	logger.Info().Msg("Del[rat]")
	err = tr.Delete(rat)
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
	require.NoError(tr.Commit())
	_, err = tr.Get(fox)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(rat)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(cow)
	require.Equal(ErrNotExist, errors.Cause(err))
	b, err = tr.Get(ham)
	require.Nil(err)
	require.Equal(testV[0], b)

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

	// delete "ham"
	logger.Info().Msg("Del[ham]")
	err = tr.Delete(ham)
	require.Nil(err)
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	require.NoError(tr.Commit())
	_, err = tr.Get(ham)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ant)
	require.Equal(ErrNotExist, errors.Cause(err))
	b, err = tr.Get(cat)
	require.Nil(err)
	require.Equal(testV[2], b)

	// delete "cat"
	logger.Info().Msg("Del[cat]")
	err = tr.Delete(cat)
	require.Nil(err)
	require.Equal(EmptyRoot, tr.RootHash())
	require.Nil(tr.Stop(context.Background()))
}

func TestBatchCommit(t *testing.T) {
	require := require.New(t)
	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)

	tr, err := NewTrie(db.NewBoltDB(testTriePath, cfg), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	// insert 3 entries
	require.Nil(tr.Upsert(cat, testV[2]))
	require.Nil(tr.Upsert(car, testV[1]))
	require.Nil(tr.Upsert(egg, testV[4]))
	require.Nil(tr.Commit())
	c, _ := tr.Get(cat)
	require.Equal(testV[2], c)
	root := tr.RootHash()
	// insert another 3 entries
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
	tr, err = NewTrie(db.NewBoltDB(testTriePath, cfg), "test", root)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	// entries committed exist
	v, _ = tr.Get(cat)
	require.Equal(testV[2], v)
	v, _ = tr.Get(car)
	require.Equal(testV[1], v)
	v, _ = tr.Get(egg)
	require.Equal(testV[4], v)
	// entries not committed won't exist
	_, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ham)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(fox)
	require.Equal(ErrNotExist, errors.Cause(err))

	// insert 3 entries again
	require.Nil(tr.Upsert(dog, testV[3]))
	require.Nil(tr.Upsert(ham, testV[0]))
	require.Nil(tr.Upsert(fox, testV[6]))
	v, _ = tr.Get(fox)
	require.Equal(testV[6], v)
	require.Nil(tr.Upsert(fox, testV[5]))
	v, _ = tr.Get(fox)
	require.Equal(testV[5], v)
	root = tr.RootHash()
	// commit and reopen
	require.Nil(tr.Commit())
	require.Nil(tr.Stop(context.Background()))
	tr, err = NewTrie(db.NewBoltDB(testTriePath, cfg), "test", root)
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

func TestCollision(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	tr, err := NewTrie(db.NewBoltDB(testTriePath, cfg), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	defer require.Nil(tr.Stop(context.Background()))

	// this creates 2 leaf (with same value) on branch '0' and '1'
	require.NoError(tr.Upsert(br1, testV[0]))
	require.NoError(tr.Upsert(br2, testV[0]))
	// this adds another 2 leaf (with same value), which splits the first 2 leaf
	require.NoError(tr.Upsert(cl1, testV[0]))
	require.NoError(tr.Upsert(cl2, testV[0]))
	v, _ := tr.Get(br1)
	require.Equal(testV[0], v)
	v, _ = tr.Get(br2)
	require.Equal(testV[0], v)
	v, _ = tr.Get(cl1)
	require.Equal(testV[0], v)
	v, _ = tr.Get(cl2)
	require.Equal(testV[0], v)
}

func Test4kEntries(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	tr, err := NewTrie(db.NewBoltDB(testTriePath, cfg), "test", EmptyRoot)
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	root := EmptyRoot
	seed := time.Now().Nanosecond()
	// insert 4k entries
	var k [32]byte
	k[0] = byte(seed)
	c := 0
	for c = 0; c < 1<<12; c++ {
		k = blake2b.Sum256(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:4]); err == nil {
			logger.Warn().Hex("k", k[:4]).Msg("collision")
			break
		}
		require.Nil(tr.Upsert(k[:4], v))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:4])
		require.Nil(err)
		require.Equal(v, b)
		// update <k, v>
		v = testV[7-k[0]&7]
		require.Nil(tr.Upsert(k[:4], v))
		b, err = tr.Get(k[:4])
		require.Nil(err)
		require.Equal(v, b)
		if c%64 == 0 {
			logger.Info().Hex("key", k[:4]).Msg("Put --")
			require.NoError(tr.Commit())
		}
	}
	// delete 4k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := blake2b.Sum256(d[:])
	d2 := blake2b.Sum256(d1[:])
	d3 := blake2b.Sum256(d2[:])
	d = d3
	for i := 0; i < c-3; i++ {
		d = blake2b.Sum256(d[:])
		require.Nil(tr.Delete(d[:4]))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:4])
		require.Equal(ErrNotExist, errors.Cause(err))
		if i%64 == 0 {
			logger.Info().Hex("key", d[:4]).Msg("Del --")
			require.NoError(tr.Commit())
		}
	}
	require.Nil(tr.Delete(d1[:4]))
	require.Nil(tr.Delete(d2[:4]))
	require.Nil(tr.Delete(d3[:4]))
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
	// insert 128k entries
	var k [32]byte
	k[0] = byte(seed)
	c := 0
	for c = 0; c < 1<<17; c++ {
		k = blake2b.Sum256(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:4]); err == nil {
			logger.Warn().Hex("k", k[:4]).Msg("collision")
			break
		}
		require.Nil(tr.Upsert(k[:4], v))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:4])
		require.Nil(err)
		require.Equal(v, b)
		if c%(2<<10) == 0 {
			logger.Info().Hex("key", k[:4]).Msg("Put --")
			require.NoError(tr.Commit())
		}
	}
	// delete 128k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := blake2b.Sum256(d[:])
	d2 := blake2b.Sum256(d1[:])
	d3 := blake2b.Sum256(d2[:])
	d = d3
	for i := 0; i < c-3; i++ {
		d = blake2b.Sum256(d[:])
		require.Nil(tr.Delete(d[:4]))
		newRoot := tr.RootHash()
		require.NotEqual(newRoot, EmptyRoot)
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:4])
		require.Equal(ErrNotExist, errors.Cause(err))
		if i%(2<<10) == 0 {
			logger.Info().Hex("key", d[:4]).Msg("Del --")
			require.NoError(tr.Commit())
		}
	}
	require.Nil(tr.Delete(d1[:4]))
	require.Nil(tr.Delete(d2[:4]))
	require.Nil(tr.Delete(d3[:4]))
	require.Nil(tr.Commit())
	// trie should fallback to empty
	require.Equal(EmptyRoot, tr.RootHash())
	require.Nil(tr.Stop(context.Background()))
	logger.Warn().Int("entries", c).Msg("test")
}
