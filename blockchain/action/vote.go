// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// VoteIntrinsicGas represents the intrinsic gas for vote
	VoteIntrinsicGas = uint64(10000)
)

// Vote defines the struct of account-based vote
type Vote struct {
	action
}

// NewVote returns a Vote instance
func NewVote(nonce uint64, voterAddress string, voteeAddress string, gasLimit uint64, gasPrice *big.Int) (*Vote, error) {
	if voterAddress == "" {
		return nil, errors.Wrap(ErrAddress, "address of the voter is empty")
	}
	return &Vote{
		action: action{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  voterAddress,
			dstAddr:  voteeAddress,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
	}, nil
}

// Voter returns the voter's address
func (v *Vote) Voter() string {
	return v.SrcAddr()
}

// VoterPublicKey returns the voter's public key
func (v *Vote) VoterPublicKey() keypair.PublicKey {
	return v.SrcPubkey()
}

// SetVoterPublicKey sets the voter's public key
func (v *Vote) SetVoterPublicKey(voterPubkey keypair.PublicKey) {
	v.SetSrcPubkey(voterPubkey)
}

// Votee returns the votee's address
func (v *Vote) Votee() string {
	return v.DstAddr()
}

// TotalSize returns the total size of this Vote
func (v *Vote) TotalSize() uint32 {
	size := TimestampSizeInBytes
	size += NonceSizeInBytes
	size += VersionSizeInBytes
	size += len(v.srcPubkey)
	size += len(v.srcAddr)
	size += len(v.dstAddr)
	size += GasSizeInBytes
	if v.gasPrice != nil && len(v.gasPrice.Bytes()) > 0 {
		size += len(v.gasPrice.Bytes())
	}
	size += len(v.signature)
	return uint32(size)
}

// ByteStream returns a raw byte stream of this Transfer
func (v *Vote) ByteStream() []byte {
	// TODO: remove pbVote.Timestamp from the proto because we never set it
	stream := make([]byte, TimestampSizeInBytes)
	enc.MachineEndian.PutUint64(stream, uint64(0))
	stream = append(stream, v.srcPubkey[:]...)
	stream = append(stream, v.srcAddr...)
	stream = append(stream, v.dstAddr...)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, v.nonce)
	stream = append(stream, temp...)
	temp = make([]byte, 4)
	enc.MachineEndian.PutUint32(temp, v.version)
	stream = append(stream, temp...)
	temp = make([]byte, GasSizeInBytes)
	enc.MachineEndian.PutUint64(temp, v.gasLimit)
	stream = append(stream, temp...)
	if v.gasPrice != nil && len(v.gasPrice.Bytes()) > 0 {
		stream = append(stream, v.gasPrice.Bytes()...)
	}
	// Signature = Sign(hash(ByteStream())), so not included
	return stream
}

// ConvertToActionPb converts Vote to protobuf's ActionPb
func (v *Vote) ConvertToActionPb() *iproto.ActionPb {
	pbVote := &iproto.ActionPb{
		Action: &iproto.ActionPb_Vote{
			Vote: &iproto.VotePb{
				VoterAddress: v.srcAddr,
				SelfPubkey:   v.srcPubkey[:],
				VoteeAddress: v.dstAddr,
			},
		},
		Version:   v.version,
		Nonce:     v.nonce,
		GasLimit:  v.gasLimit,
		Signature: v.signature,
	}
	if v.gasPrice != nil {
		pbVote.GasPrice = v.gasPrice.Bytes()
	}
	return pbVote
}

// ToJSON converts Vote to VoteJSON
func (v *Vote) ToJSON() (*explorer.Vote, error) {
	// used by account-based model
	voterPubKey, err := keypair.BytesToPubKeyString(v.srcPubkey[:])
	if err != nil {
		return nil, err
	}
	vote := &explorer.Vote{
		Version:     int64(v.version),
		Nonce:       int64(v.nonce),
		VoterPubKey: voterPubKey,
		Voter:       v.srcAddr,
		Votee:       v.dstAddr,
		GasLimit:    int64(v.gasLimit),
		GasPrice:    v.gasPrice.Int64(),
		Signature:   hex.EncodeToString(v.signature),
	}
	return vote, nil
}

// Serialize returns a serialized byte stream for the Transfer
func (v *Vote) Serialize() ([]byte, error) {
	return proto.Marshal(v.ConvertToActionPb())
}

// ConvertFromActionPb converts a protobuf's ActionPb to Vote
func (v *Vote) ConvertFromActionPb(pbAct *iproto.ActionPb) {
	v.version = pbAct.Version
	v.nonce = pbAct.Nonce
	v.gasLimit = pbAct.GasLimit
	if v.gasPrice == nil {
		v.gasPrice = big.NewInt(0)
	}
	if len(pbAct.GasPrice) > 0 {
		v.gasPrice.SetBytes(pbAct.GasPrice)
	}
	v.signature = pbAct.Signature
	pbVote := pbAct.GetVote()
	if pbVote != nil {
		v.srcAddr = pbVote.VoterAddress
		v.dstAddr = pbVote.VoteeAddress
		copy(v.srcPubkey[:], pbVote.SelfPubkey)
	}
}

// NewVoteFromJSON creates a new Vote from VoteJSON
func NewVoteFromJSON(jsonVote *explorer.Vote) (*Vote, error) {
	// used by account-based model
	voterPubKey, err := keypair.StringToPubKeyBytes(jsonVote.VoterPubKey)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Vote from VoteJSON")
		return nil, err
	}
	var srcPubkey keypair.PublicKey
	copy(srcPubkey[:], voterPubKey)
	signature, err := hex.DecodeString(jsonVote.Signature)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Vote from VoteJSON")
		return nil, err
	}
	return &Vote{
		action: action{
			version:   uint32(jsonVote.Version),
			nonce:     uint64(jsonVote.Nonce),
			srcAddr:   jsonVote.Voter,
			dstAddr:   jsonVote.Votee,
			gasLimit:  uint64(jsonVote.GasLimit),
			gasPrice:  big.NewInt(jsonVote.GasPrice),
			srcPubkey: srcPubkey,
			signature: signature,
		},
	}, nil
}

// Deserialize parse the byte stream into Vote
func (v *Vote) Deserialize(buf []byte) error {
	pbVote := &iproto.ActionPb{}
	if err := proto.Unmarshal(buf, pbVote); err != nil {
		return err
	}
	v.ConvertFromActionPb(pbVote)
	return nil
}

// Hash returns the hash of the Vote
func (v *Vote) Hash() hash.Hash32B {
	return blake2b.Sum256(v.ByteStream())
}

// IntrinsicGas returns the intrinsic gas of a vote
func (v *Vote) IntrinsicGas() (uint64, error) {
	return VoteIntrinsicGas, nil
}

// Cost returns the total cost of a vote
func (v *Vote) Cost() (*big.Int, error) {
	intrinsicGas, err := v.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the vote")
	}
	voteFee := big.NewInt(0).Mul(v.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return voteFee, nil
}
