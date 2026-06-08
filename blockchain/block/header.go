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
// pubkey holds the block producer's public key as the minimal Verifier
// interface (Bytes + Verify). Pre-fork it is a secp256k1 crypto.PublicKey;
// once the BLS Producer Identity follow-up to IIP-52 activates, BLS-signed
// blocks carry a *crypto.BLS12381PublicKey. Identity-derivation methods are
// not on the storage interface — see verifier.go for the rationale. Code
// that needs a string identity calls ProducerAddress (dispatched by type);
// code that wants the typed secp256k1 key calls PublicKey and handles nil
// for BLS-signed headers.
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
	pubkey           Verifier          // block producer's public key (typed; either secp256k1 or BLS12-381)
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
// whose pubkey is not a secp256k1 key (e.g. post-fork BLS-signed headers).
// Use ProducerPubKey for the raw bytes regardless of scheme.
func (h *Header) PublicKey() crypto.PublicKey {
	if h.pubkey == nil {
		return nil
	}
	pk, ok := h.pubkey.(crypto.PublicKey)
	if !ok {
		return nil
	}
	return pk
}

// ProducerPubKey returns the raw bytes of the producer's public key,
// regardless of signature scheme. For pre-fork headers this is a secp256k1
// pubkey (33 or 65 bytes); for post-fork BLS-signed headers this is a 48-byte
// BLS12-381 pubkey. Returns nil for an empty header.
func (h *Header) ProducerPubKey() []byte {
	if h.pubkey == nil {
		return nil
	}
	b := h.pubkey.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
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
		if h.pubkey != nil {
			header.ProducerPubkey = append([]byte(nil), h.pubkey.Bytes()...)
		}
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
	if len(pubKey) == 0 {
		h.pubkey = nil
		return nil
	}
	// Length-based dispatch lives only here, at the wire→typed boundary. After
	// this point Header carries a typed Verifier; downstream code uses
	// pubkey.Verify and pubkey.Bytes without re-inspecting the length.
	switch len(pubKey) {
	case crypto.BLSPubkeyLength:
		bls, err := crypto.BLS12381PublicKeyFromBytes(pubKey)
		if err != nil {
			return errors.Wrap(err, "invalid BLS producer pubkey in header")
		}
		h.pubkey = bls
	default:
		pk, err := crypto.BytesToPublicKey(pubKey)
		if err != nil {
			return errors.Wrap(err, "invalid secp256k1 producer pubkey in header")
		}
		h.pubkey = pk
	}
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

// VerifySignature verifies the signature saved in block header against the
// digest of the header core. The verification scheme is whatever the stored
// Verifier (typed at decode time) implements — secp256k1 pre-fork,
// BLS12-381 post-fork.
func (h *Header) VerifySignature() bool {
	if h.pubkey == nil {
		return false
	}
	digest := h.HashHeaderCore()
	return h.pubkey.Verify(digest[:], h.blockSig)
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
// Dispatch is by the stored pubkey's concrete type:
//   - secp256k1 (crypto.PublicKey): the iotex address ("io1...") derived
//     from hash160 of the pubkey, matching pre-fork semantics.
//   - BLS12-381 (*crypto.BLS12381PublicKey): the hex encoding of the
//     48-byte BLS public key. BLS public keys have no account semantics
//     in iotex (no balance, no tx sender role) and are intentionally not
//     derived into a 20-byte iotex address; the hex form is the canonical
//     post-fork operator identifier.
//
// The return type stays string so callers that use the value as a map key
// or for `==` comparison continue to work; only the string format flips
// across the fork boundary.
func (h *Header) ProducerAddress() string {
	if h.pubkey == nil {
		return ""
	}
	if blsPK, ok := h.pubkey.(*crypto.BLS12381PublicKey); ok {
		return hex.EncodeToString(blsPK.Bytes())
	}
	if pk, ok := h.pubkey.(crypto.PublicKey); ok {
		addr := pk.Address()
		if addr == nil {
			return ""
		}
		return addr.String()
	}
	return ""
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
