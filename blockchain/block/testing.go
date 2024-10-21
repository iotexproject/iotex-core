// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestingBuilder is used to construct Block.
type TestingBuilder struct{ blk Block }

// NewTestingBuilder creates a Builder.
func NewTestingBuilder() *TestingBuilder {
	return &TestingBuilder{
		blk: Block{
			Header: Header{
				version: version.ProtocolVersion,
			},
		},
	}
}

// SetVersion sets the protocol version for block which is building.
func (b *TestingBuilder) SetVersion(v uint32) *TestingBuilder {
	b.blk.Header.version = v
	return b
}

// SetHeight sets the block height for block which is building.
func (b *TestingBuilder) SetHeight(h uint64) *TestingBuilder {
	b.blk.Header.height = h
	return b
}

// SetTimeStamp sets the time stamp for block which is building.
func (b *TestingBuilder) SetTimeStamp(ts time.Time) *TestingBuilder {
	b.blk.Header.timestamp = ts
	return b
}

// SetPrevBlockHash sets the previous block hash for block which is building.
func (b *TestingBuilder) SetPrevBlockHash(h hash.Hash256) *TestingBuilder {
	b.blk.Header.prevBlockHash = h
	return b
}

// AddActions adds actions for block which is building.
func (b *TestingBuilder) AddActions(acts ...*action.SealedEnvelope) *TestingBuilder {
	if b.blk.Actions == nil {
		b.blk.Actions = make([]*action.SealedEnvelope, 0)
	}
	b.blk.Actions = append(b.blk.Actions, acts...)
	return b
}

// SetReceipts sets the receipts after running actions included in this building block.
func (b *TestingBuilder) SetReceipts(receipts []*action.Receipt) *TestingBuilder {
	b.blk.Receipts = receipts // make a shallow copy
	return b
}

// SignAndBuild signs and then builds a block.
func (b *TestingBuilder) SignAndBuild(signerPrvKey crypto.PrivateKey) (Block, error) {
	var err error
	b.blk.Header.txRoot, err = b.blk.CalculateTxRoot()
	if err != nil {
		log.L().Debug("error in getting hash", zap.Error(err))
		return Block{}, errors.New("failed to get hash")
	}
	b.blk.Header.pubkey = signerPrvKey.PublicKey()
	h := b.blk.Header.HashHeaderCore()
	sig, err := signerPrvKey.Sign(h[:])
	if err != nil {
		log.L().Debug("error in getting hash", zap.Error(err))
		return Block{}, errors.New("failed to sign block")
	}
	b.blk.Header.blockSig = sig
	return b.blk, nil
}

// NewBlockDeprecated returns a new block
// This method is deprecated. Only used in old tests.
func NewBlockDeprecated(
	chainID uint32,
	height uint64,
	prevBlockHash hash.Hash256,
	timestamp time.Time,
	producer crypto.PublicKey,
	actions []*action.SealedEnvelope,
) *Block {
	block := &Block{
		Header: Header{
			version:       version.ProtocolVersion,
			height:        height,
			timestamp:     timestamp,
			prevBlockHash: prevBlockHash,
			pubkey:        producer,
			txRoot:        hash.ZeroHash256,
			receiptRoot:   hash.ZeroHash256,
		},
		Body: Body{
			Actions: actions,
		},
	}

	var err error
	block.Header.txRoot, err = block.CalculateTxRoot()
	if err != nil {
		return &Block{}
	}
	return block
}

func createTestBlobSidecar(m, n int) *types.BlobTxSidecar {
	testBlob := kzg4844.Blob{byte(m), byte(n)}
	testBlobCommit := MustNoErrorV(kzg4844.BlobToCommitment(testBlob))
	testBlobProof := MustNoErrorV(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
	return &types.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{testBlob},
		Commitments: []kzg4844.Commitment{testBlobCommit},
		Proofs:      []kzg4844.Proof{testBlobProof},
	}
}

func CreateTestBlockWithBlob(start, n int) ([]*Block, error) {
	var (
		amount = big.NewInt(20)
		price  = big.NewInt(10)
	)
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, amount, nil, 100000, price)
	if err != nil {
		return nil, err
	}
	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, amount, nil, 100000, price)
	if err != nil {
		return nil, err
	}
	blks := make([]*Block, n)
	for i := start; i < start+n; i++ {
		sc3 := createTestBlobSidecar(i, i+1)
		act3 := (&action.EnvelopeBuilder{}).SetTxType(action.BlobTxType).SetChainID(1).SetNonce(uint64(i)).
			SetGasLimit(20000).SetDynamicGas(big.NewInt(100), big.NewInt(200)).
			SetBlobTxData(uint256.NewInt(15), sc3.BlobHashes(), sc3).
			SetAction(action.NewTransfer(amount, "", nil)).Build()
		tsf3, err := action.Sign(act3, identityset.PrivateKey(25))
		if err != nil {
			return nil, err
		}
		if tsf3.TxType() != action.BlobTxType {
			return nil, action.ErrInvalidAct
		}
		sc4 := createTestBlobSidecar(i+1, i)
		act4 := (&action.EnvelopeBuilder{}).SetTxType(action.BlobTxType).SetChainID(1).SetNonce(uint64(i)).
			SetGasLimit(20000).SetDynamicGas(big.NewInt(100), big.NewInt(200)).
			SetBlobTxData(uint256.NewInt(15), sc4.BlobHashes(), sc4).
			SetAction(action.NewTransfer(amount, "", nil)).Build()
		tsf4, err := action.Sign(act4, identityset.PrivateKey(26))
		if err != nil {
			return nil, err
		}
		if tsf4.TxType() != action.BlobTxType {
			return nil, action.ErrInvalidAct
		}
		blkhash, err := tsf1.Hash()
		if err != nil {
			return nil, err
		}
		blk, err := NewTestingBuilder().
			SetHeight(uint64(i)).
			SetPrevBlockHash(blkhash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(tsf1, tsf3, tsf2, tsf4).
			SignAndBuild(identityset.PrivateKey(27))
		if err != nil {
			return nil, err
		}
		blks[i-start] = &blk
	}
	return blks, nil
}
