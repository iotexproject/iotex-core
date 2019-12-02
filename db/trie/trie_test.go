// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
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
)

const testTriePath = "trie.test"

func TestEmptyTrie(t *testing.T) {
	require := require.New(t)
	tr, err := NewTrie()
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	require.True(tr.isEmptyRootHash(tr.RootHash()))
	require.NoError(tr.Stop(context.Background()))
}

func Test2Roots(t *testing.T) {
	require := require.New(t)

	// first trie
	trieDB := newInMemKVStore()
	tr, err := NewTrie(KVStoreOption(trieDB), KeyLengthOption(8))
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
	root := tr.RootHash()
	require.NoError(tr.Stop(context.Background()))

	// second trie
	tr1, err := NewTrie(KVStoreOption(trieDB), KeyLengthOption(8))
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
	root1 := tr1.RootHash()
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
	require.Equal(ErrNotExist, errors.Cause(err))

	// create a new one and load second trie's root
	tr2, err := NewTrie(KVStoreOption(trieDB), KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr2.Start(context.Background()))
	require.NoError(tr2.SetRootHash(root1))
	require.Equal(root1, tr2.RootHash())
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
	require.Equal(ErrNotExist, errors.Cause(err))
	require.NoError(tr.Stop(context.Background()))
	require.NoError(tr2.Stop(context.Background()))
}

func TestInsert(t *testing.T) {
	require := require.New(t)

	tr, err := NewTrie(KVStoreOption(newInMemKVStore()), KeyLengthOption(8))
	require.NotNil(tr)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	// this adds one L to root R
	t.Log("Put[cat]")
	require.NoError(tr.Upsert(cat, testV[2]))
	catRoot := tr.RootHash()
	require.False(tr.isEmptyRootHash(catRoot))
	root := catRoot

	// this splits L --> E + B + 2L (cat, rat)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat)
	 */
	t.Log("Put[rat]")
	require.NoError(tr.Upsert(rat, []byte("rat")))
	ratRoot := tr.RootHash()
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
	root = tr.RootHash()
	// root should keep the same since the value is same
	require.Equal(root, ratRoot)

	// this adds another L to B (car)
	/*
	 *  Root --1--> E --234567--> B --> (cat, rat, car)
	 */
	t.Log("Put[car]")
	require.NoError(tr.Upsert(car, testV[1]))
	newRoot := tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(car)
	require.NoError(err)
	require.Equal(testV[1], b)
	// delete car
	t.Log("Del[car]")
	require.NoError(tr.Delete(car))
	newRoot = tr.RootHash()
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
	newRoot = tr.RootHash()
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
	require.Equal(ErrNotExist, errors.Cause(err))

	// this deletes 'dog' and turns B1 into another E2
	/*
	 *  Root --1--> E --234--> E2 --5--> E1 --67--> B --> (cat, rat)
	 */
	t.Log("Del[dog]")
	require.NoError(tr.Delete(dog))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// this splits E1 --> B2 + E3 + L (match = 0, div = 2)
	/*
	 *  Root --1--> E --234--> E2 --5--> B2 --6--> E3 --7--> B --> (cat, rat)
	 *                                    | --8--> L --10--> egg
	 */
	t.Log("Put[egg]")
	require.NoError(tr.Upsert(egg, testV[4]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], b)
	// delete egg
	t.Log("Del[egg]")
	require.NoError(tr.Delete(egg))
	eggRoot := tr.RootHash()
	require.NotEqual(eggRoot, root)
	root = eggRoot

	// this splits E (with match = 4, div = 1)
	t.Log("Put[egg]")
	require.NoError(tr.Upsert(egg, testV[4]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	require.NoError(err)
	require.Equal(testV[4], b)
	// delete egg
	t.Log("Del[egg]")
	require.NoError(tr.Delete(egg))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	require.Equal(newRoot, eggRoot)
	root = newRoot
	_, err = tr.Get(dog)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(egg)
	require.Equal(ErrNotExist, errors.Cause(err))

	// insert 'ham' 'fox' 'cow'
	t.Log("Put[ham]")
	require.NoError(tr.Upsert(ham, testV[0]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], b)
	t.Log("Put[fox]")
	require.NoError(tr.Upsert(fox, testV[5]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(fox)
	require.NoError(err)
	require.Equal(testV[5], b)
	t.Log("Put[cow]")
	require.NoError(tr.Upsert(cow, testV[6]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(cow)
	require.NoError(err)
	require.Equal(testV[6], b)

	// delete fox rat cow
	t.Log("Del[fox]")
	require.NoError(tr.Delete(fox))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	t.Log("Del[rat]")
	require.NoError(tr.Delete(rat))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	t.Log("Del[cow]")
	require.NoError(tr.Delete(cow))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	_, err = tr.Get(fox)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(rat)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(cow)
	require.Equal(ErrNotExist, errors.Cause(err))
	b, err = tr.Get(ham)
	require.NoError(err)
	require.Equal(testV[0], b)

	// this adds another path to root B
	t.Log("Put[ant]")
	require.NoError(tr.Upsert(ant, testV[7]))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ant)
	require.NoError(err)
	require.Equal(testV[7], b)
	t.Log("Del[ant]")
	require.NoError(tr.Delete(ant))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	root = newRoot

	// delete "ham"
	t.Log("Del[ham]")
	require.NoError(tr.Delete(ham))
	newRoot = tr.RootHash()
	require.NotEqual(newRoot, root)
	_, err = tr.Get(ham)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ant)
	require.Equal(ErrNotExist, errors.Cause(err))
	b, err = tr.Get(cat)
	require.NoError(err)
	require.Equal(testV[2], b)

	// delete "cat"
	t.Log("Del[cat]")
	require.NoError(tr.Delete(cat))
	require.True(tr.isEmptyRootHash(tr.RootHash()))
	require.NoError(tr.Stop(context.Background()))
}

func TestBatchCommit(t *testing.T) {
	require := require.New(t)

	tr, err := NewTrie(KeyLengthOption(8))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	trieDB := tr.DB()
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
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(ham)
	require.Equal(ErrNotExist, errors.Cause(err))
	_, err = tr.Get(fox)
	require.Equal(ErrNotExist, errors.Cause(err))

	// insert 3 entries again
	require.NoError(tr.Upsert(dog, testV[3]))
	require.NoError(tr.Upsert(ham, testV[0]))
	require.NoError(tr.Upsert(fox, testV[6]))
	v, _ = tr.Get(fox)
	require.Equal(testV[6], v)
	require.NoError(tr.Upsert(fox, testV[5]))
	v, _ = tr.Get(fox)
	require.Equal(testV[5], v)
	root := tr.RootHash()
	// commit and reopen
	require.NoError(tr.Stop(context.Background()))
	tr, err = NewTrie(KVStoreOption(trieDB), RootHashOption(root), KeyLengthOption(8))
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
	cfg := config.Default.DB
	path := "test-history-trie.bolt"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg.DbPath = testPath
	dao := db.NewBoltDB(cfg)
	require.NoError(dao.Start(context.Background()))
	AccountKVNameSpace := "Account"
	AccountTrieRootKey := "accountTrieRoot"
	addrKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	value1 := []byte{1}
	value2 := []byte{2}

	trieDB, err := db.NewKVStoreForTrie(AccountKVNameSpace, dao)
	require.NoError(err)
	tr, err := NewTrie(KVStoreOption(trieDB), RootKeyOption(AccountTrieRootKey))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))

	// insert 1 entries
	require.NoError(tr.Upsert(addrKey, value1))
	c, err := tr.Get(addrKey)
	require.NoError(err)
	require.Equal(value1, c)
	oldRoot := tr.RootHash()

	// update entry
	require.NoError(tr.Upsert(addrKey, value2))

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

	tr, err := NewTrie(KeyLengthOption(8))
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
	require := require.New(t)

	tr, err := NewTrie(KeyLengthOption(4))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	root := tr.emptyRootHash()
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
		newRoot := tr.RootHash()
		require.False(tr.isEmptyRootHash(newRoot))
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
		newRoot := tr.RootHash()
		require.False(tr.isEmptyRootHash(newRoot))
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:4])
		require.Equal(ErrNotExist, errors.Cause(err))
		if i%64 == 0 {
			t.Logf("Del -- key: %x", d[:4])
		}
	}
	require.NoError(tr.Delete(d1[:4]))
	require.NoError(tr.Delete(d2[:4]))
	require.NoError(tr.Delete(d3[:4]))
	// trie should fallback to empty
	require.True(tr.isEmptyRootHash(tr.RootHash()))
	require.NoError(tr.Stop(context.Background()))
}

func TestPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestPressure in short mode.")
	}

	require := require.New(t)

	tr, err := NewTrie(KeyLengthOption(4))
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	root := tr.emptyRootHash()
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
		newRoot := tr.RootHash()
		require.False(tr.isEmptyRootHash(newRoot))
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
		newRoot := tr.RootHash()
		require.False(tr.isEmptyRootHash(newRoot))
		require.NotEqual(newRoot, root)
		root = newRoot
		_, err := tr.Get(d[:4])
		require.Equal(ErrNotExist, errors.Cause(err))
		if i%(2<<10) == 0 {
			t.Logf("Del -- key: %x", d[:4])
		}
	}
	require.NoError(tr.Delete(d1[:4]))
	require.NoError(tr.Delete(d2[:4]))
	require.NoError(tr.Delete(d3[:4]))
	// trie should fallback to empty
	require.True(tr.isEmptyRootHash(tr.RootHash()))
	require.NoError(tr.Stop(context.Background()))
	t.Logf("Warning: test %d entries", c)
}
