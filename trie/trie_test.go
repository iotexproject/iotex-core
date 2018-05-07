package trie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrie(t *testing.T) {
	assert := assert.New(t)

	tr, err := NewTrie()
	assert.Nil(err)
	assert.Equal(tr.RootHash(), emptyRoot)
}
