// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/test/identityset"
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
				Version:   version.ProtocolVersion,
				Height:    123456789,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: senderPubKey.Bytes(),
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
				SenderPubKey: senderPubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.Transfer{},
					},
					Version: version.ProtocolVersion,
					Nonce:   102,
				},
				SenderPubKey: senderPubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				SenderPubKey: senderPubKey.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{},
					},
					Version: version.ProtocolVersion,
					Nonce:   104,
				},
				SenderPubKey: senderPubKey.Bytes(),
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

func TestBlockCompressionSize(t *testing.T) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		blk := makeBlock(t, n)
		blkBytes, err := blk.Serialize()
		require.NoError(t, err)
		compressedBlkBytes, err := compress.Compress(blkBytes)
		require.NoError(t, err)
		log.L().Info(
			"Compression result",
			zap.Int("numActions", n),
			zap.Int("before", len(blkBytes)),
			zap.Int("after", len(compressedBlkBytes)),
		)
	}
}

func BenchmarkBlockCompression(b *testing.B) {
	for _, i := range []int{1, 10, 100, 1000, 2000} {
		b.Run(fmt.Sprintf("numActions: %d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				blk := makeBlock(b, i)
				blkBytes, err := blk.Serialize()
				require.NoError(b, err)
				b.StartTimer()
				_, err = compress.Compress(blkBytes)
				b.StopTimer()
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkBlockDecompression(b *testing.B) {
	for _, i := range []int{1, 10, 100, 1000, 2000} {
		b.Run(fmt.Sprintf("numActions: %d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				blk := makeBlock(b, i)
				blkBytes, err := blk.Serialize()
				require.NoError(b, err)
				blkBytes, err = compress.Compress(blkBytes)
				require.NoError(b, err)
				b.StartTimer()
				_, err = compress.Decompress(blkBytes)
				b.StopTimer()
				require.NoError(b, err)
			}
		})
	}
}

func makeBlock(tb testing.TB, n int) *Block {
	rand.Seed(time.Now().Unix())
	sevlps := make([]action.SealedEnvelope, 0)
	for j := 1; j <= n; j++ {
		i := rand.Int()
		tsf, err := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		require.NoError(tb, err)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		require.NoError(tb, err)
		sevlps = append(sevlps, sevlp)
	}
	rap := RunnableActionsBuilder{}
	ra := rap.
		SetHeight(1).
		SetTimeStamp(time.Now()).
		AddActions(sevlps...).
		Build(identityset.PrivateKey(0).PublicKey())
	blk, err := NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(tb, err)
	return &blk
}
