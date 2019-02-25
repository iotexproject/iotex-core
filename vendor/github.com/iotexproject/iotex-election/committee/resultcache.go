// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package committee

import (
	"github.com/iotexproject/iotex-election/types"
)

type resultCache struct {
	size    uint32
	results []*types.ElectionResult
	heights []uint64
	index   map[uint64]int
	cursor  int
}

func newResultCache(size uint32) *resultCache {
	return &resultCache{
		size:    size,
		results: make([]*types.ElectionResult, size),
		heights: make([]uint64, size),
		index:   map[uint64]int{},
		cursor:  0,
	}
}

func (c *resultCache) insert(height uint64, r *types.ElectionResult) {
	if i, exists := c.index[height]; exists {
		c.results[i] = r
		return
	}
	delete(c.index, c.heights[c.cursor])
	c.results[c.cursor] = r
	c.heights[c.cursor] = height
	c.index[height] = c.cursor
	c.cursor = (c.cursor + 1) % int(c.size)
}

func (c *resultCache) get(height uint64) *types.ElectionResult {
	i, exists := c.index[height]
	if !exists {
		return nil
	}
	return c.results[i]
}
