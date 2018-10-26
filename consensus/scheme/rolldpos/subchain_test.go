package rolldpos

import (
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/iotexproject/iotex-core/blockchain"
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

	addr := testaddress.Addrinfo["producer"]
	subAddr := testaddress.Addrinfo["echo"]
	blk := blockchain.Block{}
	blkpb := &iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   101,
			},
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   102,
			},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   103,
			},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   104,
			},
		},
	}
	blk.ConvertFromBlockPb(blkpb)
	txRoot := blk.CalculateTxRoot()
	blkpb.Header.TxRoot = txRoot[:]
	blkpb.Header.StateRoot = []byte("state root")
	blk = blockchain.Block{}
	blk.ConvertFromBlockPb(blkpb)
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
			explorerapi.PutSubChainBlockMerkelRoot{
				Name:  "state",
				Value: hex.EncodeToString(stateRoot[:]),
			},
			explorerapi.PutSubChainBlockMerkelRoot{
				Name:  "tx",
				Value: hex.EncodeToString(txRoot[:]),
			},
		},
	}

	exp := mock_explorer.NewMockExplorer(ctrl)
	exp.EXPECT().GetAddressDetails(addr.RawAddress).Return(explorerapi.AddressDetails{PendingNonce: 100}, nil).Times(1)
	exp.EXPECT().PutSubChainBlock(req).Times(1)

	putBlockToParentChain(exp, req.SubChainAddress, addr, &blk)
}
