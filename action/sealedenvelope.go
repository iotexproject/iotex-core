package action

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope
	encoding      iotextypes.Encoding
	externChainID uint32
	srcPubkey     crypto.PublicKey
	signature     []byte
}

// envelopeHash returns the raw hash of embedded Envelope (this is the hash to be signed)
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) envelopeHash() (hash.Hash256, error) {
	switch sealed.encoding {
	case iotextypes.Encoding_ETHEREUM_RLP:
		tx, err := actionToRLP(sealed.Action())
		if err != nil {
			return hash.ZeroHash256, err
		}
		return rlpRawHash(tx, sealed.externChainID)
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.BytesToHash256(byteutil.Must(proto.Marshal(sealed.Envelope.Proto()))), nil
	}
	return hash.ZeroHash256, errors.Errorf("unknown encoding type %s", sealed.encoding)
}

// Hash returns the hash value of SealedEnvelope.
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) Hash() hash.Hash256 {
	if IsRLP(sealed.encoding) {
		tx, err := actionToRLP(sealed.Action())
		if err != nil {
			return hash.ZeroHash256
		}
		h, err := rlpSignedHash(tx, sealed.externChainID, sealed.Signature())
		if err != nil {
			return hash.ZeroHash256
		}
		return h
	}
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

// Encoding returns the encoding
func (sealed *SealedEnvelope) Encoding() uint32 {
	return uint32(sealed.encoding)
}

// Proto converts it to it's proto scheme.
func (sealed *SealedEnvelope) Proto() *iotextypes.Action {
	return &iotextypes.Action{
		Core:         sealed.Envelope.Proto(),
		SenderPubKey: sealed.srcPubkey.Bytes(),
		Signature:    sealed.signature,
		Encoding:     sealed.encoding,
	}
}

// LoadProto loads from proto scheme.
func (sealed *SealedEnvelope) LoadProto(pbAct *iotextypes.Action) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if sealed == nil {
		return errors.New("nil action to load proto")
	}
	sigSize := len(pbAct.GetSignature())
	if sigSize != 65 {
		return errors.Errorf("invalid signature length = %d, expecting 65", sigSize)
	}

	encoding := pbAct.GetEncoding()
	if !IsValidEncoding(encoding) {
		return errors.Errorf("invalid encoding = %v", encoding)
	}
	var elp Envelope = &envelope{}
	if err := elp.LoadProto(pbAct.GetCore()); err != nil {
		return err
	}
	if IsRLP(encoding) {
		// verify action type can support RLP-encoding
		if _, err := actionToRLP(elp.Action()); err != nil {
			return err
		}
		sealed.externChainID = config.ExternChainID()
	}

	// populate pubkey and signature
	srcPub, err := crypto.BytesToPublicKey(pbAct.GetSenderPubKey())
	if err != nil {
		return err
	}

	// clear 'sealed' and populate new value
	sealed.Envelope = elp
	sealed.srcPubkey = srcPub
	sealed.signature = make([]byte, sigSize)
	copy(sealed.signature, pbAct.GetSignature())
	sealed.encoding = encoding
	elp.Action().SetEnvelopeContext(*sealed)
	return nil
}

func actionToRLP(action Action) (rlpTransaction, error) {
	var tx rlpTransaction
	switch act := action.(type) {
	case *Transfer:
		tx = (*Transfer)(act)
	case *Execution:
		tx = (*Execution)(act)
	default:
		return nil, errors.Errorf("invalid action type %T not supported", act)
	}
	return tx, nil
}

// IsValidEncoding checks whether the encoding is valid or not
func IsValidEncoding(enc iotextypes.Encoding) bool {
	return enc == iotextypes.Encoding_IOTEX_PROTOBUF || enc == iotextypes.Encoding_ETHEREUM_RLP
}

// IsRLP returns whether the tx is RLP-encoded or not
func IsRLP(enc iotextypes.Encoding) bool {
	return enc == iotextypes.Encoding_ETHEREUM_RLP
}
