// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	explorerapi "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/test/mock/mock_explorer"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutBlockToParentChain(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := testaddress.Addrinfo["producer"].String()
	pubKey := testaddress.Keyinfo["producer"].PubKey
	priKey := testaddress.Keyinfo["producer"].PriKey
	subAddr := testaddress.Addrinfo["echo"].String()
	blk := block.Block{}
	blkpb := &iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    123456789,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: pubKey.Bytes(),
		},
		Actions: []*iotextypes.Action{
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.Transfer{},
					},
					Version: version.ProtocolVersion,
					Nonce:   101,
				},
				SenderPubKey: pubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.Transfer{},
					},
					Version: version.ProtocolVersion,
					Nonce:   102,
				},
				SenderPubKey: pubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				SenderPubKey: pubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   104,
				},
				SenderPubKey: pubKey.Bytes(),
			},
		},
	}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))
	txRoot := blk.CalculateTxRoot()
	blkpb.Header.Core.TxRoot = txRoot[:]
	blk = block.Block{}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))

	req := explorerapi.PutSubChainBlockRequest{
		Version:         1,
		Nonce:           100,
		SenderAddress:   addr,
		SenderPubKey:    pubKey.HexString(),
		GasLimit:        1000000,
		GasPrice:        "10",
		SubChainAddress: subAddr,
		Height:          123456789,
		Roots: []explorerapi.PutSubChainBlockMerkelRoot{
			{
				Name:  "tx",
				Value: hex.EncodeToString(txRoot[:]),
			},
		},
		Signature: "fe36ae0659698fe0c5a59cbd4fb29f69cb156a7956d6e9be85896ed6e8f2fcf13575750040aa18c437d0baf949964a7cea1574b4ee927074f29ccf6eb705cfbdce49244f9de72a00",
	}

	exp := mock_explorer.NewMockExplorer(ctrl)
	exp.EXPECT().GetAddressDetails(addr).Return(explorerapi.AddressDetails{PendingNonce: 100}, nil).Times(1)
	exp.EXPECT().PutSubChainBlock(gomock.Any()).Times(1).Do(func(in explorerapi.PutSubChainBlockRequest) {
		assert.Equal(t, in.Height, req.Height)
	})

	putBlockToParentChain(exp, req.SubChainAddress, priKey, addr, &blk)
}
