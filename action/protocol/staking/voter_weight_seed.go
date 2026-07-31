// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// _voterWeightSeedKey is the single-value key holding the seeding cursor.
var _voterWeightSeedKey = []byte{_voterWeightSeed}

// voterWeightSeedCursor tracks the one-time write-out of voter weights that
// follows IIP-59 activation.
//
// The weights become committed state at the activation height, but the set is
// too large to write in one block (~7s at 30k buckets against a 2.5s budget), so
// it is flushed a batch at a time. This cursor is that progress marker, and it
// is consensus state: every node advances it identically.
//
// The cursor is a *key position*, not an offset. Voters are added and removed
// while the flush is running, and a numeric offset would skip or repeat entries
// as the set shifts underneath it; a key position is stable against both.
type voterWeightSeedCursor struct {
	// LastCand/LastVoter is the last pair written, an exclusive lower bound on
	// the next batch. Meaningless until Started.
	LastCand  hash.Hash160
	LastVoter hash.Hash160
	// Started distinguishes "nothing written yet" from "resumed at the
	// zero-valued key", which is a legitimate position.
	Started bool
	// Done is set when the walk has passed the last pair. From then on the
	// cursor is read once per block and nothing else happens.
	Done       bool
	DoneHeight uint64
}

const _voterWeightSeedCursorLen = 2*len(hash.Hash160{}) + 2 + 8

// Serialize implements state.Serializer.
func (c *voterWeightSeedCursor) Serialize() ([]byte, error) {
	out := make([]byte, 0, _voterWeightSeedCursorLen)
	out = append(out, c.LastCand[:]...)
	out = append(out, c.LastVoter[:]...)
	out = append(out, boolToByte(c.Started), boolToByte(c.Done))
	var height [8]byte
	binary.BigEndian.PutUint64(height[:], c.DoneHeight)
	return append(out, height[:]...), nil
}

// Deserialize implements state.Deserializer.
func (c *voterWeightSeedCursor) Deserialize(buf []byte) error {
	if len(buf) != _voterWeightSeedCursorLen {
		return errors.Errorf("voter weight seed cursor must be %d bytes, got %d", _voterWeightSeedCursorLen, len(buf))
	}
	n := len(hash.Hash160{})
	copy(c.LastCand[:], buf[:n])
	copy(c.LastVoter[:], buf[n:2*n])
	c.Started = buf[2*n] != 0
	c.Done = buf[2*n+1] != 0
	c.DoneHeight = binary.BigEndian.Uint64(buf[2*n+2:])
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *voterWeightSeedCursor) Encode() (systemcontracts.GenericValue, error) {
	data, err := c.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (c *voterWeightSeedCursor) Decode(v systemcontracts.GenericValue) error {
	return c.Deserialize(v.PrimaryData)
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// position returns the exclusive lower bound for the next batch, or nil when
// the walk has not started.
func (c *voterWeightSeedCursor) position() *voterWeightRef {
	if c == nil || !c.Started {
		return nil
	}
	return &voterWeightRef{cand: c.LastCand, voter: c.LastVoter}
}

// readVoterWeightSeedCursor returns the persisted cursor, or nil if seeding has
// not begun.
func readVoterWeightSeedCursor(sr protocol.StateReader) (*voterWeightSeedCursor, error) {
	c := &voterWeightSeedCursor{}
	if _, err := sr.State(c,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_voterWeightSeedKey),
	); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read voter weight seed cursor")
	}
	return c, nil
}

func writeVoterWeightSeedCursor(sm protocol.StateManager, c *voterWeightSeedCursor) error {
	_, err := sm.PutState(c,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_voterWeightSeedKey),
	)
	return errors.Wrap(err, "failed to write voter weight seed cursor")
}

// voterWeightSeedingComplete reports whether the activation flush has finished.
// Before activation there is nothing to flush and no cursor, which counts as
// incomplete: callers use this to decide whether the persisted weights cover
// the whole voter set, and before activation they cover none of it.
func voterWeightSeedingComplete(sr protocol.StateReader) (bool, error) {
	c, err := readVoterWeightSeedCursor(sr)
	if err != nil {
		return false, err
	}
	return c != nil && c.Done, nil
}

// seedVoterWeights writes the next batch of voter weights into state.
//
// It does not read buckets. At the activation height the in-memory view already
// holds the complete table — the staking hooks have maintained it on both sides
// of the fork and a restart rebuilds it from buckets — so seeding is a flush of
// what is already in memory, not a recomputation of it.
//
// That is what makes concurrent mutation a non-issue. Commit writes each touched
// pair's *absolute* current weight, so a pair the block mutates is written
// correctly whether or not the cursor has reached it, and writing it again later
// is idempotent. Pairs created during the window are written by the same path.
// Nothing here has to reason about which side of the cursor a bucket falls on.
//
// Returns true when this call completed the walk.
func seedVoterWeights(ctx context.Context, sm protocol.StateManager, batchSize uint64) (bool, error) {
	if !voterWeightPersistenceEnabled(ctx) {
		// Pre-activation: nothing is persisted, so there is nothing to seed and
		// no cursor to create. The whole path stays inert.
		return false, nil
	}
	cursor, err := readVoterWeightSeedCursor(sm)
	if err != nil {
		return false, err
	}
	if cursor != nil && cursor.Done {
		return false, nil
	}
	if cursor == nil {
		cursor = &voterWeightSeedCursor{}
	}

	view := voterWeightsFromSM(sm)
	if view == nil {
		// No view installed (test setups that skip Protocol.Start). Nothing can
		// be flushed, and refusing to start would be a halt — leave the cursor
		// where it is and let a later block make progress.
		return false, nil
	}

	// batchSize 0 means "everything in one block", which is only sane for tests
	// and small chains.
	limit := int(batchSize)
	if batchSize == 0 {
		limit = -1
	}
	pairs := view.SeedPairsAfter(cursor.position(), limit)
	for _, ref := range pairs {
		view.MarkForRewrite(ref.cand, ref.voter)
	}

	if len(pairs) > 0 {
		last := pairs[len(pairs)-1]
		cursor.LastCand, cursor.LastVoter = last.cand, last.voter
		cursor.Started = true
	}
	// A short batch means the walk reached the end of the table.
	if limit < 0 || len(pairs) < limit {
		cursor.Done = true
		cursor.DoneHeight = protocol.MustGetBlockCtx(ctx).BlockHeight
	}
	if err := writeVoterWeightSeedCursor(sm, cursor); err != nil {
		return false, err
	}
	return cursor.Done, nil
}

// refLess orders pairs by candidate then voter — the order SeedPairsAfter walks
// and the order the cursor's position is interpreted in.
func refLess(a, b voterWeightRef) bool {
	if c := bytes.Compare(a.cand[:], b.cand[:]); c != 0 {
		return c < 0
	}
	return bytes.Compare(a.voter[:], b.voter[:]) < 0
}
