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
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestGenesisBlock(t *testing.T) {
	r := require.New(t)

	config.SetGenesisTimestamp(config.Default.Genesis.Timestamp)
	blk := GenesisBlock()
	r.EqualValues(version.ProtocolVersion, blk.Version())
	r.Zero(blk.Height())
	r.Equal(genesis.Default.Timestamp, blk.Timestamp().Unix())
	r.Equal(hash.ZeroHash256, blk.PrevHash())
	r.Equal(hash.ZeroHash256, blk.TxRoot())
	r.Equal(hash.ZeroHash256, blk.DeltaStateDigest())
	r.Equal(hash.ZeroHash256, blk.ReceiptRoot())

	r.Equal(hash.ZeroHash256, GenesisHash())
	LoadGenesisHash()
	h := blk.HashBlock()
	r.Equal(GenesisHash(), h)
	r.Equal("ab7d006c1f7a9345ad05eef1b4f062814a176c25c7558052e18896844ee71edb", hex.EncodeToString(h[:]))
	r.NoError(VerifyBlock(blk))
}
