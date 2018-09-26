// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// SecretWitness defines the struct of DKG secret witness
type SecretWitness struct {
	action
	witness [][]byte
}

// NewSecretWitness returns a SecretWitness instance
func NewSecretWitness(
	nonce uint64,
	sender string,
	witness [][]byte,
) (*SecretWitness, error) {
	if len(sender) == 0 {
		return nil, errors.Wrap(ErrAddress, "address of sender is empty")
	}
	return &SecretWitness{
		action: action{
			version: version.ProtocolVersion,
			nonce:   nonce,
			srcAddr: sender,
		},
		witness: witness,
	}, nil
}

// ByteStream returns a raw byte stream of this SecretWitness
func (sw *SecretWitness) ByteStream() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, sw.version)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, sw.nonce)
	stream = append(stream, temp...)
	stream = append(stream, sw.srcAddr...)
	for _, w := range sw.witness {
		stream = append(stream, w...)
	}
	return stream
}

// ConvertToActionPb converts SecretWitness to protobuf's ActionPb
func (sw *SecretWitness) ConvertToActionPb() *iproto.ActionPb {
	// used by account-based model
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_SecretWitness{
			SecretWitness: &iproto.SecretWitnessPb{
				Sender:  sw.srcAddr,
				Witness: sw.witness,
			},
		},
		Version: sw.version,
		Nonce:   sw.nonce,
	}
	return act
}

// Serialize returns a serialized byte stream for the SecretWitness
func (sw *SecretWitness) Serialize() ([]byte, error) {
	return proto.Marshal(sw.ConvertToActionPb())
}

// ConvertFromActionPb converts a protobuf's ActionPb to SecretWitness
func (sw *SecretWitness) ConvertFromActionPb(pbAct *iproto.ActionPb) {
	sw.version = pbAct.GetVersion()
	// used by account-based model
	sw.nonce = pbAct.Nonce

	pbSecretWitness := pbAct.GetSecretWitness()
	sw.srcAddr = pbSecretWitness.Sender
	sw.witness = pbSecretWitness.Witness
}

// Deserialize parses the byte stream into SecretWitness
func (sw *SecretWitness) Deserialize(buf []byte) error {
	pbAct := &iproto.ActionPb{}
	if err := proto.Unmarshal(buf, pbAct); err != nil {
		return err
	}
	sw.ConvertFromActionPb(pbAct)
	return nil
}

// Hash returns the hash of the SecretWitness
func (sw *SecretWitness) Hash() hash.Hash32B {
	return blake2b.Sum256(sw.ByteStream())
}

// IntrinsicGas returns the intrinsic gas of a secret witness
func (sw *SecretWitness) IntrinsicGas() (uint64, error) { return 0, nil }
