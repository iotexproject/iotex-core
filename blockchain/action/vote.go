// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
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
	stream = append(stream, v.Signature...)
	return stream
}

// ConvertToVotePb converts Vote to protobuf's VotePb
func (v *Vote) ConvertToVotePb() *iproto.VotePb {
	return v.VotePb
}

// Serialize returns a serialized byte stream for the Transfer
func (v *Vote) Serialize() ([]byte, error) {
	return proto.Marshal(v)
}

// ConvertFromVotePb converts Vote to protobuf's VotePb
func (v *Vote) ConvertFromVotePb(pbVote *iproto.VotePb) {
	v.VotePb = pbVote
}

// Deserialize parse the byte stream into Vote
func (v *Vote) Deserialize(buf []byte) error {
	if err := proto.Unmarshal(buf, v); err != nil {
		return err
	}
	return nil
}

// Hash returns the hash of the Vote
func (v *Vote) Hash() common.Hash32B {
	hash := blake2b.Sum256(v.ByteStream())
	return blake2b.Sum256(hash[:])
}
