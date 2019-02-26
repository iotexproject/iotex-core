// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
