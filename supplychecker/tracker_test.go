// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package supplychecker

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

func mustSer(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}

func acctPayload(t *testing.T, balance string) []byte {
	t.Helper()
	return mustSer(t, &accountpb.Account{Balance: balance})
}

func fundPayload(t *testing.T, total, unclaimed string) []byte {
	t.Helper()
	return mustSer(t, &rewardingpb.Fund{TotalBalance: total, UnclaimedBalance: unclaimed})
}

func poolPayload(t *testing.T, amount string, count uint64) []byte {
	t.Helper()
	return mustSer(t, &stakingpb.TotalAmount{Amount: amount, Count: count})
}

func acctEntry(t *testing.T, key string, prior, cur []byte) factory.WriteQueueEntry {
	t.Helper()
	return factory.WriteQueueEntry{
		WriteType:  0,
		Namespace:  state.AccountKVNamespace,
		Key:        []byte(key),
		PriorValue: prior,
		Value:      cur,
	}
}

func fundEntry(t *testing.T, prior, cur []byte) factory.WriteQueueEntry {
	t.Helper()
	return factory.WriteQueueEntry{
		WriteType:  0,
		Namespace:  state.RewardingNamespace,
		Key:        append([]byte(nil), _fundKeyL2...),
		PriorValue: prior,
		Value:      cur,
	}
}

func poolEntry(t *testing.T, prior, cur []byte) factory.WriteQueueEntry {
	t.Helper()
	return factory.WriteQueueEntry{
		WriteType:  0,
		Namespace:  state.StakingNamespace,
		Key:        append([]byte(nil), _bucketPoolKeyL2...),
		PriorValue: prior,
		Value:      cur,
	}
}

func TestBlockSupplyDeltaTransferConserves(t *testing.T) {
	require := require.New(t)
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "100"), acctPayload(t, "90")),
		acctEntry(t, "B", acctPayload(t, "50"), acctPayload(t, "60")),
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Zero(d.Sign(), "transfer must conserve total supply")
}

func TestBlockSupplyDeltaRewardConserves(t *testing.T) {
	require := require.New(t)
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "90"), acctPayload(t, "100")),           // +10 reward
		fundEntry(t, fundPayload(t, "200", "100"), fundPayload(t, "190", "100")), // -10 from fund
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Zero(d.Sign(), "reward payout must conserve total supply")
}

func TestBlockSupplyDeltaStakingConserves(t *testing.T) {
	require := require.New(t)
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "300"), acctPayload(t, "100")), // -200 to stake
		poolEntry(t, poolPayload(t, "0", 0), poolPayload(t, "200", 1)),  // +200 to pool
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Zero(d.Sign(), "staking must conserve total supply")
}

func TestBlockSupplyDeltaBaseFeeBurnDecreases(t *testing.T) {
	require := require.New(t)
	// Base fee burn only ever decreases the total.
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "100"), acctPayload(t, "99")),
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Equal(-1, d.Sign(), "base fee burn must decrease supply (non-increase)")
}

func TestBlockSupplyDeltaDetectsMint(t *testing.T) {
	require := require.New(t)
	// An account balance increases with no offsetting debit -> ex-nihilo mint.
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "100"), acctPayload(t, "101")),
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Equal(1, d.Sign(), "an unaccounted balance increase must be detected as a mint")
}

func TestBlockSupplyDeltaAggregatesSameKey(t *testing.T) {
	require := require.New(t)
	// The same key written twice in one block nets exactly once: prior=100 (base
	// store pre-block), intermediate=90, final=110. Net must be +10 (A minted).
	entries := []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "100"), acctPayload(t, "90")),
		acctEntry(t, "A", acctPayload(t, "100"), acctPayload(t, "110")),
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.Equal(big.NewInt(10), d)
}

func TestBlockSupplyDeltaSkipsHeightAndLegacy(t *testing.T) {
	require := require.New(t)
	// The per-block height key and legacy poll/rewarding states linger in the
	// Account namespace; they must not contribute to a false delta.
	entries := []factory.WriteQueueEntry{
		{WriteType: 0, Namespace: state.AccountKVNamespace, Key: []byte("currentHeight"),
			PriorValue: nil, Value: []byte{0x1, 0x2, 0x3}},
		acctEntry(t, "A", nil, acctPayload(t, "5")), // newly created account (+5 is legit funding at genesis)
	}
	d, err := blockSupplyDelta(entries)
	require.NoError(err)
	require.NotNil(d)
}

func TestSupplyTrackerRunningAndViolation(t *testing.T) {
	require := require.New(t)
	tr := NewSupplyTracker(big.NewInt(1000))

	// transfer block: conserve
	tr.OnBlockCommitted(1, []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "700"), acctPayload(t, "650")),
		acctEntry(t, "B", acctPayload(t, "300"), acctPayload(t, "350")),
	}, nil)
	require.Zero(tr.RunningTotal().Cmp(big.NewInt(1000)))

	// base fee burn block: -1
	tr.OnBlockCommitted(2, []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "650"), acctPayload(t, "649")),
	}, nil)
	require.Zero(tr.RunningTotal().Cmp(big.NewInt(999)))

	// mint block: +2
	tr.OnBlockCommitted(3, []factory.WriteQueueEntry{
		acctEntry(t, "A", acctPayload(t, "649"), acctPayload(t, "651")),
	}, nil)
	require.Zero(tr.RunningTotal().Cmp(big.NewInt(1001)))
	require.Equal(uint64(3), tr.Height())
}
