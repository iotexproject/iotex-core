// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackBucketGuardEvent(t *testing.T) {
	r := require.New(t)
	sponsor := bytes.Repeat([]byte{0x11}, 20)
	counterparty := bytes.Repeat([]byte{0x22}, 20)
	topics, data, err := PackBucketGuardEvent(1, 2, 3, 4, sponsor, counterparty)
	r.NoError(err)
	r.Len(topics, 3)
	r.Equal("73a1cc2c5c34de8d095f5003eb096fa77f44357dd83dcea0225ca286942426ec", hex.EncodeToString(topics[0][:]))
	r.Equal("0000000000000000000000000000000000000000000000000000000000000001", hex.EncodeToString(topics[1][:]))
	r.Equal("0000000000000000000000000000000000000000000000000000000000000002", hex.EncodeToString(topics[2][:]))
	r.Len(data, 56)
	r.Equal("00000000000000030000000000000004", hex.EncodeToString(data[:16]))
	r.Equal(hex.EncodeToString(sponsor), hex.EncodeToString(data[16:36]))
	r.Equal(hex.EncodeToString(counterparty), hex.EncodeToString(data[36:]))

	_, _, err = PackBucketGuardEvent(1, 0, 0, 0, sponsor[:19], counterparty)
	r.Error(err)
	_, _, err = PackBucketGuardEvent(1, 0, 0, 0, sponsor, counterparty[:19])
	r.Error(err)
}
