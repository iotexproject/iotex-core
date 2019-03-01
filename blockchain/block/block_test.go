// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestMerkle(t *testing.T) {
	require := require.New(t)

	producerAddr := ta.Addrinfo["producer"].String()
	producerPubKey := ta.Keyinfo["producer"].PubKey
	producerPriKey := ta.Keyinfo["producer"].PriKey
	amount := uint64(50 << 22)
	// create testing transactions
	selp0, err := testutil.SignedTransfer(producerAddr, producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp3, err := testutil.SignedTransfer(ta.Addrinfo["charlie"].String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp4, err := testutil.SignedTransfer(ta.Addrinfo["echo"].String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	// create block using above 5 tx and verify merkle
	block := NewBlockDeprecated(
		0,
		0,
		hash.ZeroHash256,
		testutil.TimestampNow(),
		producerPubKey,
		[]action.SealedEnvelope{selp0, selp1, selp2, selp3, selp4},
	)
	hash := block.CalculateTxRoot()
	require.Equal("eb5cb75ae199d96de7c1cd726d5e1a3dff15022ed7bdc914a3d8b346f1ef89c9", hex.EncodeToString(hash[:]))

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	senderPubKey := ta.Keyinfo["producer"].PubKey
	require.NoError(t, blk.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version: version.ProtocolVersion,
				Height:  123456789,
			},
			ProducerPubkey: keypair.PublicKeyToBytes(senderPubKey),
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
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.Transfer{},
					},
					Version: version.ProtocolVersion,
					Nonce:   102,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   104,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
		},
	}))

	blk.Header.txRoot = blk.CalculateTxRoot()
	blk.Header.receiptRoot = hash.Hash256b(([]byte)("test"))

	raw, err := blk.Serialize()
	require.Nil(t, err)

	var newblk Block
	err = newblk.Deserialize(raw)
	require.Nil(t, err)

	blockBytes := blk.ByteStream()
	require.True(t, len(blockBytes) > 0)

	require.Equal(t, uint64(123456789), newblk.Header.height)

	require.Equal(t, uint64(101), newblk.Actions[0].Nonce())
	require.Equal(t, uint64(102), newblk.Actions[1].Nonce())

	require.Equal(t, uint64(103), newblk.Actions[2].Nonce())
	require.Equal(t, uint64(104), newblk.Actions[3].Nonce())

	require.Equal(t, blk.Header.txRoot, blk.TxRoot())
	require.Equal(t, blk.Header.receiptRoot, blk.ReceiptRoot())
}
