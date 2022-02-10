// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	ham = []byte{1, 2, 3, 4, 2, 3, 4, 5}
	car = []byte{1, 2, 3, 4, 5, 6, 7, 7}
	cat = []byte{1, 2, 3, 4, 5, 6, 7, 8}
	rat = []byte{1, 2, 3, 4, 5, 6, 7, 9}
	egg = []byte{1, 2, 3, 4, 5, 8, 1, 0}
	dog = []byte{1, 2, 3, 4, 6, 7, 1, 0}
	fox = []byte{1, 2, 3, 5, 6, 7, 8, 9}
	cow = []byte{1, 2, 5, 6, 7, 8, 9, 0}
	ant = []byte{2, 3, 4, 5, 6, 7, 8, 9}

	br1 = []byte{0, 3, 4, 5, 6, 7, 8, 9}
	br2 = []byte{1, 3, 4, 5, 6, 7, 8, 9}
	cl1 = []byte{0, 0, 4, 5, 6, 7, 8, 9}
	cl2 = []byte{1, 0, 4, 5, 6, 7, 8, 9}

	testV = [8][]byte{
		[]byte("ham"), []byte("car"), []byte("cat"), []byte("dog"),
		[]byte("egg"), []byte("fox"), []byte("cow"), []byte("ant"),
	}

	emptyTrieRootHash = []byte{0x61, 0x8e, 0x1c, 0xe1, 0xff, 0xfb, 0x18, 0x25, 0x15, 0x9f, 0x9a, 0xa2, 0xc2, 0xe9, 0x37, 0x24, 0x2, 0xfb, 0xd0, 0xab}
)

func TestEmptyTrie(t *testing.T) {
	require := require.New(t)
	tr, err := New()
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer require.NoError(tr.Stop(context.Background()))
	require.True(tr.IsEmpty())
	rootHash, err := tr.RootHash()
	require.NoError(err)
	require.Equal(emptyTrieRootHash, rootHash)
}

func Test2Roots(t *testing.T) {
	require := require.New(t)

	// first trie
	trieDB := trie.NewMemKVStore()
	tr, err := New(KVStoreOption(trieDB), KeyLengthOption(8), AsyncOption())
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	require.NoError(tr.Upsert(cat, testV[2]))
	v, err := tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], v)
	require.NoError(tr.Upsert(car, testV[1]))
	v, err = tr.Get(car)
	require.NoError(err)
	require.Equal(testV[1], v)
	require.NoError(tr.Upsert(egg, testV[4]))
	v, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], v)
	root, err := tr.RootHash()
	require.NoError(err)
	require.NoError(tr.Stop(context.Background()))

	// second trie
	tr1, err := New(KVStoreOption(trieDB), KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr1.Start(context.Background()))
	require.NoError(tr1.Upsert(dog, testV[3]))
	v, err = tr1.Get(dog)
	require.NoError(err)
	require.Equal(testV[3], v)
	require.NoError(tr1.Upsert(ham, testV[0]))
	v, err = tr1.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], v)
	require.NoError(tr1.Upsert(fox, testV[5]))
	v, err = tr1.Get(fox)
	require.NoError(err)
	require.Equal(testV[5], v)
	root1, err := tr1.RootHash()
	require.NoError(err)
	require.NotEqual(root, root1)
	require.NoError(tr1.Stop(context.Background()))

	// start first trie again
	require.NoError(tr.Start(context.Background()))
	v, err = tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], v)
	v, err = tr.Get(car)
	require.NoError(err)
	require.Equal(testV[1], v)
	v, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], v)
	// does not contain dog
	_, err = tr.Get(dog)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	// create a new one and load second trie's root
	tr2, err := New(KVStoreOption(trieDB), KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr2.Start(context.Background()))
	require.NoError(tr2.SetRootHash(root1))
	root2, err := tr2.RootHash()
	require.NoError(err)
	require.Equal(root1, root2)
	v, err = tr2.Get(dog)
	require.NoError(err)
	require.Equal(testV[3], v)
	v, err = tr2.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], v)
	v, err = tr2.Get(fox)
	require.NoError(err)
	require.Equal(testV[5], v)
	// does not contain cat
	_, err = tr2.Get(cat)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	require.NoError(tr.Stop(context.Background()))
	require.NoError(tr2.Stop(context.Background()))
}

func TestInsert(t *testing.T) {
	require := require.New(t)

	tr, err := New(KVStoreOption(trie.NewMemKVStore()), KeyLengthOption(8))
	require.NotNil(tr)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	// this adds one L to root R
	t.Log("Put[cat]")
	require.NoError(tr.Upsert(cat, testV[2]))
	require.False(tr.IsEmpty())
	root, err := tr.RootHash()
	require.NoError(err)

	// this splits L --> E + B + 2L (cat, rat)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat)
	 */
	t.Log("Put[rat]")
	require.NoError(tr.Upsert(rat, []byte("rat")))
	ratRoot, err := tr.RootHash()
	require.NoError(err)
	require.NotEqual(ratRoot, root)
	b, err := tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.NoError(err)
	require.Equal([]byte("rat"), b)

	// it's OK to upsert a key with same value again
	require.NoError(tr.Upsert(rat, []byte("rat")))
	require.NoError(tr.Upsert(cat, testV[2]))
	b, err = tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.NoError(err)
	require.Equal([]byte("rat"), b)
	root, err = tr.RootHash()
	require.NoError(err)
	// root should keep the same since the value is same
	require.Equal(root, ratRoot)

	// this adds another L to B (car)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat, car)
	 */
	t.Log("Put[car]")
	require.NoError(tr.Upsert(car, testV[1]))
	newRoot, err := tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(car)
	require.NoError(err)
	require.Equal(testV[1], b)
	// delete car
	t.Log("Del[car]")
	require.NoError(tr.Delete(car))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	require.Equal(newRoot, ratRoot)
	root = newRoot

	// this splits E --> B1 + E1 + L (match = 3, div = 3)
	/*
	 *  Root --1--> E --234--> B1 --5--> E1 --67--> B --> (cat, rat)
	 *                          | --6--> L --710--> dog
	 */
	t.Log("Put[dog]")
	require.NoError(tr.Upsert(dog, testV[3]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	// Get returns "dog" now
	b, err = tr.Get(dog)
	require.NoError(err)
	require.Equal(testV[3], b)
	b, err = tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], b)
	b, err = tr.Get(rat)
	require.NoError(err)
	require.Equal([]byte("rat"), b)
	_, err = tr.Get(car)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	// this deletes 'dog' and turns B1 into another E2
	/*
	 *  Root --1--> E --234--> E2 --5--> E1 --67--> B --> (cat, rat)
	 */
	t.Log("Del[dog]")
	require.NoError(tr.Delete(dog))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot

	// this splits E1 --> B2 + E3 + L (match = 0, div = 2)
	/*
	 *  Root --1--> E --234--> E2 --5--> B2 --6--> E3 --7--> B --> (cat, rat)
	 *                                    | --8--> L --10--> egg
	 */
	t.Log("Put[egg]")
	require.NoError(tr.Upsert(egg, testV[4]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], b)
	// delete egg
	t.Log("Del[egg]")
	require.NoError(tr.Delete(egg))
	eggRoot, err := tr.RootHash()
	require.NoError(err)
	require.NotEqual(eggRoot, root)
	root = eggRoot

	// this splits E (with match = 4, div = 1)
	t.Log("Put[egg]")
	require.NoError(tr.Upsert(egg, testV[4]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], b)
	// delete egg
	t.Log("Del[egg]")
	require.NoError(tr.Delete(egg))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	require.Equal(newRoot, eggRoot)
	root = newRoot
	_, err = tr.Get(dog)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(egg)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	// insert 'ham' 'fox' 'cow'
	t.Log("Put[ham]")
	require.NoError(tr.Upsert(ham, testV[0]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], b)
	t.Log("Put[fox]")
	require.NoError(tr.Upsert(fox, testV[5]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(fox)
	require.NoError(err)
	require.Equal(testV[5], b)
	t.Log("Put[cow]")
	require.NoError(tr.Upsert(cow, testV[6]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(cow)
	require.NoError(err)
	require.Equal(testV[6], b)

	// delete fox rat cow
	t.Log("Del[fox]")
	require.NoError(tr.Delete(fox))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	t.Log("Del[rat]")
	require.NoError(tr.Delete(rat))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	t.Log("Del[cow]")
	require.NoError(tr.Delete(cow))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	_, err = tr.Get(fox)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(rat)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(cow)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	b, err = tr.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], b)

	// this adds another path to root B
	t.Log("Put[ant]")
	require.NoError(tr.Upsert(ant, testV[7]))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ant)
	require.NoError(err)
	require.Equal(testV[7], b)
	t.Log("Del[ant]")
	require.NoError(tr.Delete(ant))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	root = newRoot

	// delete "ham"
	t.Log("Del[ham]")
	require.NoError(tr.Delete(ham))
	newRoot, err = tr.RootHash()
	require.NoError(err)
	require.NotEqual(newRoot, root)
	_, err = tr.Get(ham)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ant)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	b, err = tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], b)

	// delete "cat"
	t.Log("Del[cat]")
	require.NoError(tr.Delete(cat))
	require.True(tr.IsEmpty())
	require.NoError(tr.Stop(context.Background()))
}

func TestBatchCommit(t *testing.T) {
	require := require.New(t)

	trieDB := trie.NewMemKVStore()
	tr, err := New(KeyLengthOption(8), KVStoreOption(trieDB))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	// insert 3 entries
	require.NoError(tr.Upsert(cat, testV[2]))
	require.NoError(tr.Upsert(car, testV[1]))
	require.NoError(tr.Upsert(egg, testV[4]))
	c, _ := tr.Get(cat)
	require.Equal(testV[2], c)
	// entries committed exist
	v, _ := tr.Get(cat)
	require.Equal(testV[2], v)
	v, _ = tr.Get(car)
	require.Equal(testV[1], v)
	v, _ = tr.Get(egg)
	require.Equal(testV[4], v)
	// entries not committed won't exist
	_, err = tr.Get(dog)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ham)
	require.Equal(trie.ErrNotExist, errors.Cause(err))
	_, err = tr.Get(fox)
	require.Equal(trie.ErrNotExist, errors.Cause(err))

	// insert 3 entries again
	require.NoError(tr.Upsert(dog, testV[3]))
	require.NoError(tr.Upsert(ham, testV[0]))
	require.NoError(tr.Upsert(fox, testV[6]))
	v, _ = tr.Get(fox)
	require.Equal(testV[6], v)
	require.NoError(tr.Upsert(fox, testV[5]))
	v, _ = tr.Get(fox)
	require.Equal(testV[5], v)
	root, err := tr.RootHash()
	require.NoError(err)
	// commit and reopen
	require.NoError(tr.Stop(context.Background()))
	tr, err = New(KVStoreOption(trieDB), RootHashOption(root), KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
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
	require.NoError(tr.Stop(context.Background()))
}

func TestHistoryTrie(t *testing.T) {
	require := require.New(t)
	cfg := db.DefaultConfig
	AccountKVNamespace := "Account"
	path := "test-history-trie.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	defer testutil.CleanupPathV2(testPath)
	require.NoError(err)

	cfg.DbPath = testPath
	opts := []db.KVStoreFlusherOption{
		db.FlushTranslateOption(func(wi *batch.WriteInfo) *batch.WriteInfo {
			if wi.WriteType() == batch.Delete {
				return nil
			}
			return wi
		}),
	}
	dao := db.NewBoltDB(cfg)
	ctx := context.Background()
	require.NoError(dao.Start(ctx))
	flusher, err := db.NewKVStoreFlusher(dao, batch.NewCachedBatch(), opts...)
	require.NoError(err)
	addrKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	value1 := []byte{1}
	value2 := []byte{2}

	trieDB, err := trie.NewKVStore(AccountKVNamespace, flusher.KVStoreWithBuffer())
	require.NoError(err)
	tr, err := New(KVStoreOption(trieDB), RootHashOption(nil))
	require.NoError(err)
	require.NoError(tr.Start(ctx))

	// insert 1 entry
	require.NoError(tr.Upsert(addrKey, value1))
	c, err := tr.Get(addrKey)
	require.NoError(err)
	require.Equal(value1, c)
	oldRoot, err := tr.RootHash()
	require.NoError(err)

	// update entry
	require.NoError(tr.Upsert(addrKey, value2))
	require.NoError(flusher.Flush())

	// get new value
	c, err = tr.Get(addrKey)
	require.NoError(err)
	require.Equal(value2, c)

	// get history value
	require.NoError(tr.SetRootHash(oldRoot))
	c, err = tr.Get(addrKey)
	require.NoError(err)
	require.Equal(value1, c)
}

func TestCollision(t *testing.T) {
	require := require.New(t)

	tr, err := New(KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer func() {
		require.NoError(tr.Stop(context.Background()))
	}()

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
	t.Run("test async mode", func(t *testing.T) {
		test4kEntries(t, true)
	})
	t.Run("test sync mode", func(t *testing.T) {
		test4kEntries(t, false)
	})
}

func test4kEntries(t *testing.T, enableAsync bool) {
	require := require.New(t)

	tr, err := New(KeyLengthOption(4), AsyncOption())
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	root, err := tr.RootHash()
	require.NoError(err)
	seed := time.Now().Nanosecond()
	// insert 4k entries
	var k [32]byte
	k[0] = byte(seed)
	c := 0
	for c = 0; c < 1<<12; c++ {
		k = hash.Hash256b(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:4]); err == nil {
			t.Logf("Warning: collision on k %x", k[:4])
			break
		}
		require.NoError(tr.Upsert(k[:4], v))
		newRoot, err := tr.RootHash()
		require.NoError(err)
		require.False(tr.IsEmpty())
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:4])
		require.NoError(err)
		require.Equal(v, b)
		// update <k, v>
		v = testV[7-k[0]&7]
		require.NoError(tr.Upsert(k[:4], v))
		b, err = tr.Get(k[:4])
		require.NoError(err)
		require.Equal(v, b)
		if c%64 == 0 {
			t.Logf("Put -- key: %x", k[:4])
		}
	}
	// delete 4k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := hash.Hash256b(d[:])
	d2 := hash.Hash256b(d1[:])
	d3 := hash.Hash256b(d2[:])
	d = d3
	for i := 0; i < c-3; i++ {
		d = hash.Hash256b(d[:])
		require.NoError(tr.Delete(d[:4]))
		newRoot, err := tr.RootHash()
		require.NoError(err)
		require.False(tr.IsEmpty())
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err = tr.Get(d[:4])
		require.Equal(trie.ErrNotExist, errors.Cause(err))
		if i%64 == 0 {
			t.Logf("Del -- key: %x", d[:4])
		}
	}
	require.NoError(tr.Delete(d1[:4]))
	require.NoError(tr.Delete(d2[:4]))
	require.NoError(tr.Delete(d3[:4]))
	// trie should fallback to empty
	require.True(tr.IsEmpty())
	require.NoError(tr.Stop(context.Background()))
}

func TestPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestPressure in short mode.")
	}

	require := require.New(t)

	tr, err := New(KeyLengthOption(4))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	root, err := tr.RootHash()
	require.NoError(err)
	seed := time.Now().Nanosecond()
	// insert 128k entries
	var k [32]byte
	k[0] = byte(seed)
	c := 0
	for c = 0; c < 1<<17; c++ {
		k = hash.Hash256b(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:4]); err == nil {
			t.Logf("Warning: collision on k %x", k[:4])
			break
		}
		require.NoError(tr.Upsert(k[:4], v))
		newRoot, err := tr.RootHash()
		require.NoError(err)
		require.False(tr.IsEmpty())
		require.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:4])
		require.NoError(err)
		require.Equal(v, b)
		if c%(2<<10) == 0 {
			t.Logf("Put -- key: %x", k[:4])
		}
	}
	// delete 128k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := hash.Hash256b(d[:])
	d2 := hash.Hash256b(d1[:])
	d3 := hash.Hash256b(d2[:])
	d = d3
	for i := 0; i < c-3; i++ {
		d = hash.Hash256b(d[:])
		require.NoError(tr.Delete(d[:4]))
		newRoot, err := tr.RootHash()
		require.NoError(err)
		require.False(tr.IsEmpty())
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err = tr.Get(d[:4])
		require.Equal(trie.ErrNotExist, errors.Cause(err))
		if i%(2<<10) == 0 {
			t.Logf("Del -- key: %x", d[:4])
		}
	}
	require.NoError(tr.Delete(d1[:4]))
	require.NoError(tr.Delete(d2[:4]))
	require.NoError(tr.Delete(d3[:4]))
	// trie should fallback to empty
	require.True(tr.IsEmpty())
	require.NoError(tr.Stop(context.Background()))
	t.Logf("Warning: test %d entries", c)
}

func TestTrieGet(t *testing.T) {
	var (
		require = require.New(t)
		tests   = []struct{ k, v []byte }{
			{[]byte("iotex"), []byte("coin")},
			{[]byte("block"), []byte("chain")},
			{[]byte("puppy"), []byte("dog")},
		}
	)

	trie, err := New(KeyLengthOption(5))
	require.NoError(err)
	require.NoError(trie.Start(context.Background()))
	defer require.NoError(trie.Stop(context.Background()))

	for _, test := range tests {
		require.NoError(trie.Upsert(test.k, test.v))
	}
	for _, test := range tests {
		val, err := trie.Get(test.k)
		require.NoError(err)
		require.Equal(test.v, val)
	}
}

func TestTrieUpsert(t *testing.T) {
	var (
		require = require.New(t)
		key     = []byte("puppy")
		tests   = []struct{ k, v []byte }{
			{key, []byte("dog")},
			{key, []byte("teddy")},
			{key, []byte("snoop")},
			{key, []byte("snooooooop")},
		}
	)

	trie, err := New(KeyLengthOption(5))
	require.NoError(err)
	require.NoError(trie.Start(context.Background()))
	defer require.NoError(trie.Stop(context.Background()))

	for _, test := range tests {
		require.NoError(trie.Upsert(test.k, test.v))
	}
	val, err := trie.Get(key)
	require.NoError(err)
	require.Equal(tests[len(tests)-1].v, val)
}

func TestTrieDelete(t *testing.T) {
	var (
		require = require.New(t)
		tests   = []struct{ k, v []byte }{
			{[]byte("iotex"), []byte("coin")},
			{[]byte("block"), []byte("chain")},
			{[]byte("puppy"), []byte("dog")},
		}
	)
	tr, err := New(KeyLengthOption(5))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer require.NoError(tr.Stop(context.Background()))

	for _, test := range tests {
		_, err = tr.Get(test.k)
		require.Equal(trie.ErrNotExist, errors.Cause(err))

		require.NoError(tr.Upsert(test.k, test.v))
	}
	for _, test := range tests {
		val, err := tr.Get(test.k)
		require.NoError(err)
		require.Equal(test.v, val)

		require.NoError(tr.Delete(test.k))
		_, err = tr.Get(test.k)
		require.Equal(trie.ErrNotExist, errors.Cause(err))
	}
	require.True(tr.IsEmpty())
}

func TestTrieSetRootHash(t *testing.T) {
	var (
		require = require.New(t)
		tests   = []struct{ k, v []byte }{
			{[]byte("iotex"), []byte("coin")},
			{[]byte("block"), []byte("chain")},
			{[]byte("chain"), []byte("link")},
			{[]byte("puppy"), []byte("dog")},
			{[]byte("night"), []byte("knight")},
		}
		trieDB = trie.NewMemKVStore()
	)

	tr, err := New(KVStoreOption(trieDB), KeyLengthOption(5))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer require.NoError(tr.Stop(context.Background()))
	for _, test := range tests {
		require.NoError(tr.Upsert(test.k, test.v))
	}
	rootHash, err := tr.RootHash()
	require.NoError(err)
	require.NotEqual(emptyTrieRootHash, rootHash)

	// new empty trie and load tr's rootHash
	tr1, err := New(KVStoreOption(trieDB), KeyLengthOption(5))
	require.NoError(err)
	require.NoError(tr1.Start(context.Background()))
	defer require.NoError(tr1.Stop(context.Background()))
	tr1Hash, err := tr1.RootHash()
	require.NoError(err)
	require.Equal(emptyTrieRootHash, tr1Hash)
	require.NoError(tr1.SetRootHash(rootHash))
	for _, test := range tests {
		val, err := tr1.Get(test.k)
		require.NoError(err)
		require.Equal(test.v, val)
	}
}
