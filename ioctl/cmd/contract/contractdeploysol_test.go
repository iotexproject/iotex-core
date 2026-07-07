// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestExpandSolSources(t *testing.T) {
	r := require.New(t)

	dir := t.TempDir()
	a := filepath.Join(dir, "A.sol")
	b := filepath.Join(dir, "B.sol")
	other := filepath.Join(dir, "notes.txt")
	r.NoError(os.WriteFile(a, []byte("// A"), 0o600))
	r.NoError(os.WriteFile(b, []byte("// B"), 0o600))
	r.NoError(os.WriteFile(other, []byte("x"), 0o600))
	// a nested directory should NOT be recursed into
	sub := filepath.Join(dir, "sub")
	r.NoError(os.Mkdir(sub, 0o755))
	r.NoError(os.WriteFile(filepath.Join(sub, "C.sol"), []byte("// C"), 0o600))

	t.Run("directory expands to its .sol files only", func(t *testing.T) {
		got, err := expandSolSources([]string{dir})
		r.NoError(err)
		r.ElementsMatch([]string{a, b}, got)
	})

	t.Run("explicit files are passed through unchanged (backward compatible)", func(t *testing.T) {
		got, err := expandSolSources([]string{a, b})
		r.NoError(err)
		r.Equal([]string{a, b}, got)
	})

	t.Run("mixed files and directories", func(t *testing.T) {
		got, err := expandSolSources([]string{a, sub})
		r.NoError(err)
		r.ElementsMatch([]string{a, filepath.Join(sub, "C.sol")}, got)
	})

	t.Run("missing source errors", func(t *testing.T) {
		_, err := expandSolSources([]string{filepath.Join(dir, "does-not-exist.sol")})
		r.Error(err)
	})
}

func TestAliasInitAmount(t *testing.T) {
	r := require.New(t)

	cmd := &cobra.Command{Use: "deploy"}
	_initialAmountFlag.RegisterCommand(cmd)
	aliasInitAmount(cmd)

	// --amount is accepted as an alias and writes through to the init-amount flag.
	r.NoError(cmd.Flags().Set("amount", "5"))
	r.Equal("5", _initialAmountFlag.Value().(string))

	// the historical --init-amount spelling keeps working.
	r.NoError(cmd.Flags().Set("init-amount", "7"))
	r.Equal("7", _initialAmountFlag.Value().(string))
}
