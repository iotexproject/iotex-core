// Copyright (c) 2019 IoTeX
// This program is free software: you can redistribute it and/or modify it under the terms of the
// GNU General Public License as published by the Free Software Foundation, either version 3 of
// the License, or (at your option) any later version.
// This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; 
// without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See
// the GNU General Public License for more details.
// You should have received a copy of the GNU General Public License along with this program. If
// not, see <http://www.gnu.org/licenses/>.

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
