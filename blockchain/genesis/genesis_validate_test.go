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
	// A scheduled toBeEnabledHeight activates IIP-59, so the era length stops
	// being decorative. Zero makes IsEraBoundary false forever and one leaves no
	// room between a settlement and the freeze that supersedes its era window;
	// both fail silently at run time, which is what this rejects.
	for _, epochsPerEra := range []uint64{0, 1} {
		t.Run(fmt.Sprintf("epochsPerRewardEra=%d", epochsPerEra), func(t *testing.T) {
			r := require.New(t)
			path := writeGenesisYAML(t, fmt.Sprintf(`
blockchain:
  greenlandHeight: 999
  toBeEnabledHeight: 1000
rewarding:
  epochsPerRewardEra: %d
`, epochsPerEra))
			_, err := New(path)
			r.ErrorContains(err, "epochsPerRewardEra must be at least 2")
		})
	}
}

func TestNewAcceptsEra(t *testing.T) {
	r := require.New(t)

	t.Run("scheduled with a settleable era", func(t *testing.T) {
		path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 999
  toBeEnabledHeight: 1000
rewarding:
  epochsPerRewardEra: 2
`)
		g, err := New(path)
		r.NoError(err)
		r.Equal(uint64(2), g.Rewarding.EpochsPerRewardEra)
	})

	t.Run("scheduled before Greenland", func(t *testing.T) {
		path := writeGenesisYAML(t, `
blockchain:
  greenlandHeight: 1001
  toBeEnabledHeight: 1000
rewarding:
  epochsPerRewardEra: 2
`)
		_, err := New(path)
		r.ErrorContains(err, "toBeEnabledHeight 1000 must not precede greenlandHeight 1001")
	})

	t.Run("unscheduled tolerates any era", func(t *testing.T) {
		// toBeEnabledHeight defaults to MaxUint64: IIP-59 never activates, so
		// EpochsPerRewardEra is never read and must not block a node from
		// starting. Existing networks rely on this.
		path := writeGenesisYAML(t, `
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
