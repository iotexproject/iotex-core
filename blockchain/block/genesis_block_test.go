// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

func TestGenesisBlock(t *testing.T) {
	r := require.New(t)

	g := genesis.Default
	genesis.SetGenesisTimestamp(g.Timestamp)
	blk := GenesisBlock()
	r.EqualValues(version.ProtocolVersion, blk.Version())
	r.Zero(blk.Height())
	r.Equal(g.Timestamp, blk.Timestamp().Unix())
	r.Equal(hash.ZeroHash256, blk.PrevHash())
	r.Equal(hash.ZeroHash256, blk.TxRoot())
	r.Equal(hash.ZeroHash256, blk.DeltaStateDigest())
	r.Equal(hash.ZeroHash256, blk.ReceiptRoot())

	r.Equal(hash.ZeroHash256, GenesisHash())
	LoadGenesisHash(&g)
	h := GenesisHash()
	r.Equal("b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b", hex.EncodeToString(h[:]))
}
