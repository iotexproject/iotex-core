// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestGenesisBlock(t *testing.T) {
	r := require.New(t)

	g := genesis.Default
	genesis.SetGenesisTimestamp(g.Timestamp)
	blk := GenesisBlock()
	r.EqualValues(version.ProtocolVersion, blk.Version())
	r.Zero(blk.Height())
	r.Equal(genesis.Default.Timestamp, blk.Timestamp().Unix())
	r.Equal(hash.ZeroHash256, blk.PrevHash())
	r.Equal(hash.ZeroHash256, blk.TxRoot())
	r.Equal(hash.ZeroHash256, blk.DeltaStateDigest())
	r.Equal(hash.ZeroHash256, blk.ReceiptRoot())

	r.Equal(hash.ZeroHash256, GenesisHash())
	LoadGenesisHash(&g)
	h := GenesisHash()
	r.Equal("e825cf0df9c72bc8cc74b23485af65c4d8a8ba691db335e44de7362cd86bac7f", hex.EncodeToString(h[:]))
}
