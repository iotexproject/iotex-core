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

	producer := ta.IotxAddrinfo["producer"]
	amount := uint64(50 << 22)
	// create testing transactions
	cbtsf0 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), producer.RawAddress)
	require.NotNil(cbtsf0)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(producer.RawAddress).
		SetGasLimit(100).
		SetAction(cbtsf0).Build()
	selp0, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf1 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.IotxAddrinfo["alfa"].RawAddress)
	require.NotNil(cbtsf1)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.IotxAddrinfo["alfa"].RawAddress).
		SetGasLimit(100).
		SetAction(cbtsf1).Build()
	selp1, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf2 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.IotxAddrinfo["bravo"].RawAddress)
	require.NotNil(cbtsf2)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.IotxAddrinfo["bravo"].RawAddress).
		SetGasLimit(100).
		SetAction(cbtsf2).Build()
	selp2, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf3 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.IotxAddrinfo["charlie"].RawAddress)
	require.NotNil(cbtsf3)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.IotxAddrinfo["charlie"].RawAddress).
		SetGasLimit(100).
		SetAction(cbtsf3).Build()
	selp3, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf4 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.IotxAddrinfo["echo"].RawAddress)
	require.NotNil(cbtsf4)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.IotxAddrinfo["echo"].RawAddress).
		SetGasLimit(100).
		SetAction(cbtsf4).Build()
	selp4, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	// verify tx hash
	hash0, e := hex.DecodeString("4ef861f8ed4d99377a149ca2a114bd7b224ea93f8e9258a9691f0c6a30ac5c4e")
	require.NoError(e)
	actual := cbtsf0.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash0, actual[:])

	hash1, e := hex.DecodeString("201990668742dd16825ce64446f2002f2b52814ee40242fba767edeca0b9d311")
	require.NoError(e)
	actual = cbtsf1.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash1, actual[:])

	hash2, e := hex.DecodeString("c3c94823d6f60ac5743c9524405c6353b3d6b7b7fd95a7b9e6b48cdfa9f8c6b2")
	require.NoError(e)
	actual = cbtsf2.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash2, actual[:])

	hash3, e := hex.DecodeString("ba12ca7a859da93df536a3a1748294b3b29e96c1dc2a214db41d312391fdbed0")
	require.NoError(e)
	actual = cbtsf3.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash3, actual[:])

	hash4, e := hex.DecodeString("53cacd3e5d5f907c6b8463fb5a191c35c0267b61814f216f3d6a97b6c375c5d9")
	require.NoError(e)
	actual = cbtsf4.Hash()
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
		hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.IotxAddrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{selp0, selp1, selp2, selp3, selp4},
	)
	hash := block.CalculateTxRoot()
	require.Equal(hash07[:], hash[:])

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	sender := ta.IotxAddrinfo["producer"]
	require.NoError(t, blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        102,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        104,
			},
		},
	}))

	blk.Header.txRoot = blk.CalculateTxRoot()

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
}
