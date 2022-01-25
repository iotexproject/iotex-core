// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
	logsBloom        bloom.BloomFilter // bloom filter for all contract events in this block
	blockSig         []byte            // block signature
	pubkey           crypto.PublicKey  // block producer's public key
}

// Errors
var (
	ErrTxRootMismatch      = errors.New("transaction merkle root does not match")
	ErrDeltaStateMismatch  = errors.New("delta state digest doesn't match")
	ErrReceiptRootMismatch = errors.New("receipt root hash does not match")
)

// Version returns the version of this block.
func (h *Header) Version() uint32 { return h.version }

// Height returns the height of this block.
func (h *Header) Height() uint64 { return h.height }

// Timestamp returns the Timestamp of this block.
func (h *Header) Timestamp() time.Time { return h.timestamp }

// PrevHash returns the hash of prev block.
func (h *Header) PrevHash() hash.Hash256 { return h.prevBlockHash }

// TxRoot returns the hash of all actions in this block.
func (h *Header) TxRoot() hash.Hash256 { return h.txRoot }

// DeltaStateDigest returns the delta sate digest after applying this block.
func (h *Header) DeltaStateDigest() hash.Hash256 { return h.deltaStateDigest }

// PublicKey returns the public key of this header.
func (h *Header) PublicKey() crypto.PublicKey { return h.pubkey }

// ReceiptRoot returns the receipt root after apply this block
func (h *Header) ReceiptRoot() hash.Hash256 { return h.receiptRoot }

// HashBlock return the hash of this block (actually hash of block header)
func (h *Header) HashBlock() hash.Hash256 { return h.HashHeader() }

// LogsBloomfilter return the bloom filter for all contract log events
func (h *Header) LogsBloomfilter() bloom.BloomFilter { return h.logsBloom }

// BlockHeaderProto returns BlockHeader proto.
func (h *Header) BlockHeaderProto() *iotextypes.BlockHeader {
	header := iotextypes.BlockHeader{
		Core: h.BlockHeaderCoreProto(),
	}

	if h.height > 0 {
		header.ProducerPubkey = h.pubkey.Bytes()
		header.Signature = h.blockSig
	}
	return &header
}

// BlockHeaderCoreProto returns BlockHeaderCore proto.
func (h *Header) BlockHeaderCoreProto() *iotextypes.BlockHeaderCore {
	ts, err := ptypes.TimestampProto(h.timestamp)
	if err != nil {
		log.L().Panic("failed to cast to ptypes.timestamp", zap.Error(err))
	}
	header := iotextypes.BlockHeaderCore{
		Version:          h.version,
		Height:           h.height,
		Timestamp:        ts,
		PrevBlockHash:    h.prevBlockHash[:],
		TxRoot:           h.txRoot[:],
		DeltaStateDigest: h.deltaStateDigest[:],
		ReceiptRoot:      h.receiptRoot[:],
	}
	if h.logsBloom != nil {
		header.LogsBloom = h.logsBloom.Bytes()
	}
	return &header
}

// LoadFromBlockHeaderProto loads from protobuf
func (h *Header) LoadFromBlockHeaderProto(pb *iotextypes.BlockHeader) error {
	if err := h.loadFromBlockHeaderCoreProto(pb.GetCore()); err != nil {
		return err
	}
	sig := pb.GetSignature()
	h.blockSig = make([]byte, len(sig))
	copy(h.blockSig, sig)
	pubKey, err := crypto.BytesToPublicKey(pb.GetProducerPubkey())
	if err != nil {
		return err
	}
	h.pubkey = pubKey
	return nil
}

func (h *Header) loadFromBlockHeaderCoreProto(pb *iotextypes.BlockHeaderCore) error {
	h.version = pb.GetVersion()
	h.height = pb.GetHeight()
	ts, err := ptypes.Timestamp(pb.GetTimestamp())
	if err != nil {
		return err
	}
	h.timestamp = ts
	copy(h.prevBlockHash[:], pb.GetPrevBlockHash())
	copy(h.txRoot[:], pb.GetTxRoot())
	copy(h.deltaStateDigest[:], pb.GetDeltaStateDigest())
	copy(h.receiptRoot[:], pb.GetReceiptRoot())
	if pb.GetLogsBloom() != nil {
		h.logsBloom, err = bloom.BloomFilterFromBytesLegacy(pb.GetLogsBloom(), 2048, 3)
	}
	return err
}

// SerializeCore returns byte stream for header core.
func (h *Header) SerializeCore() []byte {
	return byteutil.Must(proto.Marshal(h.BlockHeaderCoreProto()))
}

// Serialize returns the serialized byte stream of the block header
func (h *Header) Serialize() ([]byte, error) {
	return proto.Marshal(h.BlockHeaderProto())
}

// Deserialize loads from the serialized byte stream
func (h *Header) Deserialize(buf []byte) error {
	pb := &iotextypes.BlockHeader{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	return h.LoadFromBlockHeaderProto(pb)
}

// HashHeader hashes the header
func (h *Header) HashHeader() hash.Hash256 {
	s, _ := h.Serialize()
	return hash.Hash256b(s)
}

// HashHeaderCore hahes the header core.
func (h *Header) HashHeaderCore() hash.Hash256 {
	return hash.Hash256b(h.SerializeCore())
}

// VerifySignature verifies the signature saved in block header
func (h *Header) VerifySignature() bool {
	hash := h.HashHeaderCore()

	if h.pubkey == nil {
		return false
	}
	return h.pubkey.Verify(hash[:], h.blockSig)
}

// VerifyDeltaStateDigest verifies the delta state digest in header
func (h *Header) VerifyDeltaStateDigest(digest hash.Hash256) error {
	if h.deltaStateDigest != digest {
		return ErrDeltaStateMismatch
	}
	return nil
}

// VerifyReceiptRoot verifies the receipt root in header
func (h *Header) VerifyReceiptRoot(root hash.Hash256) error {
	if h.receiptRoot != root {
		return ErrReceiptRootMismatch
	}
	return nil
}

// VerifyDeltaStateDigest verifies the delta state digest in header
func (h *Header) VerifyTransactionRoot(root hash.Hash256) error {
	if h.txRoot != root {
		return ErrTxRootMismatch
	}
	return nil
}

// ProducerAddress returns the address of producer
func (h *Header) ProducerAddress() string {
	addr := h.pubkey.Address()
	return addr.String()
}

// HeaderLogger returns a new logger with block header fields' value.
func (h *Header) HeaderLogger(l *zap.Logger) *zap.Logger {
	return l.With(zap.Uint32("version", h.version),
		zap.Uint64("height", h.height),
		zap.String("timestamp", h.timestamp.String()),
		log.Hex("prevBlockHash", h.prevBlockHash[:]),
		log.Hex("txRoot", h.txRoot[:]),
		log.Hex("receiptRoot", h.receiptRoot[:]),
		log.Hex("deltaStateDigest", h.deltaStateDigest[:]),
	)
}
