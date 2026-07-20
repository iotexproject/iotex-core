// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// TestOnlyPoolEntry is a single row from the pending block-reward pool
// enumerated by TestOnlyDumpRewardState. CandidateID is a copy safe to
// retain after the underlying state changes.
type TestOnlyPoolEntry struct {
	CandidateID []byte
	Amount      *big.Int
}

// TestOnlyRewardStateSnapshot is a deterministic snapshot of the rewarding
// protocol state suitable for byte-level equality assertions between two
// independent runs of the same fixture. Sole consumer is PR 5's
// determinism regression tests; production must not depend on this.
//
// PerAddress is limited to the caller-supplied address set. Callers that
// need a full accounting must pass every address the fixture could credit
// — delegates' reward addresses, voters, foundation bonus recipient,
// block producers. Nil-balance addresses are omitted so an unused address
// does not affect equality.
type TestOnlyRewardStateSnapshot struct {
	TotalBalance     *big.Int
	UnclaimedBalance *big.Int
	PerAddress       map[string]*big.Int
	PoolEntries      []TestOnlyPoolEntry
	CursorPresent    bool
	CursorDelegates  uint32
	CursorIndex      uint32
	CursorVoterIndex uint32
	CursorTargetEra  uint64
}

// TestOnlyDumpRewardState snapshots fund state, per-address unclaimed
// balances for addrs, all pending pool entries in deterministic order,
// and the cursor summary. Returns a fully-owned struct — reading it does
// not alias into the state manager.
func (p *Protocol) TestOnlyDumpRewardState(
	ctx context.Context,
	sr protocol.StateReader,
	addrs []address.Address,
) (*TestOnlyRewardStateSnapshot, error) {
	total, _, err := p.TotalBalance(ctx, sr)
	if err != nil {
		return nil, err
	}
	unclaimed, _, err := p.AvailableBalance(ctx, sr)
	if err != nil {
		return nil, err
	}
	perAddr := make(map[string]*big.Int, len(addrs))
	for _, a := range addrs {
		if a == nil {
			continue
		}
		bal, _, err := p.UnclaimedBalance(ctx, sr, a)
		if err != nil {
			return nil, err
		}
		if bal.Sign() > 0 {
			perAddr[a.String()] = new(big.Int).Set(bal)
		}
	}
	entries, err := p.TestOnlyAllPoolEntries(ctx, sr)
	if err != nil {
		return nil, err
	}
	idx, voterIdx, totalCands, era, present, err := p.TestOnlyEpochDrainSnapshot(ctx, sr)
	if err != nil {
		return nil, err
	}
	return &TestOnlyRewardStateSnapshot{
		TotalBalance:     new(big.Int).Set(total),
		UnclaimedBalance: new(big.Int).Set(unclaimed),
		PerAddress:       perAddr,
		PoolEntries:      entries,
		CursorPresent:    present,
		CursorDelegates:  totalCands,
		CursorIndex:      idx,
		CursorVoterIndex: voterIdx,
		CursorTargetEra:  era,
	}, nil
}

// TestOnlyAllPoolEntries walks the pending block-reward pool index and
// returns each (candID, amount) pair sorted by candID bytes. Zero-amount
// entries are elided so equality does not depend on residuals that were
// never physically written.
func (p *Protocol) TestOnlyAllPoolEntries(
	ctx context.Context,
	sr protocol.StateReader,
) ([]TestOnlyPoolEntry, error) {
	ids, err := p.readPendingBlockRewardPoolIndex(ctx, sr)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i], ids[j]) < 0 })
	out := make([]TestOnlyPoolEntry, 0, len(ids))
	for _, id := range ids {
		amt, err := p.readPendingBlockRewardPool(ctx, sr, id)
		if err != nil {
			return nil, err
		}
		if amt.Sign() == 0 {
			continue
		}
		cp := make([]byte, len(id))
		copy(cp, id)
		out = append(out, TestOnlyPoolEntry{CandidateID: cp, Amount: new(big.Int).Set(amt)})
	}
	return out, nil
}

// TestOnlyAssertFundInvariant checks the core rewarding-fund conservation
// identity:
//
//	totalBalance == unclaimedBalance + Σ(perAddress balance) + Σ(pool balance)
//
// It must hold at every block boundary. The addrs slice must include
// every address that could have been credited a reward — the helper
// cannot enumerate them on its own without a namespace scan.
//
// Returns nil on parity, or a formatted error describing the delta.
func (p *Protocol) TestOnlyAssertFundInvariant(
	ctx context.Context,
	sr protocol.StateReader,
	addrs []address.Address,
) error {
	total, _, err := p.TotalBalance(ctx, sr)
	if err != nil {
		return err
	}
	unclaimed, _, err := p.AvailableBalance(ctx, sr)
	if err != nil {
		return err
	}
	sumAddr := new(big.Int)
	for _, a := range addrs {
		if a == nil {
			continue
		}
		bal, _, err := p.UnclaimedBalance(ctx, sr, a)
		if err != nil {
			return err
		}
		sumAddr = new(big.Int).Add(sumAddr, bal)
	}
	entries, err := p.TestOnlyAllPoolEntries(ctx, sr)
	if err != nil {
		return err
	}
	sumPool := new(big.Int)
	for _, e := range entries {
		sumPool = new(big.Int).Add(sumPool, e.Amount)
	}
	lhs := total
	rhs := new(big.Int).Add(new(big.Int).Add(unclaimed, sumAddr), sumPool)
	if lhs.Cmp(rhs) != 0 {
		return fmt.Errorf(
			"rewarding fund invariant violated: total=%s but unclaimed+perAddr+pool=%s "+
				"(unclaimed=%s sumAddr=%s sumPool=%s delta=%s)",
			lhs.String(), rhs.String(),
			unclaimed.String(), sumAddr.String(), sumPool.String(),
			new(big.Int).Sub(lhs, rhs).String(),
		)
	}
	return nil
}
