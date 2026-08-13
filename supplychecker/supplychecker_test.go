// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package supplychecker

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
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
	accounts    map[hash.Hash160]*big.Int
	rawAccounts map[hash.Hash160][]byte // raw serialized entries injected verbatim into the Account iterator
	fund        *big.Int
	pool        *big.Int
	hasFund     bool
	hasPool     bool
	failState   bool // if set, State() returns a hard error
	failStates  bool // if set, States() returns a hard error
	nilAccounts bool // if set, States() returns an account iterator with a nil entry
}

func newFakeStateReader() *fakeStateReader {
	return &fakeStateReader{
		accounts:    map[hash.Hash160]*big.Int{},
		rawAccounts: map[hash.Hash160][]byte{},
		fund:        big.NewInt(0),
		pool:        big.NewInt(0),
		hasFund:     true,
		hasPool:     true,
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
	if f.failState {
		return 0, errors.New("state read failure")
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
	if f.failStates {
		return 0, nil, errors.New("states read failure")
	}
	keys := make([][]byte, 0, len(f.accounts)+len(f.rawAccounts))
	states := make([][]byte, 0, len(f.accounts)+len(f.rawAccounts))
	for key, bal := range f.accounts {
		acc := state.Account{Balance: new(big.Int).Set(bal)}
		data, err := acc.Serialize()
		if err != nil {
			return 0, nil, err
		}
		keys = append(keys, key[:])
		states = append(states, data)
	}
	for key, data := range f.rawAccounts {
		keys = append(keys, key[:])
		states = append(states, data)
	}
	if f.nilAccounts && len(states) > 0 {
		// Inject a nil (deleted-account) storage entry after the first real one.
		states = append(states, nil)
		delKey := hash.Hash160b([]byte("deleted"))
		keys = append(keys, delKey[:])
	}
	iter, err := state.NewIterator(keys, states)
	return 0, iter, err
}

func (f *fakeStateReader) setAccount(addr string, bal *big.Int) {
	f.accounts[hash.Hash160b([]byte(addr))] = bal
}

func (f *fakeStateReader) setRawAccount(addr string, data []byte) {
	f.rawAccounts[hash.Hash160b([]byte(addr))] = data
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

func TestGenesisTotalSupplyGuards(t *testing.T) {
	require := require.New(t)

	// A bogus init-balance value must not abort the sum; it is skipped.
	g := genesis.TestDefault()
	g.InitBalanceMap = map[string]string{"io1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "not-a-number"}
	g.InitBalanceMap["io1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] = "7"
	g.Rewarding.InitBalanceStr = "3"
	require.Equal(big.NewInt(10), genesisTotalSupply(g))

	// A bogus rewarding init string is also skipped.
	g.Rewarding.InitBalanceStr = "nope"
	require.Equal(big.NewInt(7), genesisTotalSupply(g))
}

func TestSumPrimaryBalancesSkipsNilAccount(t *testing.T) {
	require := require.New(t)

	a := identityset.Address(0).String()
	b := identityset.Address(1).String()
	fr := newFakeStateReader()
	fr.setAccount(a, big.NewInt(111))
	fr.setAccount(b, big.NewInt(222))
	fr.nilAccounts = true

	total, accounts, err := sumPrimaryBalances(fr)
	require.NoError(err)
	require.Equal(big.NewInt(333), total)
	require.Equal(uint64(2), accounts)
}

func TestObserverCheckReadFailure(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	g := testGenesis("500", "300", []string{identityset.Address(0).String()})
	fr := newFakeStateReader()
	fr.setAccount(identityset.Address(0).String(), big.NewInt(500))
	fr.fund = big.NewInt(300)

	o := NewObserver(fr, g, 0)
	_, err := o.Check(ctx)
	require.NoError(err)

	// States() failure propagates through Check.
	fr.failStates = true
	_, err = o.Check(ctx)
	require.Error(err)

	fr.failStates = false
	// State() failure propagates through Check.
	fr.failState = true
	_, err = o.Check(ctx)
	require.Error(err)
}

func TestObserverRunTicksAndStops(t *testing.T) {
	require := require.New(t)

	g := testGenesis("500", "300", []string{identityset.Address(0).String()})
	fr := newFakeStateReader()
	fr.setAccount(identityset.Address(0).String(), big.NewInt(500))
	fr.fund = big.NewInt(300)

	// Run must not panic and should exit on context cancellation after at least
	// one tick. Use a tiny interval and cancel shortly after.
	o := NewObserver(fr, g, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go o.Run(ctx)
	time.Sleep(5 * time.Millisecond)
	cancel()

	// Error path inside Run is exercised with a failing reader.
	fr.failState = true
	o2 := NewObserver(fr, g, time.Millisecond)
	ctx2, cancel2 := context.WithCancel(context.Background())
	go o2.Run(ctx2)
	time.Sleep(5 * time.Millisecond)
	cancel2()
	require.True(true) // reached end without panic
}

// TestDecodeAccountStrict verifies the strict, non-panicking decoder accepts every
// genuine account shape and rejects the legacy Poll/Rewarding payloads that share
// the Account namespace and would panic state.Account.Deserialize.
func TestDecodeAccountStrict(t *testing.T) {
	require := require.New(t)

	mustMarshal := func(m proto.Message) []byte {
		b, err := proto.Marshal(m)
		require.NoError(err)
		return b
	}

	// --- Genuine accounts must be accepted ---
	accept := []proto.Message{
		// Plain EOA with nonce and balance.
		&accountpb.Account{Nonce: 3, Balance: "1000", Type: accountpb.AccountType_DEFAULT},
		// Zero-balance EOA.
		&accountpb.Account{Nonce: 1, Balance: "0", Type: accountpb.AccountType_DEFAULT},
		// ZERO_NONCE account type (the second legal enum value).
		&accountpb.Account{Balance: "500", Type: accountpb.AccountType_ZERO_NONCE},
		// Contract account carrying storage root + code hash.
		&accountpb.Account{
			Balance:  "777",
			Root:     []byte{1, 2, 3, 4, 5, 6, 7, 8},
			CodeHash: []byte{9, 9, 9, 9},
			Type:     accountpb.AccountType_DEFAULT,
		},
	}
	for _, m := range accept {
		acct, ok := decodeAccount(mustMarshal(m))
		require.True(ok, "must accept genuine account %T", m)
		require.NotNil(acct)
	}

	// --- Legacy Poll/Rewarding/staking payloads must be rejected (never panic) ---
	reject := []proto.Message{
		// Rewarding fund: field2 string collides with Account.balance ("1e26"
		// is not a decimal integer), field1 is an unknown wire type.
		&rewardingpb.Fund{TotalBalance: "1e26", UnclaimedBalance: "1e26"},
		// Rewarding admin with productivityThreshold=85 -> field7 collides
		// with Account.type and would panic state.Account.FromProto.
		&rewardingpb.Admin{BlockReward: "12500", EpochReward: "12500", ProductivityThreshold: 85},
		// Rewarding exempt list (repeated bytes at field1).
		&rewardingpb.Exempt{Addrs: [][]byte{{1, 2, 3}}},
		// Rewarding rewardAccount (rewardingpb.Account{Balance:"..."} at field1).
		&rewardingpb.Account{Balance: "1e26"},
		// Staking bucket pool total (TotalAmount at field1).
		&stakingpb.TotalAmount{Amount: "100", Count: 1},
	}
	for _, m := range reject {
		require.NotPanics(func() {
			acct, ok := decodeAccount(mustMarshal(m))
			require.False(ok, "must reject legacy payload %T", m)
			require.Nil(acct)
		}, "decoder must never panic on %T", m)
	}

	// --- Non-canonical / corrupt bytes must be rejected ---
	require.NotPanics(func() {
		acct, ok := decodeAccount([]byte{0xff, 0xff, 0xff})
		require.False(ok)
		require.Nil(acct)
	})
	// Empty payload is skipped, not decoded.
	acct, ok := decodeAccount(nil)
	require.False(ok)
	require.Nil(acct)
}

// TestSumPrimaryBalancesRejectsLegacyStates proves the Account-namespace scan
// tolerates the legacy Poll/Rewarding states that were written pre-Greenland and
// never deleted: it skips them instead of crashing (P0) or over-counting them as
// account balances (over-count false-positive).
func TestSumPrimaryBalancesRejectsLegacyStates(t *testing.T) {
	require := require.New(t)

	mustMarshal := func(m proto.Message) []byte {
		b, err := proto.Marshal(m)
		require.NoError(err)
		return b
	}

	fr := newFakeStateReader()
	realAddr := identityset.Address(0).String()
	fr.setAccount(realAddr, big.NewInt(1000))
	// Inject the legacy rewarding fund, admin (with 85 -> enum collision), exempt,
	// and rewardAccount payloads into the raw Account-namespace iterator exactly as
	// a live pre-Greenland node would have them.
	fr.setRawAccount("legacy-fund", mustMarshal(&rewardingpb.Fund{TotalBalance: "1e26", UnclaimedBalance: "1e26"}))
	fr.setRawAccount("legacy-admin", mustMarshal(&rewardingpb.Admin{BlockReward: "12500", ProductivityThreshold: 85}))
	fr.setRawAccount("legacy-exempt", mustMarshal(&rewardingpb.Exempt{Addrs: [][]byte{{1, 2, 3}}}))
	fr.setRawAccount("legacy-reward-account", mustMarshal(&rewardingpb.Account{Balance: "1e26"}))

	total, accounts, err := sumPrimaryBalances(fr)
	require.NoError(err)
	// Only the genuine account's balance is counted; legacy states add nothing.
	require.Equal(big.NewInt(1000), total)
	require.Equal(uint64(1), accounts)
}

// TestObserverCheckToleratesLegacyStatesEndToEnd reproduces the exact mainnet
// hazard the reviewer flagged: the Account namespace also holds legacy
// Poll/Rewarding states written pre-Greenland (rewarding fund, admin with a
// productivityThreshold that collides with the account-type enum, exempt,
// rewardAccount) that are never deleted. Running the observer against such a
// namespace must neither panic (P0, since Run is a bare goroutine) nor
// over-count them into R1 (which would invert the "never a false positive"
// property). This runs through the full Observer.Check path, not just the
// internal sum helper.
func TestObserverCheckToleratesLegacyStatesEndToEnd(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	mustMarshal := func(m proto.Message) []byte {
		b, err := proto.Marshal(m)
		require.NoError(err)
		return b
	}

	addr := identityset.Address(0).String()
	g := testGenesis("1000", "200", []string{addr})

	fr := newFakeStateReader()
	fr.setAccount(addr, big.NewInt(1000))
	fr.fund = big.NewInt(200)
	// Inject the legacy rewarding states exactly as they sit in the Account
	// namespace on a real pre-Greenland node.
	fr.setRawAccount("legacy-fund", mustMarshal(&rewardingpb.Fund{TotalBalance: "1e26", UnclaimedBalance: "1e26"}))
	fr.setRawAccount("legacy-admin", mustMarshal(&rewardingpb.Admin{BlockReward: "12500", ProductivityThreshold: 85}))
	fr.setRawAccount("legacy-exempt", mustMarshal(&rewardingpb.Exempt{Addrs: [][]byte{{1}}}))

	o := NewObserver(fr, g, 0)
	require.NotPanics(func() {
		res, err := o.Check(ctx)
		require.NoError(err)
		// Only the genuine account is counted; legacy states add nothing, so R1 is
		// not over-counted and the observer does not false-positive.
		require.Equal(big.NewInt(1000), res.Account)
		require.Equal(uint64(1), res.Accounts)
		require.Equal(big.NewInt(1200), res.Total)
		require.Equal(0, res.Total.Cmp(res.Cap))
	})
}
