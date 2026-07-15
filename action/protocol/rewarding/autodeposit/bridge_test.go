// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// mainnetContract is the pinned mainnet AutoDeposit deployment. Tests use
// it verbatim so a rename in production would be caught here too.
const mainnetContract = "io108ckwzlzpkhva7cnfceajlu7wu6ql5kq95uat9"

func TestNew(t *testing.T) {
	r := require.New(t)

	t.Run("empty contract rejected", func(t *testing.T) {
		_, err := New("")
		r.ErrorIs(err, ErrEmptyContractAddress)
	})

	t.Run("garbage address rejected", func(t *testing.T) {
		_, err := New("not-a-bech32-address")
		r.Error(err)
	})

	t.Run("valid address accepted", func(t *testing.T) {
		b, err := New(mainnetContract)
		r.NoError(err)
		r.Equal(mainnetContract, b.Contract())
	})
}
