// Copyright (c) 2022 IoTeX Foundation
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

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestMerkle(t *testing.T) {
	require := require.New(t)

	producerAddr := identityset.Address(27).String()
	producerPubKey := identityset.PrivateKey(27).PublicKey()
	producerPriKey := identityset.PrivateKey(27)
	amount := uint64(50 << 22)
	// create testing transactions
	selp0, err := action.SignedTransfer(producerAddr, producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp1, err := action.SignedTransfer(identityset.Address(28).String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp2, err := action.SignedTransfer(identityset.Address(29).String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp3, err := action.SignedTransfer(identityset.Address(30).String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	selp4, err := action.SignedTransfer(identityset.Address(32).String(), producerPriKey, 1, big.NewInt(int64(amount)), nil, 100, big.NewInt(0))
	require.NoError(err)

	// create block using above 5 tx and verify merkle
	actions := []action.SealedEnvelope{selp0, selp1, selp2, selp3, selp4}
	block := NewBlockDeprecated(
		0,
		0,
		hash.ZeroHash256,
		testutil.TimestampNow(),
		producerPubKey,
		actions,
	)
	hash, err := block.CalculateTxRoot()
	require.NoError(err)
	require.Equal("eb5cb75ae199d96de7c1cd726d5e1a3dff15022ed7bdc914a3d8b346f1ef89c9", hex.EncodeToString(hash[:]))

	hashes := actionHashs(block)
	for i := range hashes {
		h, err := actions[i].Hash()
		require.NoError(err)
		require.Equal(hex.EncodeToString(h[:]), hashes[i])
	}

	t.Log("Merkle root match pass\n")
}

var (
	_pkBytes = identityset.PrivateKey(27).PublicKey().Bytes()
	_pbBlock = iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    123456789,
				Timestamp: timestamppb.Now(),
			},
			ProducerPubkey: _pkBytes,
		},
		Body: &iotextypes.BlockBody{
			Actions: []*iotextypes.Action{
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{
								Amount:    "100000000000000000",
								Recipient: "alice",
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   101,
						ChainID: 1,
					},
					SenderPubKey: _pkBytes,
					Signature:    action.ValidSig,
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Execution{
							Execution: &iotextypes.Execution{
								Contract: "bob",
								Amount:   "200000000000000000",
								Data:     []byte{1, 2, 3, 4},
							},
						},
						Version: version.ProtocolVersion,
						Nonce:   102,
						ChainID: 2,
					},
					SenderPubKey: _pkBytes,
					Signature:    action.ValidSig,
				},
			},
		},
	}
)

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	require.NoError(t, blk.ConvertFromBlockPb(&_pbBlock, 0))

	txHash, err := blk.CalculateTxRoot()
	require.NoError(t, err)

	blk.Header.txRoot = txHash
	blk.Header.receiptRoot = hash.Hash256b(([]byte)("test"))

	raw, err := blk.Serialize()
	require.NoError(t, err)

	var newblk Block
	err = newblk.Deserialize(raw, 0)
	require.NoError(t, err)
	require.Equal(t, blk, newblk)
}

func TestBlockCompressionSize(t *testing.T) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		blk := makeBlock(t, n)
		blkBytes, err := blk.Serialize()
		require.NoError(t, err)
		compressedBlkBytes, err := compress.CompGzip(blkBytes)
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
				_, err = compress.CompGzip(blkBytes)
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
				blkBytes, err = compress.CompGzip(blkBytes)
				require.NoError(b, err)
				b.StartTimer()
				_, err = compress.DecompGzip(blkBytes)
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
	ra := rap.AddActions(sevlps...).
		Build()
	blk, err := NewBuilder(ra).
		SetHeight(1).
		SetTimestamp(time.Now()).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(tb, err)
	return &blk
}

func TestVerifyBlock(t *testing.T) {
	require := require.New(t)

	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash, err := tsf1.Hash()
	require.NoError(err)
	blk, err := NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	t.Run("success", func(t *testing.T) {
		require.True(blk.Header.VerifySignature())
		require.NoError(blk.VerifyTxRoot())
	})

	t.Run("wrong root hash", func(t *testing.T) {
		blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
		require.True(blk.Header.VerifySignature())
		require.Error(blk.VerifyTxRoot())
	})
}

// actionHashs returns action hashs in the block
func actionHashs(blk *Block) []string {
	actHash := make([]string, len(blk.Actions))
	for i := range blk.Actions {
		h, err := blk.Actions[i].Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		actHash[i] = hex.EncodeToString(h[:])
	}
	return actHash
}
