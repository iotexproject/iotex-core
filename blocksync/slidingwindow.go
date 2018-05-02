// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
)

const (
	// Open indicates an open state
	Open = iota
	// Closing indicates a closing state
	Closing = Open + 1
	// Closed indicates a closed state
	Closed = Closing + 1
)

// WindowSize defines the size of window
var WindowSize uint64 = 8

var (
	// ErrInvalidRange indicates invalid range.
	ErrInvalidRange = errors.New("invalid open/close values")
)

//
// A sliding window is a range [close, open] where close denotes the maximal value
// that exists, while open denotes the minimal value that does not exist yet
//
// It is used to track blocks received from p2p network during a block sync event
// For instance a sliding window of [100, 400] means blocks 101~400 are missing and
// need to be downloaded from peers. The sliding window manages the progress of such
// sync event, and exposes interface for BlockSyncer to use

// SlidingWindow implements a sliding window
type SlidingWindow struct {
	mu        sync.RWMutex
	start     time.Time
	end       time.Time
	State     int
	prevState int
	close     uint64
	open      uint64
	orphan    []int           // orphaned item that has not been received yet
	elapse    []time.Duration // time (in millisecond) it takes to complete each sync task
	bc        *blockchain.Blockchain
}

// NewSlidingWindow returns a SlidingWindow instance
func NewSlidingWindow() *SlidingWindow {
	return &SlidingWindow{State: Open, prevState: Open}
}

// SetRange set the initial range for sliding window
func (sw *SlidingWindow) SetRange(left uint64, right uint64) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if right <= left {
		return ErrInvalidRange
	}
	sw.close = left
	sw.open = right
	sw.updateState()
	// record the time the sync task starts
	sw.start = time.Now()
	return nil
}

// Next returns the next close value of the sliding window
func (sw *SlidingWindow) Next() uint64 {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.close + 1
}

// updateState updates the sliding window state
func (sw *SlidingWindow) updateState() {
	sw.prevState = sw.State
	gap := sw.open - sw.close
	switch {
	case gap == 1:
		sw.State = Closed
	case gap < WindowSize:
		sw.State = Closing
	default:
		sw.State = Open
	}
	glog.Infof("window = [%d  %d], state = %d | %d", sw.close, sw.open, sw.prevState, sw.State)
}

// Update updates the window [close, open]
func (sw *SlidingWindow) Update(value uint64) {
	sw.mu.Lock()
	switch {
	case value > sw.open:
		sw.open = value
	case value == sw.close+1:
		sw.close++
		if sw.close == sw.open {
			sw.open++
		}
	}
	sw.updateState()
	sw.mu.Unlock()
}

// TurnClose returns true if State transitions Open --> Closing/Closed
func (sw *SlidingWindow) TurnClose() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.prevState == Open && sw.State == Closing
}

// TurnOpen returns true if State transitions Closing/Closed --> Open
func (sw *SlidingWindow) TurnOpen() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.prevState != Open && sw.State == Open
}
