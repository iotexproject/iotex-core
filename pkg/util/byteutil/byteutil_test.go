// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package byteutil

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"
)

func TestMust(t *testing.T) {
	t.Run("return identical output when given nil error", func(t *testing.T) {
		var expectedErr error

		result := Must([]byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}, expectedErr)

		require.Equal(t, []byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}, result)
	})

	t.Run("panics when an error was given", func(t *testing.T) {
		expectedErr := errors.New("an error was given")

		require.Panics(t, func() {
			Must([]byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}, expectedErr)
		}, expectedErr)
	})
}
