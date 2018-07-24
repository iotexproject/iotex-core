// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"encoding/hex"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// ErrVoteError indicates error for a vote action
var ErrVoteError = errors.New("vote error")

const (
	// NonceSizeInBytes defines the size of nonce in byte units
	NonceSizeInBytes = 8
	// TimestampSizeInBytes defines the size of 8-byte timestamp
	TimestampSizeInBytes = 8
	// BooleanSizeInBytes defines the size of booleans
	BooleanSizeInBytes = 1
)

// Vote defines the struct of account-based vote
type Vote struct {
	*iproto.VotePb
}

// NewVote returns a Vote instance
func NewVote(nonce uint64, selfPubKey keypair.PublicKey, votePubKey keypair.PublicKey) *Vote {
	pbVote := &iproto.VotePb{
		Version: version.ProtocolVersion,

		Nonce:      nonce,
		SelfPubkey: selfPubKey[:],
		VotePubkey: votePubKey[:],
	}
	return &Vote{pbVote}
}

// SelfPublicKey returns the self public key of the vote
func (v *Vote) SelfPublicKey() (keypair.PublicKey, error) {
	return keypair.BytesToPublicKey(v.SelfPubkey)
}

// VotePublicKey returns the vote public key of the vote
func (v *Vote) VotePublicKey() (keypair.PublicKey, error) {
	return keypair.BytesToPublicKey(v.VotePubkey)
}

// TotalSize returns the total size of this Vote
func (v *Vote) TotalSize() uint32 {
	size := TimestampSizeInBytes
	size += NonceSizeInBytes
	size += versionSizeInBytes
	size += len(v.SelfPubkey)
	size += len(v.VotePubkey)
	size += len(v.Signature)
	return uint32(size)
}

// ByteStream returns a raw byte stream of this Transfer
func (v *Vote) ByteStream() []byte {
	stream := make([]byte, TimestampSizeInBytes)
	enc.MachineEndian.PutUint64(stream, v.Timestamp)
	stream = append(stream, v.SelfPubkey...)
	stream = append(stream, v.VotePubkey...)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, v.Nonce)
	stream = append(stream, temp...)
	temp = make([]byte, 4)
	enc.MachineEndian.PutUint32(temp, v.Version)
	stream = append(stream, temp...)
	// Signature = Sign(hash(ByteStream())), so not included
	return stream
}

// ConvertToVotePb converts Vote to protobuf's VotePb
func (v *Vote) ConvertToVotePb() *iproto.VotePb {
	return v.VotePb
}

// ToJSON converts Vote to VoteJSON
func (v *Vote) ToJSON() (*explorer.Vote, error) {
	// used by account-based model
	voterPubKey, err := keypair.BytesToPubKeyString(v.SelfPubkey)
	if err != nil {
		return nil, err
	}
	voteePubKey, err := keypair.BytesToPubKeyString(v.VotePubkey)
	if err != nil {
		return nil, err
	}
	vote := &explorer.Vote{
		Version:     int64(v.Version),
		Nonce:       int64(v.Nonce),
		VoterPubKey: voterPubKey,
		VoteePubKey: voteePubKey,
		Signature:   hex.EncodeToString(v.Signature),
	}
	return vote, nil
}

// Serialize returns a serialized byte stream for the Transfer
func (v *Vote) Serialize() ([]byte, error) {
	return proto.Marshal(v.ConvertToVotePb())
}

// ConvertFromVotePb converts a protobuf's VotePb to Vote
func (v *Vote) ConvertFromVotePb(pbVote *iproto.VotePb) {
	v.VotePb = pbVote
}

// NewVoteFromJSON creates a new Vote from VoteJSON
func NewVoteFromJSON(jsonVote *explorer.Vote) (*Vote, error) {
	v := &Vote{}
	v.Version = uint32(jsonVote.Version)
	// used by account-based model
	v.Nonce = uint64(jsonVote.Nonce)
	voterPubKey, err := keypair.StringToPubKeyBytes(jsonVote.VoterPubKey)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Vote from VoteJSON")
		return nil, err
	}
	v.SelfPubkey = voterPubKey
	voteePubKey, err := keypair.StringToPubKeyBytes(jsonVote.VoteePubKey)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Vote from VoteJSON")
		return nil, err
	}
	v.VotePubkey = voteePubKey
	signature, err := hex.DecodeString(jsonVote.Signature)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Vote from VoteJSON")
		return nil, err
	}
	v.Signature = signature

	return v, nil
}

// Deserialize parse the byte stream into Vote
func (v *Vote) Deserialize(buf []byte) error {
	pbVote := &iproto.VotePb{}
	if err := proto.Unmarshal(buf, pbVote); err != nil {
		return err
	}
	v.ConvertFromVotePb(pbVote)
	return nil
}

// Hash returns the hash of the Vote
func (v *Vote) Hash() hash.Hash32B {
	hash := blake2b.Sum256(v.ByteStream())
	return blake2b.Sum256(hash[:])
}

// Sign signs the Vote using sender's private key
func (v *Vote) Sign(sender *iotxaddress.Address) (*Vote, error) {
	// check the sender is correct
	if !bytes.Equal(v.SelfPubkey, sender.PublicKey[:]) {
		return nil, errors.Wrapf(ErrVoteError, "signing pubKey %x does not match with Vote pubKey %x",
			v.SelfPubkey, sender.PublicKey)
	}
	// check the public key is actually owned by sender
	pkhash := iotxaddress.GetPubkeyHash(sender.RawAddress)
	if !bytes.Equal(pkhash, iotxaddress.HashPubKey(sender.PublicKey)) {
		return nil, errors.Wrapf(ErrVoteError, "signing addr %s does not own correct public key",
			sender.RawAddress)
	}
	if err := v.sign(sender); err != nil {
		return nil, err
	}
	return v, nil
}

// Verify verifies the Vote using sender's public key
func (v *Vote) Verify(sender *iotxaddress.Address) error {
	hash := v.Hash()
	if success := cp.Verify(sender.PublicKey, hash[:], v.Signature); success {
		return nil
	}
	return errors.Wrapf(ErrVoteError, "Failed to verify Vote signature = %x", v.Signature)
}

//======================================
// private functions
//======================================

func (v *Vote) sign(sender *iotxaddress.Address) error {
	hash := v.Hash()
	if v.Signature = cp.Sign(sender.PrivateKey, hash[:]); v.Signature != nil {
		return nil
	}
	return errors.Wrapf(ErrVoteError, "Failed to sign Vote hash = %x", hash)
}
