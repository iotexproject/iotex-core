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
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// SecretProposal defines the struct of DKG secret proposal
type SecretProposal struct {
	AbstractAction
	secret []uint32
}

// NewSecretProposal returns a SecretProposal instance
func NewSecretProposal(
	nonce uint64,
	sender string,
	recipient string,
	secret []uint32,
) (*SecretProposal, error) {
	if len(sender) == 0 || len(recipient) == 0 {
		return nil, errors.Wrap(ErrAddress, "address of sender or recipient is empty")
	}
	return &SecretProposal{
		AbstractAction: AbstractAction{
			version: version.ProtocolVersion,
			nonce:   nonce,
			srcAddr: sender,
			dstAddr: recipient,
		},
		secret: secret,
	}, nil
}

// Secret returns the secret
func (sp *SecretProposal) Secret() []uint32 { return sp.secret }

// ByteStream returns a raw byte stream of this SecretProposal
func (sp *SecretProposal) ByteStream() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, sp.version)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, sp.nonce)
	stream = append(stream, temp...)
	stream = append(stream, sp.srcAddr...)
	stream = append(stream, sp.dstAddr...)
	for _, s := range sp.secret {
		stream = append(stream, byteutil.Uint32ToBytes(s)...)
	}
	return stream
}

// Proto converts SecretProposal to protobuf's ActionPb
func (sp *SecretProposal) Proto() *iproto.ActionPb {
	// used by account-based model
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_SecretProposal{
			SecretProposal: &iproto.SecretProposalPb{
				Recipient: sp.dstAddr,
				Secret:    sp.secret,
			},
		},
		Version:      sp.version,
		Sender:       sp.srcAddr,
		SenderPubKey: sp.srcPubkey[:],
		Nonce:        sp.nonce,
	}
	return act
}

// Serialize returns a serialized byte stream for the SecretProposal
func (sp *SecretProposal) Serialize() ([]byte, error) {
	return proto.Marshal(sp.Proto())
}

// LoadProto converts a protobuf's ActionPb to SecretProposal
func (sp *SecretProposal) LoadProto(pbAct *iproto.ActionPb) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	srcPub, err := keypair.BytesToPublicKey(pbAct.SenderPubKey)
	if err != nil {
		return err
	}
	if sp == nil {
		return errors.New("nil action to load proto")
	}
	*sp = SecretProposal{}
	pbSecretProposal := pbAct.GetSecretProposal()
	if pbSecretProposal == nil {
		return errors.New("empty SecretProposal action proto to load")
	}

	ab := &Builder{}
	act := ab.SetVersion(pbAct.Version).
		SetNonce(pbAct.Nonce).
		SetSourceAddress(pbAct.Sender).
		SetSourcePublicKey(srcPub).
		SetDestinationAddress(pbSecretProposal.Recipient).
		Build()
	act.SetSignature(pbAct.Signature)
	sp.AbstractAction = act
	sp.secret = pbSecretProposal.Secret
	return nil
}

// Deserialize parses the byte stream into SecretProposal
func (sp *SecretProposal) Deserialize(buf []byte) error {
	pbAct := &iproto.ActionPb{}
	if err := proto.Unmarshal(buf, pbAct); err != nil {
		return err
	}
	return sp.LoadProto(pbAct)
}

// Hash returns the hash of the SecretProposal
func (sp *SecretProposal) Hash() hash.Hash32B {
	return blake2b.Sum256(sp.ByteStream())
}

// IntrinsicGas returns the intrinsic gas of a secret proposal
func (sp *SecretProposal) IntrinsicGas() (uint64, error) { return 0, nil }
