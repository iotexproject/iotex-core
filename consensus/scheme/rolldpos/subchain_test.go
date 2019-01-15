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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	explorerapi "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_explorer"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutBlockToParentChain(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := testaddress.IotxAddrinfo["producer"]
	subAddr := testaddress.IotxAddrinfo["echo"]
	blk := block.Block{}
	blkpb := &iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       addr.RawAddress,
				SenderPubKey: addr.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       addr.RawAddress,
				SenderPubKey: addr.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        102,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       addr.RawAddress,
				SenderPubKey: addr.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       addr.RawAddress,
				SenderPubKey: addr.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        104,
			},
		},
	}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))
	txRoot := blk.CalculateTxRoot()
	blkpb.Header.TxRoot = txRoot[:]
	blkpb.Header.StateRoot = []byte("state root")
	blk = block.Block{}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))
	stateRoot := blk.StateRoot()

	req := explorerapi.PutSubChainBlockRequest{
		Version:         1,
		Nonce:           100,
		SenderAddress:   addr.RawAddress,
		SenderPubKey:    hex.EncodeToString(addr.PublicKey[:]),
		GasLimit:        1000000,
		GasPrice:        "10",
		SubChainAddress: subAddr.RawAddress,
		Height:          123456789,
		Roots: []explorerapi.PutSubChainBlockMerkelRoot{
			{
				Name:  "state",
				Value: hex.EncodeToString(stateRoot[:]),
			},
			{
				Name:  "tx",
				Value: hex.EncodeToString(txRoot[:]),
			},
		},
		Signature: "fe36ae0659698fe0c5a59cbd4fb29f69cb156a7956d6e9be85896ed6e8f2fcf13575750040aa18c437d0baf949964a7cea1574b4ee927074f29ccf6eb705cfbdce49244f9de72a00",
	}

	exp := mock_explorer.NewMockExplorer(ctrl)
	exp.EXPECT().GetAddressDetails(addr.RawAddress).Return(explorerapi.AddressDetails{PendingNonce: 100}, nil).Times(1)
	exp.EXPECT().PutSubChainBlock(gomock.Any()).Times(1).Do(func(in explorerapi.PutSubChainBlockRequest) {
		assert.Equal(t, in.Height, req.Height)
	})

	putBlockToParentChain(exp, req.SubChainAddress, addr.PublicKey, addr.PrivateKey, addr.RawAddress, &blk)
}
