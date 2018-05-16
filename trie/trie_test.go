package trie

import (
	"container/list"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
)

func TestEmptyTrie(t *testing.T) {
	assert := assert.New(t)

	tr, err := NewTrie(db.NewMemKVStore())
	assert.Nil(err)
	assert.Equal(tr.RootHash(), emptyRoot)
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
	err = tr.Insert(cat, []byte("cat"))
	assert.Nil(err)
	catRoot := tr.RootHash()
	assert.NotEqual(catRoot, root)
	root = catRoot
	assert.Equal(uint64(1), tr.numLeaf)
	b, err := tr.Get(cat)
	assert.Nil(err)
	assert.Equal([]byte("cat"), b)

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
	err = tr.Insert(car, []byte("car"))
	assert.Nil(err)
	newRoot := tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(car)
	assert.Nil(err)
	assert.Equal([]byte("car"), b)
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
	err = tr.Insert(dog, []byte("dog"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	// Get returns "dog" now
	b, err = tr.Get(dog)
	assert.Nil(err)
	assert.Equal([]byte("dog"), b)
	logger.Info().Msg("Del[dog]")
	err = tr.Delete(dog)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// this splits E (with match = 0, div = 2)
	logger.Info().Msg("Put[egg]")
	err = tr.Insert(egg, []byte("egg"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	assert.Nil(err)
	assert.Equal([]byte("egg"), b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	assert.Nil(err)
	eggRoot := tr.RootHash()
	assert.NotEqual(eggRoot, root)
	root = eggRoot

	// this splits E (with match = 4, div = 1)
	err = tr.Insert(egg, []byte("egg"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(egg)
	assert.Nil(err)
	assert.Equal([]byte("egg"), b)
	// delete egg
	logger.Info().Msg("Del[egg]")
	err = tr.Delete(egg)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	assert.Equal(newRoot, eggRoot)
	root = newRoot

	// insert 'ham'
	logger.Info().Msg("Put[ham]")
	err = tr.Insert(ham, []byte("ham"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(ham)
	assert.Nil(err)
	assert.Equal([]byte("ham"), b)

	// insert 'fox'
	logger.Info().Msg("Put[fox]")
	err = tr.Insert(fox, []byte("fox"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(fox)
	assert.Nil(err)
	assert.Equal([]byte("fox"), b)

	// insert 'cow'
	logger.Info().Msg("Put[cow]")
	err = tr.Insert(cow, []byte("cow"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	b, err = tr.Get(cow)
	assert.Nil(err)
	assert.Equal([]byte("cow"), b)

	// delete fox
	logger.Info().Msg("Del[fox]")
	err = tr.Delete(fox)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// delete ham
	logger.Info().Msg("Del[ham]")
	err = tr.Delete(ham)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot

	// delete cow
	logger.Info().Msg("Del[cow]")
	err = tr.Delete(cow)
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
