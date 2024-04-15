package action

import (
	"encoding/hex"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope
	encoding     iotextypes.Encoding
	evmNetworkID uint32
	srcPubkey    crypto.PublicKey
	signature    []byte
	srcAddress   address.Address
	hash         hash.Hash256
}

// envelopeHash returns the raw hash of embedded Envelope (this is the hash to be signed)
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) envelopeHash() (hash.Hash256, error) {
	switch sealed.encoding {
	case iotextypes.Encoding_ETHEREUM_EIP155, iotextypes.Encoding_ETHEREUM_UNPROTECTED:
		act, ok := sealed.Action().(EthCompatibleAction)
		if !ok {
			return hash.ZeroHash256, ErrInvalidAct
		}
		tx, err := act.ToEthTx(sealed.evmNetworkID)
		if err != nil {
			return hash.ZeroHash256, err
		}
		signer, err := NewEthSigner(sealed.encoding, sealed.evmNetworkID)
		if err != nil {
			return hash.ZeroHash256, err
		}
		return rlpRawHash(tx, signer)
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.Hash256b(byteutil.Must(proto.Marshal(sealed.Envelope.Proto()))), nil
	default:
		return hash.ZeroHash256, errors.Errorf("unknown encoding type %v", sealed.encoding)
	}
}

// Hash returns the hash value of SealedEnvelope.
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) Hash() (hash.Hash256, error) {
	if sealed.hash == hash.ZeroHash256 {
		hashVal, hashErr := sealed.calcHash()
		if hashErr == nil {
			sealed.hash = hashVal
		}
		return sealed.hash, hashErr
	}
	return sealed.hash, nil
}

func (sealed *SealedEnvelope) calcHash() (hash.Hash256, error) {
	switch sealed.encoding {
	case iotextypes.Encoding_ETHEREUM_EIP155, iotextypes.Encoding_ETHEREUM_UNPROTECTED:
		act, ok := sealed.Action().(EthCompatibleAction)
		if !ok {
			return hash.ZeroHash256, ErrInvalidAct
		}
		tx, err := act.ToEthTx(sealed.evmNetworkID)
		if err != nil {
			return hash.ZeroHash256, err
		}
		signer, err := NewEthSigner(sealed.encoding, sealed.evmNetworkID)
		if err != nil {
			return hash.ZeroHash256, err
		}
		return rlpSignedHash(tx, signer, sealed.Signature())
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.Hash256b(byteutil.Must(proto.Marshal(sealed.Proto()))), nil
	default:
		return hash.ZeroHash256, errors.Errorf("unknown encoding type %v", sealed.encoding)
	}
}

// SrcPubkey returns the source public key
func (sealed *SealedEnvelope) SrcPubkey() crypto.PublicKey { return sealed.srcPubkey }

// SenderAddress returns address of the source public key
func (sealed *SealedEnvelope) SenderAddress() address.Address {
	if sealed.srcAddress == nil {
		sealed.srcAddress = sealed.srcPubkey.Address()
	}
	return sealed.srcAddress
}

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

// loadProto loads from proto scheme.
func (sealed *SealedEnvelope) loadProto(pbAct *iotextypes.Action, evmID uint32) error {
	if pbAct == nil {
		return ErrNilProto
	}
	if sealed == nil {
		return ErrNilAction
	}
	sigSize := len(pbAct.GetSignature())
	if sigSize != 65 {
		return errors.Errorf("invalid signature length = %d, expecting 65", sigSize)
	}

	var elp Envelope = &envelope{}
	if err := elp.LoadProto(pbAct.GetCore()); err != nil {
		return err
	}
	// populate pubkey and signature
	srcPub, err := crypto.BytesToPublicKey(pbAct.GetSenderPubKey())
	if err != nil {
		return err
	}
	encoding := pbAct.GetEncoding()
	switch encoding {
	case iotextypes.Encoding_ETHEREUM_EIP155, iotextypes.Encoding_ETHEREUM_UNPROTECTED:
		// verify action type can support RLP-encoding
		act, ok := elp.Action().(EthCompatibleAction)
		if !ok {
			return ErrInvalidAct
		}
		tx, err := act.ToEthTx(evmID)
		if err != nil {
			return err
		}
		signer, err := NewEthSigner(encoding, evmID)
		if err != nil {
			return err
		}
		if _, err = rlpSignedHash(tx, signer, pbAct.GetSignature()); err != nil {
			return err
		}
		sealed.evmNetworkID = evmID
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		break
	default:
		return errors.Errorf("unknown encoding type %v", encoding)
	}

	// clear 'sealed' and populate new value
	sealed.Envelope = elp
	sealed.srcPubkey = srcPub
	sealed.signature = make([]byte, sigSize)
	copy(sealed.signature, pbAct.GetSignature())
	sealed.encoding = encoding
	sealed.hash = hash.ZeroHash256
	sealed.srcAddress = nil
	return nil
}

// VerifySignature verifies the action using sender's public key
func (sealed *SealedEnvelope) VerifySignature() error {
	if sealed.SrcPubkey() == nil {
		return errors.New("empty public key")
	}
	h, err := sealed.envelopeHash()
	if err != nil {
		return errors.Wrap(err, "failed to generate envelope hash")
	}
	if !sealed.SrcPubkey().Verify(h[:], sealed.Signature()) {
		log.L().Info("failed to verify action hash",
			zap.String("hash", hex.EncodeToString(h[:])),
			zap.String("signature", hex.EncodeToString(sealed.Signature())))
		return ErrInvalidSender
	}
	return nil
}
