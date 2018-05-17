package trie

import (
	"container/list"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
)

const testTriePath = "trie.test"

func TestEmptyTrie(t *testing.T) {
	assert := assert.New(t)

	defer os.Remove(testTriePath)
	tr, err := NewTrie(testTriePath)
	assert.Nil(err)
	assert.Equal(tr.RootHash(), emptyRoot)
	assert.Nil(tr.Close())
}

func TestInsert(t *testing.T) {
	assert := assert.New(t)
	logger.UseDebugLogger()

	tr := trie{dao: db.NewMemKVStore(), root: &branch{}, toRoot: list.New(), addNode: list.New(), numBranch: 1}
	root := emptyRoot
	assert.Equal(uint64(1), tr.numBranch)
	// query non-existing entry
	ptr, match, err := tr.query(cat)
	assert.NotNil(ptr)
	assert.Equal(0, match)
	assert.NotNil(err)
	tr.clear()
	// this splits the root B
	logger.Info().Msg("Put[cat]")
	err = tr.Insert(cat, testV[2])
	assert.Nil(err)
	catRoot := tr.RootHash()
	assert.NotEqual(catRoot, root)
	root = catRoot
	assert.Equal(uint64(1), tr.numLeaf)
	b, err := tr.Get(cat)
	assert.Nil(err)
	assert.Equal(testV[2], b)

	// this splits L --> E + B + 2L
	logger.Info().Msg("Put[rat]")
	err = tr.Insert(rat, []byte("rat"))
	assert.Nil(err)
	ratRoot := tr.RootHash()
	assert.NotEqual(ratRoot, root)
	root = ratRoot
	assert.Equal(uint64(2), tr.numBranch)
	assert.Equal(uint64(1), tr.numExt)
	assert.Equal(uint64(3), tr.numLeaf)
	b, err = tr.Get(rat)
	assert.Nil(err)
	assert.Equal([]byte("rat"), b)

	// this adds another L to B
	logger.Info().Msg("Put[car]")
	err = tr.Insert(car, testV[1])
	assert.Nil(err)
	newRoot := tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(car)
	assert.Nil(err)
	assert.Equal(testV[1], b)
	// delete car
	logger.Info().Msg("Del[car]")
	err = tr.Delete(car)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	assert.Equal(newRoot, ratRoot)
	root = newRoot

	// this splits E (with match = 3, div = 3)
	logger.Info().Msg("Put[dog]")
	err = tr.Insert(dog, testV[3])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	// Get returns "dog" now
	b, err = tr.Get(dog)
	assert.Nil(err)
	assert.Equal(testV[3], b)
	logger.Info().Msg("Del[dog]")
	err = tr.Delete(dog)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// this splits E (with match = 0, div = 2)
	logger.Info().Msg("Put[egg]")
	err = tr.Insert(egg, testV[4])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	assert.Nil(err)
	assert.Equal(testV[4], b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	assert.Nil(err)
	eggRoot := tr.RootHash()
	assert.NotEqual(eggRoot, root)
	root = eggRoot

	// this splits E (with match = 4, div = 1)
	logger.Info().Msg("Put[egg]")
	err = tr.Insert(egg, testV[4])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	assert.Nil(err)
	assert.Equal(testV[4], b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	assert.Equal(newRoot, eggRoot)
	root = newRoot

	// insert 'ham' 'fox' 'cow'
	logger.Info().Msg("Put[ham]")
	err = tr.Insert(ham, testV[0])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ham)
	assert.Nil(err)
	assert.Equal(testV[0], b)
	logger.Info().Msg("Put[fox]")
	err = tr.Insert(fox, testV[5])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(fox)
	assert.Nil(err)
	assert.Equal(testV[5], b)
	logger.Info().Msg("Put[cow]")
	err = tr.Insert(cow, testV[6])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(cow)
	assert.Nil(err)
	assert.Equal(testV[6], b)

	// delete fox ham cow
	logger.Info().Msg("Del[fox]")
	err = tr.Delete(fox)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	logger.Info().Msg("Del[ham]")
	err = tr.Delete(ham)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	logger.Info().Msg("Del[cow]")
	err = tr.Delete(cow)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// this adds another path to root B
	logger.Info().Msg("Put[ant]")
	err = tr.Insert(ant, testV[7])
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ant)
	assert.Nil(err)
	assert.Equal(testV[7], b)
	logger.Info().Msg("Del[ant]")
	err = tr.Delete(ant)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// delete "rat"
	logger.Info().Msg("Del[rat]")
	err = tr.Delete(rat)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	b, err = tr.Get(rat)
	assert.NotNil(err)
	assert.Equal([]byte(nil), b)
	// delete "cat"
	logger.Info().Msg("Del[cat]")
	err = tr.Delete(cat)
	assert.Nil(err)
	assert.Equal(emptyRoot, tr.RootHash())
}

func Test1kEntries(t *testing.T) {
	assert := assert.New(t)
	logger.UseDebugLogger()

	defer os.Remove(testTriePath)
	tr, err := NewTrie(testTriePath)
	assert.Nil(err)
	root := emptyRoot
	seed := time.Now().Nanosecond()
	// insert 64k entries
	var k [32]byte
	k[0] = byte(seed)
	for i := 0; i < 1<<10; i++ {
		k = blake2b.Sum256(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:8]); err == nil {
			continue
		}
		logger.Info().Hex("key", k[:8]).Msg("Put --")
		err := tr.Insert(k[:8], v)
		assert.Nil(err)
		newRoot := tr.RootHash()
		assert.NotEqual(newRoot, emptyRoot)
		assert.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:8])
		assert.Nil(err)
		assert.Equal(v, b)
	}
	// delete 64k entries
	var d [32]byte
	d[0] = byte(seed)
	// save the first 3, delete them last
	d1 := blake2b.Sum256(d[:])
	d2 := blake2b.Sum256(d1[:])
	d3 := blake2b.Sum256(d2[:])
	d = d3
	for i := 0; i < 1<<10-3; i++ {
		d = blake2b.Sum256(d[:])
		logger.Info().Hex("key", d[:8]).Msg("Del --")
		assert.Nil(tr.Delete(d[:8]))
	}
	assert.Nil(tr.Delete(d1[:8]))
	assert.Nil(tr.Delete(d2[:8]))
	assert.Nil(tr.Delete(d3[:8]))
	// trie should fallback to empty
	assert.Equal(emptyRoot, tr.RootHash())
}

func TestPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestPressure in short mode.")
	}

	assert := assert.New(t)
	logger.UseDebugLogger()

	defer os.Remove(testTriePath)
	tr, err := NewTrie(testTriePath)
	assert.Nil(err)
	root := emptyRoot
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
		err := tr.Insert(k[:8], v)
		assert.Nil(err)
		newRoot := tr.RootHash()
		assert.NotEqual(newRoot, emptyRoot)
		assert.NotEqual(newRoot, root)
		root = newRoot
		b, err := tr.Get(k[:8])
		assert.Nil(err)
		assert.Equal(v, b)
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
		assert.Nil(tr.Delete(d[:8]))
	}
	assert.Nil(tr.Delete(d1[:8]))
	assert.Nil(tr.Delete(d2[:8]))
	assert.Nil(tr.Delete(d3[:8]))
	// trie should fallback to empty
	assert.Equal(emptyRoot, tr.RootHash())
}
