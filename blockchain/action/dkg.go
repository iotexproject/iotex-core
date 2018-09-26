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
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// DKG defines the struct of account-based dkg
type DKG struct {
	action
	secret  []uint32
	witness []byte
}

// NewDKG returns a DKG instance
func NewDKG(
	nonce uint64,
	sender string,
	recipient string,
	secret []uint32,
	witness []byte,
) (*DKG, error) {
	if len(sender) == 0 || len(recipient) == 0 {
		return nil, errors.Wrap(ErrAddress, "address of sender or recipient is empty")
	}
	return &DKG{
		action: action{
			version: version.ProtocolVersion,
			nonce:   nonce,
			srcAddr: sender,
			dstAddr: recipient,
		},
		secret:  secret,
		witness: witness,
	}, nil
}

// ByteStream returns a raw byte stream of this DKG
func (dkg *DKG) ByteStream() []byte {
	stream := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, dkg.version)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, dkg.nonce)
	stream = append(stream, temp...)
	stream = append(stream, dkg.srcAddr...)
	stream = append(stream, dkg.dstAddr...)
	for _, s := range dkg.secret {
		stream = append(stream, byteutil.Uint32ToBytes(s)...)
	}
	stream = append(stream, dkg.witness...)
	return stream
}

// ConvertToActionPb converts DKG to protobuf's ActionPb
func (dkg *DKG) ConvertToActionPb() *iproto.ActionPb {
	// used by account-based model
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_Dkg{
			Dkg: &iproto.DKGPb{
				Sender:    dkg.srcAddr,
				Recipient: dkg.dstAddr,
				Secret:    dkg.secret,
				Witness:   dkg.witness,
			},
		},
		Version: dkg.version,
		Nonce:   dkg.nonce,
	}
	return act
}

// Serialize returns a serialized byte stream for the DKG
func (dkg *DKG) Serialize() ([]byte, error) {
	return proto.Marshal(dkg.ConvertToActionPb())
}

// ConvertFromActionPb converts a protobuf's ActionPb to DKG
func (dkg *DKG) ConvertFromActionPb(pbAct *iproto.ActionPb) {
	dkg.version = pbAct.GetVersion()
	// used by account-based model
	dkg.nonce = pbAct.Nonce

	pbDKG := pbAct.GetDkg()
	dkg.srcAddr = pbDKG.Sender
	dkg.dstAddr = pbDKG.Recipient
	dkg.secret = pbDKG.Secret
	dkg.witness = pbDKG.Witness
}

// Deserialize parses the byte stream into DKG
func (dkg *DKG) Deserialize(buf []byte) error {
	pbAct := &iproto.ActionPb{}
	if err := proto.Unmarshal(buf, pbAct); err != nil {
		return err
	}
	dkg.ConvertFromActionPb(pbAct)
	return nil
}

// Hash returns the hash of the DKG
func (dkg *DKG) Hash() hash.Hash32B {
	return blake2b.Sum256(dkg.ByteStream())
}

// IntrinsicGas returns the intrinsic gas of a dkg
func (dkg *DKG) IntrinsicGas() (uint64, error) { return 0, nil }
