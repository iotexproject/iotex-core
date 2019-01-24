// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Header defines the struct of block header
// make sure the variable type and order of this struct is same as "BlockHeaderPb" in blockchain.pb.go
type Header struct {
	version          uint32            // version
	chainID          uint32            // this chain's ID
	height           uint64            // block height
	timestamp        int64             // unix timestamp
	prevBlockHash    hash.Hash32B      // hash of previous block
	txRoot           hash.Hash32B      // merkle root of all transactions
	stateRoot        hash.Hash32B      // root of state trie
	deltaStateDigest hash.Hash32B      // digest of state change by this block
	receiptRoot      hash.Hash32B      // root of receipt trie
	blockSig         []byte            // block signature
	pubkey           keypair.PublicKey // block producer's public key
	dkgID            []byte            // dkg ID of producer
	dkgPubkey        []byte            // dkg public key of producer
	dkgBlockSig      []byte            // dkg signature of producer
}

// Version returns the version of this block.
func (h Header) Version() uint32 { return h.version }

// ChainID returns the chain id of this block.
func (h Header) ChainID() uint32 { return h.chainID }

// Height returns the height of this block.
func (h Header) Height() uint64 { return h.height }

// Timestamp returns the Timestamp of this block.
func (h Header) Timestamp() int64 { return h.timestamp }

// PrevHash returns the hash of prev block.
func (h Header) PrevHash() hash.Hash32B { return h.prevBlockHash }

// TxRoot returns the hash of all actions in this block.
func (h Header) TxRoot() hash.Hash32B { return h.txRoot }

// StateRoot returns the state root after applying this block.
func (h Header) StateRoot() hash.Hash32B { return h.stateRoot }

// DeltaStateDigest returns the delta sate digest after applying this block.
func (h Header) DeltaStateDigest() hash.Hash32B { return h.deltaStateDigest }

// PublicKey returns the public key of this header.
func (h Header) PublicKey() keypair.PublicKey { return h.pubkey }

// DKGPubkey returns DKG PublicKey.
func (h Header) DKGPubkey() []byte {
	pk := make([]byte, len(h.dkgPubkey))
	copy(pk, h.dkgPubkey)
	return pk
}

// DKGID returns DKG ID.
func (h Header) DKGID() []byte {
	id := make([]byte, len(h.dkgID))
	copy(id, h.dkgID)
	return id
}

// DKGSignature returns DKG Signature.
func (h Header) DKGSignature() []byte {
	sig := make([]byte, len(h.dkgBlockSig))
	copy(sig, h.dkgBlockSig)
	return sig
}

// ByteStream returns a byte stream of the header.
func (h Header) ByteStream() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, h.version)
	tmp4B := make([]byte, 4)
	enc.MachineEndian.PutUint32(tmp4B, h.chainID)
	stream = append(stream, tmp4B...)
	tmp8B := make([]byte, 8)
	enc.MachineEndian.PutUint64(tmp8B, h.height)
	stream = append(stream, tmp8B...)
	enc.MachineEndian.PutUint64(tmp8B, uint64(h.timestamp))
	stream = append(stream, tmp8B...)
	stream = append(stream, h.prevBlockHash[:]...)
	stream = append(stream, h.txRoot[:]...)
	stream = append(stream, h.stateRoot[:]...)
	stream = append(stream, h.deltaStateDigest[:]...)
	stream = append(stream, h.receiptRoot[:]...)
	return stream
}

// HeaderLogger returns a new logger with block header fields' value.
func (h Header) HeaderLogger(l *zap.Logger) *zap.Logger {
	return l.With(zap.Uint32("version", h.version),
		zap.Uint32("chainID", h.chainID),
		zap.Uint64("height", h.height),
		zap.Int64("timeStamp", h.timestamp),
		log.Hex("prevBlockHash", h.prevBlockHash[:]),
		log.Hex("txRoot", h.txRoot[:]),
		log.Hex("stateRoot", h.stateRoot[:]),
		log.Hex("deltaStateDigest", h.deltaStateDigest[:]),
	)
}
