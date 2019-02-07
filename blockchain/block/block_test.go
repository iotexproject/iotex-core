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
	"github.com/iotexproject/iotex-core/proto"
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
	selp0, err := testutil.SignedTransfer(
		producerAddr,
		producerAddr,
		producerPriKey,
		1,
		big.NewInt(int64(amount)),
		nil,
		100,
		big.NewInt(0),
	)
	require.NoError(err)

	selp1, err := testutil.SignedTransfer(
		producerAddr,
		ta.Addrinfo["alfa"].String(),
		producerPriKey,
		1,
		big.NewInt(int64(amount)),
		nil,
		100,
		big.NewInt(0),
	)
	require.NoError(err)

	selp2, err := testutil.SignedTransfer(
		producerAddr,
		ta.Addrinfo["bravo"].String(),
		producerPriKey,
		1,
		big.NewInt(int64(amount)),
		nil,
		100,
		big.NewInt(0),
	)
	require.NoError(err)

	selp3, err := testutil.SignedTransfer(
		producerAddr,
		ta.Addrinfo["charlie"].String(),
		producerPriKey,
		1,
		big.NewInt(int64(amount)),
		nil,
		100,
		big.NewInt(0),
	)
	require.NoError(err)

	selp4, err := testutil.SignedTransfer(
		producerAddr,
		ta.Addrinfo["echo"].String(),
		producerPriKey,
		1,
		big.NewInt(int64(amount)),
		nil,
		100,
		big.NewInt(0),
	)
	require.NoError(err)

	// verify tx hash
	hash0, e := hex.DecodeString("9e534580106ad77be0e69c8a747839d53a68f0f1151e258261f5ea2fc88c8cb8")
	require.NoError(e)
	actual := selp0.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash0, actual[:])

	hash1, e := hex.DecodeString("3a3c0eaae34ecd274e5a4e984674bd5ebecfc2aecbfb249badc1443b6f5121b2")
	require.NoError(e)
	actual = selp1.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash1, actual[:])

	hash2, e := hex.DecodeString("f3d19dd04f8a04ff91103ba7fb5f3b5b2f68d60d000510ac54c3108c12776af0")
	require.NoError(e)
	actual = selp2.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash2, actual[:])

	hash3, e := hex.DecodeString("87ebe8368331cb99c6b6498512eca388d082e83cc317b5f24a9e5754411c5880")
	require.NoError(e)
	actual = selp3.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash3, actual[:])

	hash4, e := hex.DecodeString("c40e737391cba49df33b2ca2f9a2428e49a14c1cae2456505bac99d4f3e569ba")
	require.NoError(e)
	actual = selp4.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash4, actual[:])

	// manually compute merkle root
	cat := append(hash0, hash1...)
	hash01 := blake2b.Sum256(cat)
	t.Logf("hash01 = %x", hash01)

	cat = append(hash2, hash3...)
	hash23 := blake2b.Sum256(cat)
	t.Logf("hash23 = %x", hash23)

	cat = append(hash4, hash4...)
	hash45 := blake2b.Sum256(cat)
	t.Logf("hash45 = %x", hash45)

	cat = append(hash01[:], hash23[:]...)
	hash03 := blake2b.Sum256(cat)
	t.Logf("hash03 = %x", hash03)

	cat = append(hash45[:], hash45[:]...)
	hash47 := blake2b.Sum256(cat)
	t.Logf("hash47 = %x", hash47)

	cat = append(hash03[:], hash47[:]...)
	hash07 := blake2b.Sum256(cat)
	t.Logf("hash07 = %x", hash07)

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
	require.Equal(hash07[:], hash[:])

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	senderAddr := ta.Addrinfo["producer"].String()
	senderPubKey := ta.Keyinfo["producer"].PubKey
	require.NoError(t, blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
			Pubkey:  keypair.PublicKeyToBytes(senderPubKey),
		},
		Actions: []*iproto.ActionPb{
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       senderAddr,
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       senderAddr,
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
				Version:      version.ProtocolVersion,
				Nonce:        102,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       senderAddr,
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       senderAddr,
				SenderPubKey: keypair.PublicKeyToBytes(senderPubKey),
				Version:      version.ProtocolVersion,
				Nonce:        104,
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
