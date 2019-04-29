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
	"time"

	"github.com/pkg/errors"
)

type heightManager struct {
	heights []uint64
	times   []time.Time
}

func newHeightManager() *heightManager {
	return &heightManager{
		heights: []uint64{},
		times:   []time.Time{},
	}
}

func (m *heightManager) nearestHeightBefore(ts time.Time) uint64 {
	l := len(m.heights)
	if l == 0 {
		return 0
	}
	if m.times[0].After(ts) {
		return 0
	}
	head := 0
	tail := l
	for {
		if tail-head <= 1 {
			break
		}
		mid := (head + tail) / 2
		if m.times[mid].After(ts) {
			tail = mid
		} else {
			head = mid
		}
	}
	return m.heights[head]
}

func (m *heightManager) lastestHeight() uint64 {
	l := len(m.heights)
	if l == 0 {
		return 0
	}
	return m.heights[l-1]
}

func (m *heightManager) validate(height uint64, ts time.Time) error {
	l := len(m.heights)
	if l == 0 {
		return nil
	}
	if m.heights[l-1] >= height {
		return errors.Errorf(
			"invalid height %d, current tail is %d",
			height,
			m.heights[l-1],
		)
	}
	if !ts.After(m.times[l-1]) {
		return errors.Errorf(
			"invalid timestamp %s, current tail is %s",
			ts,
			m.times[l-1],
		)
	}
	return nil
}

func (m *heightManager) add(height uint64, ts time.Time) error {
	if err := m.validate(height, ts); err != nil {
		return err
	}
	m.heights = append(m.heights, height)
	m.times = append(m.times, ts)
	return nil
}
