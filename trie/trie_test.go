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

	tr := trie{db.NewMemKVStore(), &branch{}, list.New(), list.New(), 1, 0, 0}
	root := emptyRoot
	assert.Equal(uint64(1), tr.numBranch)
	// query non-existing entry
	ptr, match, err := tr.query(cat)
	assert.NotNil(ptr)
	assert.Equal(0, match)
	assert.NotNil(err)
	tr.clear()
	// insert
	logger.Info().Msg("Put[cat]")
	err = tr.Insert(cat, []byte("cat"))
	assert.Nil(err)
	catRoot := tr.RootHash()
	assert.NotEqual(catRoot, root)
	root = catRoot
	assert.Equal(uint64(1), tr.numLeaf)
	// Get returns "cat" now
	b, err := tr.Get(cat)
	assert.Nil(err)
	assert.Equal([]byte("cat"), b)
	logger.Info().Msg("[cat] = 'cat'")
	// this insert will split leaf node
	logger.Info().Msg("Put[rat]")
	err = tr.Insert(rat, []byte("rat"))
	assert.Nil(err)
	newRoot := tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	assert.Equal(uint64(2), tr.numBranch)
	assert.Equal(uint64(1), tr.numExt)
	assert.Equal(uint64(3), tr.numLeaf)
	// Get returns "rat" now
	b, err = tr.Get(rat)
	assert.Nil(err)
	assert.Equal([]byte("rat"), b)
	logger.Info().Msg("[rat] = 'rat'")
	// delete "rat"
	logger.Info().Msg("Del[rat]")
	err = tr.Delete(rat)
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	assert.Equal(newRoot, catRoot)
	b, err = tr.Get(rat)
	assert.NotNil(err)
	assert.Equal([]byte(nil), b)
	logger.Info().Msg("[rat] = nil")
	// delete "rat"
	logger.Info().Msg("Del[cat]")
	err = tr.Delete(cat)
	assert.Nil(err)
	assert.Equal(emptyRoot, tr.RootHash())
	logger.Info().Msg("[cat] = nil")
}
