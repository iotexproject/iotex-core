// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package supplychecker

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"google.golang.org/protobuf/proto"
)

// fakeStateReader is a minimal, in-memory protocol.StateReader that models the
// three accounting reservoirs the observer reads: primary account balances,
// the rewarding fund, and the staking bucket pool.
type fakeStateReader struct {
	accounts map[hash.Hash160]*big.Int
	fund     *big.Int
	pool     *big.Int
	hasFund  bool
	hasPool  bool
}

func newFakeStateReader() *fakeStateReader {
	return &fakeStateReader{
		accounts: map[hash.Hash160]*big.Int{},
		fund:     big.NewInt(0),
		pool:     big.NewInt(0),
		hasFund:  true,
		hasPool:  true,
	}
}

func (f *fakeStateReader) Height() (uint64, error) { return 0, nil }
func (f *fakeStateReader) ReadView(string) (protocol.View, error) {
	return nil, nil
}

func (f *fakeStateReader) State(value interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	switch cfg.Namespace {
	case state.RewardingNamespace:
		if !f.hasFund {
			return 0, state.ErrStateNotExist
		}
		fnd, ok := value.(*rewardingFund)
		if !ok {
			return 0, state.ErrUnknownAccountType
		}
		fnd.totalBalance = new(big.Int).Set(f.fund)
		fnd.unclaimedBalance = new(big.Int).Set(f.fund)
	case state.StakingNamespace:
		if !f.hasPool {
			return 0, state.ErrStateNotExist
		}
		pool, ok := value.(*bucketPoolTotal)
		if !ok {
			return 0, state.ErrUnknownAccountType
		}
		pool.amount = new(big.Int).Set(f.pool)
	default:
		return 0, state.ErrStateNotExist
	}
	return 0, nil
}

func (f *fakeStateReader) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, nil, err
	}
	if cfg.Namespace != state.AccountKVNamespace {
		return 0, nil, state.ErrStateNotExist
	}
	keys := make([][]byte, 0, len(f.accounts))
	states := make([][]byte, 0, len(f.accounts))
	for key, bal := range f.accounts {
		acc := state.Account{Balance: new(big.Int).Set(bal)}
		data, err := acc.Serialize()
		if err != nil {
			return 0, nil, err
		}
		keys = append(keys, key[:])
		states = append(states, data)
	}
	iter, err := state.NewIterator(keys, states)
	return 0, iter, err
}

func (f *fakeStateReader) setAccount(addr string, bal *big.Int) {
	f.accounts[hash.Hash160b([]byte(addr))] = bal
}

// testGenesis returns a small genesis config with initial accounts plus the
// rewarding seed, so the arithmetic is easy to reason about in tests.
func testGenesis(balStr, rewardingStr string, addrs []string) genesis.Genesis {
	g := genesis.TestDefault()
	g.InitBalanceMap = map[string]string{}
	for _, a := range addrs {
		g.InitBalanceMap[a] = balStr
	}
	g.Rewarding.InitBalanceStr = rewardingStr
	return g
}

func TestSupplyCheckAfterGenesis(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	addr := identityset.Address(0).String()
	g := testGenesis("1000", "200", []string{addr})

	fr := newFakeStateReader()
	fr.setAccount(addr, big.NewInt(1000))
	fr.fund = big.NewInt(200)

	o := NewObserver(fr, g, 0)
	res, err := o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(1000), res.Account)
	require.Equal(big.NewInt(200), res.Fund)
	require.Equal(big.NewInt(0), res.Pool)
	// Total == cap at genesis: 1000 + 200 == 1000 + 200.
	require.Equal(big.NewInt(1200), res.Total)
	require.Equal(big.NewInt(1200), res.Cap)
	require.Equal(0, res.Total.Cmp(res.Cap))
}

func TestSupplyConservedOnTransfer(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	a := identityset.Address(0).String()
	b := identityset.Address(1).String()
	g := testGenesis("500", "300", []string{a, b})

	fr := newFakeStateReader()
	fr.setAccount(a, big.NewInt(500))
	fr.setAccount(b, big.NewInt(500))
	fr.fund = big.NewInt(300)

	o := NewObserver(fr, g, 0)
	res, err := o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(1300), res.Total)

	// Simulate a plain transfer a->b: total account balances unchanged.
	fr.setAccount(a, big.NewInt(400))
	fr.setAccount(b, big.NewInt(600))
	res, err = o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(1000), res.Account)
	require.Equal(big.NewInt(1300), res.Total)
	require.NotEqual(1, res.Total.Cmp(res.Cap))
}

func TestSupplyConservedAcrossReservoirs(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	a := identityset.Address(0).String()
	g := testGenesis("500", "300", []string{a})

	fr := newFakeStateReader()
	fr.setAccount(a, big.NewInt(500))
	fr.fund = big.NewInt(300)

	o := NewObserver(fr, g, 0)
	res, err := o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(800), res.Total)

	// Stake: 200 moves from the account into the bucket pool -> total unchanged.
	fr.setAccount(a, big.NewInt(300))
	fr.pool = big.NewInt(200)
	res, err = o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(300), res.Account)
	require.Equal(big.NewInt(200), res.Pool)
	require.Equal(big.NewInt(800), res.Total)
	require.Equal(0, res.Total.Cmp(res.Cap))
}

func TestSupplyCheckCatchesExNihiloMint(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	a := identityset.Address(0).String()
	g := testGenesis("500", "300", []string{a})

	fr := newFakeStateReader()
	fr.setAccount(a, big.NewInt(500))
	fr.fund = big.NewInt(300)

	o := NewObserver(fr, g, 0)
	res, err := o.Check(ctx)
	require.NoError(err)
	require.Equal(0, res.Total.Cmp(res.Cap))

	// Attacker mints 10_000 out of thin air (Harmony "empty block" analog).
	fr.setAccount(a, big.NewInt(500+10000))
	res, err = o.Check(ctx)
	require.NoError(err)
	require.Equal(1, res.Total.Cmp(res.Cap), "total must exceed cap after ex-nihilo mint")
}

// sanity that the local deserializer mirrors the real on-chain proto schema.
func TestLocalDeserializersAgainstRealSchema(t *testing.T) {
	require := require.New(t)

	fundBytes, err := proto.Marshal(&rewardingpb.Fund{
		TotalBalance:     "1000",
		UnclaimedBalance: "400",
	})
	require.NoError(err)
	var f rewardingFund
	require.NoError(f.Deserialize(fundBytes))
	require.Equal(big.NewInt(1000), f.totalBalance)
	require.Equal(big.NewInt(400), f.unclaimedBalance)

	poolBytes, err := proto.Marshal(&stakingpb.TotalAmount{
		Amount: "12345",
		Count:  7,
	})
	require.NoError(err)
	var ta bucketPoolTotal
	require.NoError(ta.Deserialize(poolBytes))
	require.Equal(big.NewInt(12345), ta.amount)
	require.Equal(uint64(7), ta.count)
}

func TestSupplyCheckToleratesMissingReserveStates(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	a := identityset.Address(0).String()
	g := testGenesis("500", "300", []string{a})

	fr := newFakeStateReader()
	fr.setAccount(a, big.NewInt(500))
	fr.fund = big.NewInt(300)
	// Funds/pool present like on mainnet.
	o := NewObserver(fr, g, 0)
	res, err := o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(800), res.Total)

	// A pre-Greenland node has no bucket-pool state. Treating it as 0 must not
	// error and must not create a false violation (only under-counts -> sound).
	fr.hasPool = false
	res, err = o.Check(ctx)
	require.NoError(err)
	require.Equal(big.NewInt(500), res.Account)
	require.Equal(big.NewInt(300), res.Fund)
	require.Equal(big.NewInt(0), res.Pool)
	require.Equal(big.NewInt(800), res.Total)
	require.NotEqual(1, res.Total.Cmp(res.Cap))
}
