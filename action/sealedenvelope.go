package action

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// SealedEnvelope is a signed action envelope.
	SealedEnvelope struct {
		Envelope
		encoding     iotextypes.Encoding
		evmNetworkID uint32
		srcPubkey    crypto.PublicKey
		signature    []byte
	}

	evmNetworkContextKey struct{}

	// EVMNetworkContext is the context of evm network
	EVMNetworkContext struct {
		// ChainID chain id registered in evm network
		ChainID uint32
	}
)

// WithEVMNetworkContext add evm network information into context.
func WithEVMNetworkContext(ctx context.Context, enc EVMNetworkContext) context.Context {
	return context.WithValue(ctx, evmNetworkContextKey{}, enc)
}

// ExtractEVMNetworkContext gets EVMNetworkContext
func ExtractEVMNetworkContext(ctx context.Context) (EVMNetworkContext, bool) {
	enc, ok := ctx.Value(evmNetworkContextKey{}).(EVMNetworkContext)
	return enc, ok
}

// MustExtractEVMNetworkContext must extract EVMNetworkdContext, else panic
func MustExtractEVMNetworkContext(ctx context.Context) EVMNetworkContext {
	enc, ok := ctx.Value(evmNetworkContextKey{}).(EVMNetworkContext)
	if !ok {
		log.S().Panic("Miss evm network context")
	}
	return enc
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
		return rlpRawHash(tx, sealed.evmNetworkID)
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.Hash256b(byteutil.Must(proto.Marshal(sealed.Envelope.Proto()))), nil
	}
	return hash.ZeroHash256, errors.Errorf("unknown encoding type %s", sealed.encoding)
}

// Hash returns the hash value of SealedEnvelope.
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) Hash() hash.Hash256 {
	switch sealed.encoding {
	case iotextypes.Encoding_ETHEREUM_RLP:
		tx, err := actionToRLP(sealed.Action())
		if err != nil {
			panic(err)
		}
		h, err := rlpSignedHash(tx, sealed.evmNetworkID, sealed.Signature())
		if err != nil {
			panic(err)
		}
		return h
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.Hash256b(byteutil.Must(proto.Marshal(sealed.Proto())))
	}
	panic("unknown encoding type")
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
func (sealed *SealedEnvelope) LoadProto(ctx context.Context, pbAct *iotextypes.Action) error {
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
	case iotextypes.Encoding_ETHEREUM_RLP:
		// verify action type can support RLP-encoding
		tx, err := actionToRLP(elp.Action())
		if err != nil {
			return err
		}
		enc, ok := ExtractEVMNetworkContext(ctx)
		if !ok {
			return errors.New("evm network chain id is not specified")
		}
		if _, err = rlpSignedHash(tx, enc.ChainID, pbAct.GetSignature()); err != nil {
			return err
		}
		sealed.evmNetworkID = enc.ChainID
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		break
	default:
		return errors.Errorf("unknown encoding type %s", encoding)
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
