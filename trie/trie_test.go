package trie

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/db"
)

func TestEmptyTrie(t *testing.T) {
	assert := assert.New(t)

	tr, err := NewTrie(db.NewMemKVStore())
	assert.Nil(err)
	assert.Equal(tr.RootHash(), emptyRoot)
}

func TestInsert(t *testing.T) {
	assert := assert.New(t)

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
	err = tr.Insert(cat, []byte("cat"))
	assert.Nil(err)
	newRoot := tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	assert.Equal(uint64(1), tr.numLeaf)
	// query can find it now
	ptr, match, err = tr.query(cat)
	assert.NotNil(ptr)
	assert.Equal(len(cat), match)
	assert.Nil(err)
	tr.clear()
	// Get returns "cat" now
	b, err := tr.Get(cat)
	assert.Nil(err)
	assert.Equal([]byte("cat"), b)
	fmt.Println("[cat] = 'cat'")
	// this insert will split leaf node
	err = tr.Insert(rat, []byte("rat"))
	assert.Nil(err)
	newRoot = tr.RootHash()
	assert.NotEqual(newRoot, root)
	root = newRoot
	assert.Equal(uint64(2), tr.numBranch)
	assert.Equal(uint64(1), tr.numExt)
	assert.Equal(uint64(3), tr.numLeaf)
	// query can find it now
	ptr, match, err = tr.query(rat)
	assert.NotNil(ptr)
	assert.Equal(len(rat), match)
	assert.Nil(err)
	tr.clear()
	// Get returns "rat" now
	b, err = tr.Get(rat)
	assert.Nil(err)
	assert.Equal([]byte("rat"), b)
	fmt.Println("[rat] = 'rat'")
}
