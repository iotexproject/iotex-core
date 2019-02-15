// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBasicHash(t *testing.T) {
	require := require.New(t)

	// basic hash test
	input := []byte("hello")
	hash := sha256.Sum256(input)
	hash = sha256.Sum256(hash[:])
	hello, _ := hex.DecodeString("9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50")
	require.Equal(hello, hash[:])
	t.Logf("sha256(sha256(\"hello\") = %x", hash)

	hash = blake2b.Sum256(input)
	hash = blake2b.Sum256(hash[:])
	hello, _ = hex.DecodeString("901c60ffffd77f743729f8fea0233c0b00223428b5192c2015f853562b45ce59")
	require.Equal(hello, hash[:])
	t.Logf("blake2b(blake2b(\"hello\") = %x", hash)
}

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
	require.Equal("27708bd46b8ea8026db2eb764d19ae5c4213d20b035290f1e799d2298717887d", hex.EncodeToString(hash[:]))

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	senderPubKey := ta.Keyinfo["producer"].PubKey
	require.NoError(t, blk.ConvertFromBlockPb(&iotextypes.BlockPb{
		Header: &iotextypes.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
			Pubkey:  keypair.PublicKeyToBytes(senderPubKey),
		},
		Actions: []*iotextypes.ActionPb{
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.TransferPb{},
					},
					Version: version.ProtocolVersion,
					Nonce:   101,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.TransferPb{},
					},
					Version: version.ProtocolVersion,
					Nonce:   102,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.VotePb{},
					},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.VotePb{},
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
	require.Equal(t, blk.Header.stateRoot, blk.StateRoot())
	require.Equal(t, blk.Header.receiptRoot, blk.ReceiptRoot())
}
