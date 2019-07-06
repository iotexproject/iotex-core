// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

func TestNewBlockProposal(t *testing.T) {
	require := require.New(t)
	bp := newBlockProposal(nil, nil)
	require.NotNil(bp)
	require.Panics(func() { bp.Height() }, "block is nil")
	require.Panics(func() { bp.Proto() }, "block is nil")
	require.Panics(func() { bp.Hash() }, "block is nil")
	require.Panics(func() { bp.ProposerAddress() }, "block is nil")
	b := getBlock(t)
	bp2 := newBlockProposal(&b, nil)
	require.NotNil(bp2)
	require.Equal(uint64(123), bp2.Height())
	require.Equal("io1vdtfpzkwpyngzvx7u2mauepnzja7kd5rryp0sg", bp2.ProposerAddress())

	h, err := bp2.Hash()
	require.NoError(err)
	require.Equal(32, len(h))

	pro, err := bp2.Proto()
	require.NoError(err)

	bp3 := newBlockProposal(nil, nil)
	require.NoError(bp3.LoadProto(pro))
	pro3, err := bp3.Proto()
	require.NoError(err)
	require.EqualValues(pro, pro3)
}
func getBlock(t *testing.T) block.Block {
	require := require.New(t)
	ts := &timestamp.Timestamp{Seconds: 10, Nanos: 10}
	hcore := &iotextypes.BlockHeaderCore{
		Version:          1,
		Height:           123,
		Timestamp:        ts,
		PrevBlockHash:    []byte(""),
		TxRoot:           []byte(""),
		DeltaStateDigest: []byte(""),
		ReceiptRoot:      []byte(""),
	}
	header := block.Header{}
	pk, err := hex.DecodeString("04ea8046cf8dc5bc9cda5f2e83e5d2d61932ad7e0e402b4f4cb65b58e9618891f54cba5cfcda873351ad9da1f5a819f54bba9e8343f2edd1ad34dcf7f35de552f3")
	require.NoError(err)
	require.NoError(header.LoadFromBlockHeaderProto(&iotextypes.BlockHeader{Core: hcore, ProducerPubkey: pk}))

	b := block.Block{Header: header}
	return b
}
