// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

const (
	_addrA = "io1uqhmnttmv0pg8prugxxn7d8ex9angrvfjfthxa"
	_addrB = "io1v3gkc49d5vwtdfdka2ekjl3h468egun8e43r7z"
	_addrC = "io1vrl48nsdm8jaujccd9cx4ve23cskr0ys6urx92"
)

// The in-repo defaults are the mainnet genesis config: New("") and
// iotex-bootstrap's genesis_mainnet.yaml produce the same Hash(). If a change to
// defaultConfig touches a field Hash() covers, the pinned constant stops
// matching mainnet and the guard silently stops guarding.
func TestMainnetGenesisHashIsPinned(t *testing.T) {
	r := require.New(t)
	cfg, err := New("")
	r.NoError(err)
	h := cfg.Hash()
	r.Equal(_mainnetGenesisHash, hex.EncodeToString(h[:]))
	r.True(cfg.IsMainnet())
}

func TestTestnetGrantsRejectedOnMainnet(t *testing.T) {
	r := require.New(t)
	cfg, err := New("")
	r.NoError(err)
	// no grants: mainnet loads as usual
	r.NoError(cfg.ValidateTestnetGrants())

	cfg.TestnetGrants = []TestnetGrant{{
		Height:     100,
		Recipients: []GrantRecipient{{Address: _addrA, Amount: "1"}},
	}}
	// still recognisable as mainnet after appending grants, which is the point
	// of leaving them out of Hash()
	r.True(cfg.IsMainnet())
	r.ErrorContains(cfg.ValidateTestnetGrants(), "must not be used on mainnet")
}

func testnetGenesis(grants ...TestnetGrant) Genesis {
	g := TestDefault()
	g.TestnetGrants = grants
	return g
}

func TestValidateTestnetGrants(t *testing.T) {
	oneIotx := unit.ConvertIotxToRau(1).String()

	for _, c := range []struct {
		name   string
		grants []TestnetGrant
		errMsg string
	}{
		{
			name:   "no grants",
			grants: nil,
		},
		{
			name: "valid",
			grants: []TestnetGrant{
				{Height: 100, Recipients: []GrantRecipient{
					{Address: _addrA, Amount: oneIotx},
					{Address: _addrB, Amount: oneIotx},
				}},
				{Height: 200, Recipients: []GrantRecipient{{Address: _addrC, Amount: oneIotx}}},
			},
		},
		{
			name:   "zero height",
			grants: []TestnetGrant{{Height: 0, Recipients: []GrantRecipient{{Address: _addrA, Amount: oneIotx}}}},
			errMsg: "height must be non-zero",
		},
		{
			name: "heights not increasing",
			grants: []TestnetGrant{
				{Height: 200, Recipients: []GrantRecipient{{Address: _addrA, Amount: oneIotx}}},
				{Height: 200, Recipients: []GrantRecipient{{Address: _addrB, Amount: oneIotx}}},
			},
			errMsg: "strictly increasing",
		},
		{
			name:   "no recipients",
			grants: []TestnetGrant{{Height: 100}},
			errMsg: "has no recipients",
		},
		{
			name: "duplicate recipient",
			grants: []TestnetGrant{{Height: 100, Recipients: []GrantRecipient{
				{Address: _addrA, Amount: oneIotx},
				{Address: _addrA, Amount: oneIotx},
			}}},
			errMsg: "more than once",
		},
		{
			name:   "bad address",
			grants: []TestnetGrant{{Height: 100, Recipients: []GrantRecipient{{Address: "not-an-address", Amount: oneIotx}}}},
			errMsg: "invalid address",
		},
		{
			name:   "unparsable amount",
			grants: []TestnetGrant{{Height: 100, Recipients: []GrantRecipient{{Address: _addrA, Amount: "1.5"}}}},
			errMsg: "unparsable amount",
		},
		{
			name:   "zero amount",
			grants: []TestnetGrant{{Height: 100, Recipients: []GrantRecipient{{Address: _addrA, Amount: "0"}}}},
			errMsg: "must be positive",
		},
		{
			name:   "negative amount",
			grants: []TestnetGrant{{Height: 100, Recipients: []GrantRecipient{{Address: _addrA, Amount: "-1"}}}},
			errMsg: "must be positive",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			g := testnetGenesis(c.grants...)
			err := g.ValidateTestnetGrants()
			if c.errMsg == "" {
				r.NoError(err)
				return
			}
			r.ErrorContains(err, c.errMsg)
		})
	}
}

func TestGrantsAtHeight(t *testing.T) {
	r := require.New(t)
	g := testnetGenesis(
		[]TestnetGrant{
			// not in sorted address order: configured order is applied order
			{Height: 100, Recipients: []GrantRecipient{
				{Address: _addrC, Amount: "3"},
				{Address: _addrA, Amount: "1"},
			}},
			{Height: 200, Recipients: []GrantRecipient{{Address: _addrB, Amount: "2"}}},
		}...,
	)
	r.NoError(g.ValidateTestnetGrants())

	addrs, amounts, err := g.GrantsAtHeight(100)
	r.NoError(err)
	r.Len(addrs, 2)
	r.Equal(_addrC, addrs[0].String())
	r.Equal(_addrA, addrs[1].String())
	r.Equal("3", amounts[0].String())
	r.Equal("1", amounts[1].String())

	addrs, amounts, err = g.GrantsAtHeight(200)
	r.NoError(err)
	r.Len(addrs, 1)
	r.Equal(_addrB, addrs[0].String())
	r.Equal("2", amounts[0].String())

	for _, h := range []uint64{0, 1, 99, 101, 199, 201} {
		addrs, amounts, err = g.GrantsAtHeight(h)
		r.NoError(err)
		r.Nil(addrs)
		r.Nil(amounts)
	}

	noGrants := TestDefault()
	addrs, amounts, err = noGrants.GrantsAtHeight(100)
	r.NoError(err)
	r.Nil(addrs)
	r.Nil(amounts)
}

// The feature is driven from a YAML file an operator edits, so the tags have to
// round-trip, and a bad file has to be rejected by New() rather than at the
// activation height.
func TestTestnetGrantsFromYAML(t *testing.T) {
	r := require.New(t)
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "genesis.yaml")
		r.NoError(os.WriteFile(p, []byte(body), 0600))
		return p
	}

	// a different timestamp is the smallest change that moves a config off the
	// mainnet hash
	const header = "blockchain:\n  timestamp: 1571036400\n"

	t.Run("absent", func(t *testing.T) {
		cfg, err := New(write(header))
		r.NoError(err)
		r.Empty(cfg.TestnetGrants)
	})

	t.Run("parsed", func(t *testing.T) {
		cfg, err := New(write(header + `account:
  testnetGrants:
    - height: 44500000
      recipients:
        - address: ` + _addrA + `
          amount: "1200100000000000000000000"
        - address: ` + _addrB + `
          amount: "100000000000000000000"
`))
		r.NoError(err)
		r.Len(cfg.TestnetGrants, 1)
		r.EqualValues(44500000, cfg.TestnetGrants[0].Height)
		r.Equal([]GrantRecipient{
			{Address: _addrA, Amount: "1200100000000000000000000"},
			{Address: _addrB, Amount: "100000000000000000000"},
		}, cfg.TestnetGrants[0].Recipients)

		addrs, amounts, err := cfg.GrantsAtHeight(44500000)
		r.NoError(err)
		r.Len(addrs, 2)
		r.Equal("1200100000000000000000000", amounts[0].String())
	})

	t.Run("invalid is rejected at load", func(t *testing.T) {
		_, err := New(write(header + `account:
  testnetGrants:
    - height: 0
      recipients:
        - address: ` + _addrA + `
          amount: "1"
`))
		r.ErrorContains(err, "height must be non-zero")
	})

	t.Run("mainnet is rejected at load", func(t *testing.T) {
		_, err := New(write(`account:
  testnetGrants:
    - height: 44500000
      recipients:
        - address: ` + _addrA + `
          amount: "1"
`))
		r.ErrorContains(err, "must not be used on mainnet")
	})
}

// Nodes with and without the grant must stay on the same p2p network, so the
// ones missing it fail loudly on the delta state digest at the activation height
// rather than quietly forming a second network at startup.
func TestTestnetGrantsDoNotChangeGenesisHash(t *testing.T) {
	r := require.New(t)
	plain := TestDefault()
	withGrant := testnetGenesis(TestnetGrant{
		Height:     100,
		Recipients: []GrantRecipient{{Address: _addrA, Amount: "1"}},
	})
	r.Equal(plain.Hash(), withGrant.Hash())
}
