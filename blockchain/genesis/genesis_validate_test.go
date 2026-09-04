// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeGenesisYAML writes a genesis fragment to a temp file and returns its path.
func writeGenesisYAML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "genesis.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0600))
	return path
}

func TestNewRejectsUnsettleableEra(t *testing.T) {
	// A scheduled zanzibarHeight activates IIP-59, so the era length stops
	// being decorative. Zero makes IsEraBoundary false forever and one leaves no
	// room between a settlement and the freeze that supersedes its era window;
	// both fail silently at run time, which is what this rejects.
	for _, epochsPerEra := range []uint64{0, 1} {
		t.Run(fmt.Sprintf("epochsPerRewardEra=%d", epochsPerEra), func(t *testing.T) {
			r := require.New(t)
			path := writeGenesisYAML(t, fmt.Sprintf(`
blockchain:
  greenlandHeight: 999
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
rewarding:
  epochsPerRewardEra: %d
`, epochsPerEra))
			_, err := New(path)
			r.ErrorContains(err, "epochsPerRewardEra must be at least 2")
		})
	}
}

// _validDelegateProfile is any well-formed address; validate only checks that
// the string parses, not that a contract lives there.
const _validDelegateProfile = "io1drde9f483guaetl3w3w6n6y7yv80f8fael7qme"

func TestNewRejectsMissingDelegateProfileContract(t *testing.T) {
	// An unset DelegateProfile address does not disable commission routing, it
	// silently maximises it. FreezeCandidateRewardSnapshots still writes a
	// snapshot for every opted-in candidate, defaulted to 100% commission, and a
	// present snapshot is what flips onchainRewardEnabled on. So every opted-in
	// delegate takes the whole epoch reward at their owner address, no voter is
	// paid on chain, and the payout stops arriving at the reward address the
	// off-chain distributor watches. Nothing errors and nothing looks wrong.
	r := require.New(t)
	path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 999
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
poll:
  delegateProfileContractAddress: ""
rewarding:
  epochsPerRewardEra: 2
`)
	_, err := New(path)
	r.ErrorContains(err, "delegateProfileContractAddress must be set")
}

func TestNewRejectsMalformedIIP59ContractAddresses(t *testing.T) {
	r := require.New(t)

	t.Run("delegate profile", func(t *testing.T) {
		path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 999
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
poll:
  delegateProfileContractAddress: "not-an-address"
rewarding:
  epochsPerRewardEra: 2
`)
		_, err := New(path)
		r.ErrorContains(err, "delegateProfileContractAddress")
	})

	t.Run("auto deposit", func(t *testing.T) {
		// Empty is a supported mode for this one -- every voter share falls back
		// to a pull-claim credit -- but a typo must not silently select it.
		path := writeGenesisYAML(t, fmt.Sprintf(`
blockchain:
  greenlandHeight: 999
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
  autoDepositContractAddress: "not-an-address"
poll:
  delegateProfileContractAddress: %q
rewarding:
  epochsPerRewardEra: 2
`, _validDelegateProfile))
		_, err := New(path)
		r.ErrorContains(err, "autoDepositContractAddress")
	})
}

func TestNewAcceptsIIP59ContractAddresses(t *testing.T) {
	r := require.New(t)

	t.Run("auto deposit may stay empty", func(t *testing.T) {
		path := writeGenesisYAML(t, fmt.Sprintf(`
blockchain:
  greenlandHeight: 999
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
poll:
  delegateProfileContractAddress: %q
rewarding:
  epochsPerRewardEra: 2
`, _validDelegateProfile))
		g, err := New(path)
		r.NoError(err)
		r.Empty(g.AutoDepositContractAddress)
	})

	t.Run("unscheduled tolerates both unset", func(t *testing.T) {
		// The addresses are read only once IIP-59 is live. Requiring them on an
		// unscheduled chain would stop every existing network from starting.
		g, err := New(writeGenesisYAML(t, "blockchain:\n  zanzibarHeight: 18446744073709551615\n  zanzibarBetaHeight: 18446744073709551615\n  zanzibarGammaHeight: 18446744073709551615\n"+
			"poll:\n  delegateProfileContractAddress: \"\"\n"+
			"rewarding:\n  epochsPerRewardEra: 24\n"))
		r.NoError(err)
		r.Empty(g.DelegateProfileContractAddress)
	})
}

func TestNewAcceptsEra(t *testing.T) {
	r := require.New(t)

	t.Run("scheduled with a settleable era", func(t *testing.T) {
		path := writeGenesisYAML(t, fmt.Sprintf(`
blockchain:
  greenlandHeight: 999
  xinguHeight: 1000
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
poll:
  delegateProfileContractAddress: %q
rewarding:
  epochsPerRewardEra: 2
`, _validDelegateProfile))
		g, err := New(path)
		r.NoError(err)
		r.Equal(uint64(2), g.Rewarding.EpochsPerRewardEra)
	})

	t.Run("scheduled before Greenland", func(t *testing.T) {
		path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 1001
  xinguHeight: 999
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
rewarding:
  epochsPerRewardEra: 2
`)
		_, err := New(path)
		r.ErrorContains(err, "zanzibarHeight 1000 must not precede greenlandHeight 1001")
	})

	t.Run("scheduled before Xingu", func(t *testing.T) {
		path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 999
  xinguHeight: 1001
  zanzibarHeight: 1000
  zanzibarBetaHeight: 1000
rewarding:
  epochsPerRewardEra: 2
`)
		_, err := New(path)
		r.ErrorContains(err, "zanzibarHeight 1000 must not precede xinguHeight 1001")
	})

	t.Run("unscheduled tolerates any era", func(t *testing.T) {
		// zanzibarHeight defaults to MaxUint64: IIP-59 never activates, so
		// EpochsPerRewardEra is never read and must not block a node from
		// starting. Existing networks rely on this.
		path := writeGenesisYAML(t, `
blockchain:
  zanzibarHeight: 18446744073709551615
  zanzibarBetaHeight: 18446744073709551615
  zanzibarGammaHeight: 18446744073709551615
rewarding:
  epochsPerRewardEra: 0
`)
		g, err := New(path)
		r.NoError(err)
		r.Equal(uint64(0), g.Rewarding.EpochsPerRewardEra)
	})

	t.Run("defaults", func(t *testing.T) {
		g, err := New("")
		r.NoError(err)
		r.Equal(uint64(24), g.Rewarding.EpochsPerRewardEra)
	})
}
