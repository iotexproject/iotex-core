// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/proto"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
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

	amount := uint64(50 << 22)
	// create testing transactions
	cbtsf0 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["miner"].RawAddress)
	require.NotNil(cbtsf0)
	cbtsf1 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["alfa"].RawAddress)
	require.NotNil(cbtsf1)
	cbtsf2 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbtsf2)
	cbtsf3 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(cbtsf3)
	cbtsf4 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["echo"].RawAddress)
	require.NotNil(cbtsf4)

	// verify tx hash
	hash0, _ := hex.DecodeString("38628817384f0c67ea3396529162e8d7cb4491c227c6a8eaaaf8828bc11614fd")
	actual := cbtsf0.Hash()
	require.Equal(hash0, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash1, _ := hex.DecodeString("ce5dd70b032614bd8e3d82b257b478eb49a4da90d531e7db6cd4eebc98ed149d")
	actual = cbtsf1.Hash()
	require.Equal(hash1, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash2, _ := hex.DecodeString("164914612e8628e0de69ea4e4184e63d46439ceb46c46fc60a43b636277dc1e9")
	actual = cbtsf2.Hash()
	require.Equal(hash2, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash3, _ := hex.DecodeString("73e0a24bb15687ffb98c254f617db7e7d59963f020de38b1a7c284ffde7dc8d5")
	actual = cbtsf3.Hash()
	require.Equal(hash3, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash4, _ := hex.DecodeString("e1c72a310432a0040bedb40ee8ea885fc93258f19687e271a2e2c2fb62723474")
	actual = cbtsf4.Hash()
	require.Equal(hash4, actual[:])
	t.Logf("actual hash = %x", actual[:])

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
	block := NewBlock(0, 0, common.ZeroHash32B, []*action.Transfer{cbtsf0, cbtsf1, cbtsf2, cbtsf3, cbtsf4}, nil)
	hash := block.TxRoot()
	require.Equal(hash07[:], hash[:])

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: common.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{
					Version: common.ProtocolVersion,
					Nonce:   101,
				},
			}},
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{
					Version: common.ProtocolVersion,
					Nonce:   102,
				},
			}},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{
					Version: common.ProtocolVersion,
					Nonce:   103,
				},
			}},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{
					Version: common.ProtocolVersion,
					Nonce:   104},
			}},
		},
	})

	blk.Header.txRoot = blk.TxRoot()

	raw, err := blk.Serialize()
	require.Nil(t, err)

	var newblk Block
	err = newblk.Deserialize(raw)
	require.Nil(t, err)

	require.Equal(t, uint64(123456789), newblk.Header.height)

	require.Equal(t, uint64(101), newblk.Transfers[0].Nonce)
	require.Equal(t, uint64(102), newblk.Transfers[1].Nonce)

	require.Equal(t, uint64(103), newblk.Votes[0].Nonce)
	require.Equal(t, uint64(104), newblk.Votes[1].Nonce)
}

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{nil}
	tsf1 := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err := tsf1.Sign(ta.Addrinfo["miner"])
	require.Nil(err)
	tsf2 := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["miner"])
	require.Nil(err)
	hash := tsf1.Hash()
	blk := NewBlock(1, 1, hash, []*action.Transfer{tsf1, tsf2}, nil)
	blk.Header.Pubkey = ta.Addrinfo["miner"].PublicKey
	blkHash := blk.HashBlock()
	blk.Header.blockSig = cp.Sign(ta.Addrinfo["miner"].PrivateKey, blkHash[:])
	require.Nil(val.Validate(blk, 0, hash))
	blk.Transfers[0], blk.Transfers[1] = blk.Transfers[1], blk.Transfers[0]
	require.NotNil(val.Validate(blk, 0, hash))
}
