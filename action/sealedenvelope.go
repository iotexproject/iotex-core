package action

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope

	srcPubkey crypto.PublicKey
	signature []byte
}

// Hash returns the hash value of SealedEnvelope.
func (sealed *SealedEnvelope) Hash() hash.Hash256 {
	return hash.Hash256b(byteutil.Must(proto.Marshal(sealed.Proto())))
}

// SrcPubkey returns the source public key
func (sealed *SealedEnvelope) SrcPubkey() crypto.PublicKey { return sealed.srcPubkey }

// Signature returns signature bytes
func (sealed *SealedEnvelope) Signature() []byte {
	sig := make([]byte, len(sealed.signature))
	copy(sig, sealed.signature)
	return sig
}

// Proto converts it to it's proto scheme.
func (sealed *SealedEnvelope) Proto() *iotextypes.Action {
	return &iotextypes.Action{
		Core:         sealed.Envelope.Proto(),
		SenderPubKey: sealed.srcPubkey.Bytes(),
		Signature:    sealed.signature,
	}
}

// LoadProto loads from proto scheme.
func (sealed *SealedEnvelope) LoadProto(pbAct *iotextypes.Action) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	srcPub, err := crypto.BytesToPublicKey(pbAct.GetSenderPubKey())
	if err != nil {
		return err
	}
	if sealed == nil {
		return errors.New("nil action to load proto")
	}
	*sealed = SealedEnvelope{}

	sealed.srcPubkey = srcPub
	sealed.signature = make([]byte, len(pbAct.GetSignature()))
	copy(sealed.signature, pbAct.GetSignature())
	if err := sealed.Envelope.LoadProto(pbAct.GetCore()); err != nil {
		return err
	}

	sealed.payload.SetEnvelopeContext(*sealed)
	return nil
}
