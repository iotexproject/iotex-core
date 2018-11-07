// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortedSlice(t *testing.T) {
	t.Parallel()

	compare := func(x interface{}, y interface{}) int {
		return int(int64(x.(uint32)) - int64(y.(uint32)))
	}

	input := make([]uint32, 15)
	for i := range input {
		input[i] = uint32(i + 1)
	}
	for i := range input {
		j := rand.Intn(i + 1)
		input[i], input[j] = input[j], input[i]
	}

	var slice1 SortedSlice
	for _, e := range input {
		slice1 = slice1.Append(e, compare)
	}
	for i, e := range input {
		val, ok := slice1.Get(e, compare)
		assert.True(t, ok)
		assert.Equal(t, input[i], val.(uint32))
	}
	_, ok := slice1.Get(uint32(0), compare)
	assert.False(t, ok)
	_, ok = slice1.Get(uint32(16), compare)
	assert.False(t, ok)
	slice1 = slice1.Delete(uint32(13), compare)
	_, ok = slice1.Get(uint32(13), compare)
	assert.False(t, ok)
	slice1 = slice1.Delete(uint32(15), compare)
	_, ok = slice1.Get(uint32(15), compare)
	assert.False(t, ok)
	slice1 = slice1.Append(uint32(4), compare)
	assert.Equal(t, 14, len(slice1))
	slice1 = slice1.Delete(uint32(4), compare)
	assert.Equal(t, 12, len(slice1))

	data, err := slice1.Serialize()
	require.NoError(t, err)
	var slice2 SortedSlice
	require.NoError(t, slice2.Deserialize(data))
	assert.Equal(t, slice1, slice2)
}
