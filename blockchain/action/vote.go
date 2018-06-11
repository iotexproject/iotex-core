// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// NonceSizeInBytes defines the size of nonce in byte units
	NonceSizeInBytes = 8
	// TimestampSizeInBytes defines the size of 8-byte timestamp
	TimestampSizeInBytes = 8
)

type (
	// Vote defines the struct of account-based vote
	Vote struct {
		*iproto.VotePb
	}
)

// NewVote returns a Vote instance
func NewVote(nonce uint64, selfPubKey []byte, votePubKey []byte) *Vote {
	pbVote := &iproto.VotePb{
		Version: common.ProtocolVersion,

		Nonce:      nonce,
		SelfPubkey: selfPubKey,
		VotePubkey: votePubKey,
	}
	return &Vote{pbVote}
}

// TotalSize returns the total size of this Vote
func (v *Vote) TotalSize() uint32 {
	size := TimestampSizeInBytes
	size += len(v.SelfPubkey)
	size += len(v.VotePubkey)
	size += len(v.Signature)
	return uint32(size)
}

// ByteStream returns a raw byte stream of this Transfer
func (v *Vote) ByteStream() []byte {
	stream := make([]byte, TimestampSizeInBytes)
	common.MachineEndian.PutUint64(stream, v.Timestamp)
	stream = append(stream, v.SelfPubkey...)
	stream = append(stream, v.VotePubkey...)
	// Signature = Sign(hash(ByteStream())), so not included
	return stream
}

// ConvertToVotePb converts Vote to protobuf's VotePb
func (v *Vote) ConvertToVotePb() *iproto.VotePb {
	return v.VotePb
}

// Serialize returns a serialized byte stream for the Transfer
func (v *Vote) Serialize() ([]byte, error) {
	return proto.Marshal(v.ConvertToVotePb())
}

// ConvertFromVotePb converts Vote to protobuf's VotePb
func (v *Vote) ConvertFromVotePb(pbVote *iproto.VotePb) {
	v.VotePb = pbVote
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
func (v *Vote) Hash() common.Hash32B {
	hash := blake2b.Sum256(v.ByteStream())
	return blake2b.Sum256(hash[:])
}

// SignVote signs the Vote using sender's private key
func SignVote(raw []byte, sender *iotxaddress.Address) ([]byte, error) {
	vote := &Vote{}
	if err := vote.Deserialize(raw); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal Vote")
	}
	// check the sender is correct
	if !bytes.Equal(vote.SelfPubkey, sender.PublicKey) {
		return nil, errors.Wrapf(ErrActionError, "signing pubKey %x does not match with Vote pubKey %x",
			vote.SelfPubkey, sender.PublicKey)
	}
	// check the public key is actually owned by sender
	pkhash := iotxaddress.GetPubkeyHash(sender.RawAddress)
	if !bytes.Equal(pkhash, iotxaddress.HashPubKey(sender.PublicKey)) {
		return nil, errors.Wrapf(ErrActionError, "signing addr %s does not own correct public key",
			sender.RawAddress)
	}
	if err := vote.sign(sender); err != nil {
		return nil, err
	}
	return vote.Serialize()
}

// Verify verifies the Vote using sender's public key
func (v *Vote) Verify(sender *iotxaddress.Address) error {
	hash := v.Hash()
	if success := cp.Verify(sender.PublicKey, hash[:], v.Signature); success {
		return nil
	}
	return errors.Wrapf(ErrActionError, "Failed to verify Vote signature = %x", v.Signature)
}

//======================================
// private functions
//======================================

func (v *Vote) sign(sender *iotxaddress.Address) error {
	hash := v.Hash()
	if v.Signature = cp.Sign(sender.PrivateKey, hash[:]); v.Signature != nil {
		return nil
	}
	return errors.Wrapf(ErrActionError, "Failed to sign Vote hash = %x", hash)
}
