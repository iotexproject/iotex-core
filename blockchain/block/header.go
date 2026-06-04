// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// Header defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
//
// producerPubkey holds the raw bytes of the block producer's public key.
// Pre-fork it is a secp256k1 pubkey (33 or 65 bytes); once BLS aggregation is
// activated (IIP-52 follow-up) blocks may carry a BLS12-381 pubkey (48
// bytes). The signature scheme is implied by len(blockSig): 65B secp256k1 vs
// 96B BLS12-381. See VerifySignature and ProducerAddress for the dispatch.
type Header struct {
	version          uint32            // version
	height           uint64            // block height
	gasUsed          uint64            // used gas
	timestamp        time.Time         // propose timestamp
	prevBlockHash    hash.Hash256      // hash of previous block
	txRoot           hash.Hash256      // merkle root of all transactions
	deltaStateDigest hash.Hash256      // digest of state change by this block
	receiptRoot      hash.Hash256      // root of receipt trie
	logsBloom        bloom.BloomFilter // bloom filter for all contract events in this block
	blockSig         []byte            // block signature (secp256k1: 65B; BLS12-381: 96B)
	producerPubkey   []byte            // block producer's public key (raw bytes)
	baseFee          *big.Int          // added by EIP-1559 and is ignored in legacy headers

	// added by EIP-4844 and is ignored in legacy headers.
	blobGasUsed   uint64
	excessBlobGas uint64
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

// GasUsed returns the gas used in this block.
func (h *Header) GasUsed() uint64 { return h.gasUsed }

// Timestamp returns the Timestamp of this block.
func (h *Header) Timestamp() time.Time { return h.timestamp }

// PrevHash returns the hash of prev block.
func (h *Header) PrevHash() hash.Hash256 { return h.prevBlockHash }

// TxRoot returns the hash of all actions in this block.
func (h *Header) TxRoot() hash.Hash256 { return h.txRoot }

// DeltaStateDigest returns the delta sate digest after applying this block.
func (h *Header) DeltaStateDigest() hash.Hash256 { return h.deltaStateDigest }

// PublicKey returns the producer's secp256k1 public key, or nil for headers
// whose producerPubkey is not a secp256k1 key (e.g. post-fork BLS-signed
// headers). Use ProducerPubKey for the raw bytes regardless of scheme.
func (h *Header) PublicKey() crypto.PublicKey {
	if len(h.producerPubkey) == 0 {
		return nil
	}
	pk, err := crypto.BytesToPublicKey(h.producerPubkey)
	if err != nil {
		return nil
	}
	return pk
}

// ProducerPubKey returns the raw bytes of the producer's public key,
// regardless of signature scheme. For pre-fork headers this is a secp256k1
// pubkey (33 or 65 bytes); for post-fork BLS-signed headers this is a 48-byte
// BLS12-381 pubkey. Returns a defensive copy.
func (h *Header) ProducerPubKey() []byte {
	if len(h.producerPubkey) == 0 {
		return nil
	}
	out := make([]byte, len(h.producerPubkey))
	copy(out, h.producerPubkey)
	return out
}

// ReceiptRoot returns the receipt root after apply this block
func (h *Header) ReceiptRoot() hash.Hash256 { return h.receiptRoot }

// HashBlock return the hash of this block (actually hash of block header)
func (h *Header) HashBlock() hash.Hash256 { return h.HashHeader() }

// LogsBloomfilter return the bloom filter for all contract log events
func (h *Header) LogsBloomfilter() bloom.BloomFilter { return h.logsBloom }

// BaseFee returns the baseFee of the block
func (h *Header) BaseFee() *big.Int {
	if h.baseFee == nil {
		return nil
	}
	return new(big.Int).Set(h.baseFee)
}

// BlobGasUsed returns the blob gas used in this block
func (h *Header) BlobGasUsed() uint64 {
	return h.blobGasUsed
}

// ExcessBlobGas returns the excess blob gas in this block
func (h *Header) ExcessBlobGas() uint64 {
	return h.excessBlobGas
}

// Proto returns BlockHeader proto.
func (h *Header) Proto() *iotextypes.BlockHeader {
	header := iotextypes.BlockHeader{
		Core: h.BlockHeaderCoreProto(),
	}

	if h.height > 0 {
		header.ProducerPubkey = append([]byte(nil), h.producerPubkey...)
		header.Signature = h.blockSig
	}
	return &header
}

// BlockHeaderCoreProto returns BlockHeaderCore proto.
func (h *Header) BlockHeaderCoreProto() *iotextypes.BlockHeaderCore {
	ts := timestamppb.New(h.timestamp)
	header := iotextypes.BlockHeaderCore{
		Version:          h.version,
		Height:           h.height,
		GasUsed:          h.gasUsed,
		Timestamp:        ts,
		PrevBlockHash:    h.prevBlockHash[:],
		TxRoot:           h.txRoot[:],
		DeltaStateDigest: h.deltaStateDigest[:],
		ReceiptRoot:      h.receiptRoot[:],
		BlobGasUsed:      h.blobGasUsed,
		ExcessBlobGas:    h.excessBlobGas,
	}
	if h.logsBloom != nil {
		header.LogsBloom = h.logsBloom.Bytes()
	}
	if h.baseFee != nil {
		header.BaseFee = h.baseFee.Bytes()
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
	pubKey := pb.GetProducerPubkey()
	h.producerPubkey = make([]byte, len(pubKey))
	copy(h.producerPubkey, pubKey)
	return nil
}

func (h *Header) loadFromBlockHeaderCoreProto(pb *iotextypes.BlockHeaderCore) error {
	h.version = pb.GetVersion()
	h.height = pb.GetHeight()
	h.gasUsed = pb.GetGasUsed()
	if err := pb.GetTimestamp().CheckValid(); err != nil {
		return err
	}
	ts := pb.GetTimestamp().AsTime()
	h.timestamp = ts
	copy(h.prevBlockHash[:], pb.GetPrevBlockHash())
	copy(h.txRoot[:], pb.GetTxRoot())
	copy(h.deltaStateDigest[:], pb.GetDeltaStateDigest())
	copy(h.receiptRoot[:], pb.GetReceiptRoot())
	var err error
	if pb.GetLogsBloom() != nil {
		h.logsBloom, err = bloom.NewBloomFilterLegacy(2048, 3)
		if err != nil {
			return err
		}
		if err = h.logsBloom.FromBytes(pb.GetLogsBloom()); err != nil {
			return err
		}
	}
	if fee := pb.GetBaseFee(); fee != nil {
		h.baseFee = new(big.Int).SetBytes(fee)
	}
	h.blobGasUsed = pb.GetBlobGasUsed()
	h.excessBlobGas = pb.GetExcessBlobGas()
	return err
}

// SerializeCore returns byte stream for header core.
func (h *Header) SerializeCore() []byte {
	return byteutil.Must(proto.Marshal(h.BlockHeaderCoreProto()))
}

// Serialize returns the serialized byte stream of the block header
func (h *Header) Serialize() ([]byte, error) {
	return proto.Marshal(h.Proto())
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

// VerifySignature verifies the signature saved in block header. The
// signature scheme is selected by len(blockSig): secp256k1 (65 bytes) is the
// pre-fork path; BLS12-381 (96 bytes, G2 compressed) is the post-fork path.
func (h *Header) VerifySignature() bool {
	if len(h.producerPubkey) == 0 {
		return false
	}
	digest := h.HashHeaderCore()
	switch len(h.blockSig) {
	case crypto.BLSAggregateSignatureLength:
		blsPK, err := crypto.BLS12381PublicKeyFromBytes(h.producerPubkey)
		if err != nil {
			return false
		}
		return blsPK.Verify(digest[:], h.blockSig)
	default:
		pk, err := crypto.BytesToPublicKey(h.producerPubkey)
		if err != nil {
			return false
		}
		return pk.Verify(digest[:], h.blockSig)
	}
}

// VerifyDeltaStateDigest verifies the delta state digest in header
func (h *Header) VerifyDeltaStateDigest(digest hash.Hash256) bool {
	return h.deltaStateDigest == digest
}

// VerifyReceiptRoot verifies the receipt root in header
func (h *Header) VerifyReceiptRoot(root hash.Hash256) bool {
	return h.receiptRoot == root
}

// VerifyTransactionRoot verifies the delta state digest in header
func (h *Header) VerifyTransactionRoot(root hash.Hash256) bool {
	return h.txRoot == root
}

// ProducerAddress returns a string identifier for the block producer.
//
// Dispatch is on len(blockSig):
//   - secp256k1 (65B): the secp256k1-derived iotex address ("io1..."),
//     matching pre-fork semantics.
//   - BLS12-381 (96B): the hex encoding of the 48-byte BLS public key.
//     BLS public keys have no account semantics in iotex (no balance, no tx
//     sender role) and are intentionally not derived into an iotex address;
//     the hex form is the canonical post-fork operator identifier.
//
// The return type stays string so existing callers that use the value as a
// map key or for `==` comparison continue to work; only the string format
// flips across the fork boundary.
func (h *Header) ProducerAddress() string {
	switch len(h.blockSig) {
	case crypto.BLSAggregateSignatureLength:
		return hex.EncodeToString(h.producerPubkey)
	default:
		pk, err := crypto.BytesToPublicKey(h.producerPubkey)
		if err != nil || pk == nil {
			return ""
		}
		addr := pk.Address()
		if addr == nil {
			return ""
		}
		return addr.String()
	}
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
		zap.Uint64("blobGasUsed", h.blobGasUsed),
		zap.Uint64("excessBlobGas", h.excessBlobGas),
	)
}
