package trie

import (
	"container/list"
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

	tr := trie{db.NewMemKVStore(), &branch{}, list.New()}
	// query non-existing entry
	ptr, match, err := tr.query(cat)
	assert.NotNil(ptr)
	assert.Equal(0, match)
	assert.NotNil(err)
	assert.Equal(1, tr.toRoot.Len())
	n := tr.toRoot.Back()
	assert.Equal(ptr, n.Value)
	b, err := tr.Get(dog)
	assert.NotNil(err)
	// insert
	err = tr.Insert(cat, []byte("cat"))
	assert.Nil(err)
	assert.NotEqual(tr.RootHash(), emptyRoot)
	// query can find it now
	ptr, match, err = tr.query(cat)
	assert.NotNil(ptr)
	assert.Equal(len(cat), match)
	assert.Nil(err)
	// Get returns "cat" now
	b, err = tr.Get(cat)
	assert.Nil(err)
	assert.Equal([]byte("cat"), b)
}
