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

func TestUint32(t *testing.T) {
	input := uint32(31415926)
	t.Run("convert uint32 to bytes", func(t *testing.T) {
		expectedValue := []uint8([]byte{0x76, 0x5e, 0xdf, 0x1})
		result := Uint32ToBytes(input)
		require.Equal(t, expectedValue, result)
	})

	t.Run("converts a uint32 to 4 bytes in big-endian", func(t *testing.T) {
		expectedValue := []uint8([]byte{0x1, 0xdf, 0x5e, 0x76})
		result := Uint32ToBytesBigEndian(input)
		require.Equal(t, expectedValue, result)
	})
}

func TestUint64(t *testing.T) {
	input := uint64(1844674407370955161)
	byteInput := []byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}
	t.Run("convert uint64 to bytes", func(t *testing.T) {
		result := Uint64ToBytes(input)
		require.Equal(t, byteInput, result)
	})

	t.Run("convert bytes to unit64", func(t *testing.T) {
		result := BytesToUint64(byteInput)
		require.Equal(t, input, result)
	})

	t.Run("converts a uint64 to 8 bytes in big-endian", func(t *testing.T) {
		expectedValue := []uint8([]byte{0x19, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99})
		result := Uint64ToBytesBigEndian(input)
		require.Equal(t, expectedValue, result)
	})

	t.Run("converts 8 bytes to uint64 in big-endian", func(t *testing.T) {
		expectedValue := uint64(11068046444225730841)
		result := BytesToUint64BigEndian(byteInput)
		require.Equal(t, expectedValue, result)
	})
}

func TestMust(t *testing.T) {
	t.Run("return identical output when given nil error", func(t *testing.T) {
		b := []byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}
		result := Must(b, nil)
		require.Equal(t, b, result)
	})
	t.Run("panics when an error was given", func(t *testing.T) {
		expectedErr := errors.New("an error was given")
		require.Panics(t, func() {
			Must([]byte{0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x19}, expectedErr)
		}, expectedErr)
	})
}
