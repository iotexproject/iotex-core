// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"go.uber.org/zap"
)

// Header defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
type Header struct {
	version          uint32            // version
	height           uint64            // block height
	timestamp        time.Time         // propose timestamp
	prevBlockHash    hash.Hash256      // hash of previous block
	txRoot           hash.Hash256      // merkle root of all transactions
	deltaStateDigest hash.Hash256      // digest of state change by this block
	receiptRoot      hash.Hash256      // root of receipt trie
	blockSig         []byte            // block signature
	pubkey           keypair.PublicKey // block producer's public key
}

// Version returns the version of this block.
func (h Header) Version() uint32 { return h.version }

// Height returns the height of this block.
func (h Header) Height() uint64 { return h.height }

// Timestamp returns the Timestamp of this block.
func (h Header) Timestamp() time.Time { return h.timestamp }

// PrevHash returns the hash of prev block.
func (h Header) PrevHash() hash.Hash256 { return h.prevBlockHash }

// TxRoot returns the hash of all actions in this block.
func (h Header) TxRoot() hash.Hash256 { return h.txRoot }

// DeltaStateDigest returns the delta sate digest after applying this block.
func (h Header) DeltaStateDigest() hash.Hash256 { return h.deltaStateDigest }

// PublicKey returns the public key of this header.
func (h Header) PublicKey() keypair.PublicKey { return h.pubkey }

// ReceiptRoot returns the receipt root after apply this block
func (h Header) ReceiptRoot() hash.Hash256 { return h.receiptRoot }

// BlockHeaderProto returns BlockHeader proto.
func (h Header) BlockHeaderProto() *iotextypes.BlockHeader {
	return &iotextypes.BlockHeader{
		Core:           h.BlockHeaderCoreProto(),
		ProducerPubkey: h.pubkey.Bytes(),
		Signature:      h.blockSig,
	}
}

// BlockHeaderCoreProto returns BlockHeaderCore proto.
func (h Header) BlockHeaderCoreProto() *iotextypes.BlockHeaderCore {
	ts, err := ptypes.TimestampProto(h.timestamp)
	if err != nil {
		log.L().Panic("failed to cast to ptypes.timestamp", zap.Error(err))
	}
	return &iotextypes.BlockHeaderCore{
		Version:          h.version,
		Height:           h.height,
		Timestamp:        ts,
		PrevBlockHash:    h.prevBlockHash[:],
		TxRoot:           h.txRoot[:],
		DeltaStateDigest: h.deltaStateDigest[:],
		ReceiptRoot:      h.receiptRoot[:],
	}
}

// CoreByteStream returns byte stream for header core.
func (h Header) CoreByteStream() []byte {
	return byteutil.Must(proto.Marshal(h.BlockHeaderCoreProto()))
}

// ByteStream returns byte stream for header.
func (h Header) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(h.BlockHeaderProto()))
}

// HashHeader hashes the header
func (h Header) HashHeader() hash.Hash256 {
	return hash.Hash256b(h.ByteStream())
}

// HashHeaderCore hahes the header core.
func (h Header) HashHeaderCore() hash.Hash256 {
	return hash.Hash256b(h.CoreByteStream())
}

// ByteStream returns a byte stream of the header.

// HeaderLogger returns a new logger with block header fields' value.
func (h Header) HeaderLogger(l *zap.Logger) *zap.Logger {
	return l.With(zap.Uint32("version", h.version),
		zap.Uint64("height", h.height),
		zap.String("timestamp", h.timestamp.String()),
		log.Hex("prevBlockHash", h.prevBlockHash[:]),
		log.Hex("txRoot", h.txRoot[:]),
		log.Hex("receiptRoot", h.receiptRoot[:]),
		log.Hex("deltaStateDigest", h.deltaStateDigest[:]),
	)
}
