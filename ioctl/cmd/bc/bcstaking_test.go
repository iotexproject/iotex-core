// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGenesisHash(t *testing.T) {
	r := require.New(t)
	const mainnet = "b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b"
	h, err := parseGenesisHash(mainnet)
	r.NoError(err)
	r.Equal(mainnet, hex.EncodeToString(h[:]))

	for _, bad := range []string{"", "zz", mainnet[:62], mainnet + "00"} {
		_, err := parseGenesisHash(bad)
		r.Error(err, bad)
	}
}
