package action

import (
	"sync"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope
	encoding     iotextypes.Encoding
	evmNetworkID uint32
	srcPubkey    crypto.PublicKey
	signature    []byte
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return &iotextypes.Action{}
	},
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
	default:
		return hash.ZeroHash256, errors.Errorf("unknown encoding type %v", sealed.encoding)
	}
}

// Hash returns the hash value of SealedEnvelope.
// an all-0 return value means the transaction is invalid
func (sealed *SealedEnvelope) Hash() (hash.Hash256, error) {
	switch sealed.encoding {
	case iotextypes.Encoding_ETHEREUM_RLP:
		tx, err := actionToRLP(sealed.Action())
		if err != nil {
			return hash.ZeroHash256, err
		}
		return rlpSignedHash(tx, sealed.evmNetworkID, sealed.Signature())
	case iotextypes.Encoding_IOTEX_PROTOBUF:
		return hash.Hash256b(byteutil.Must(sealed.serialize())), nil
	default:
		return hash.ZeroHash256, errors.Errorf("unknown encoding type %v", sealed.encoding)
	}
}

func (sealed *SealedEnvelope) serialize() ([]byte, error) {
	buf := bufPool.Get().(*iotextypes.Action)
	buf.Core = sealed.Envelope.Proto()
	buf.SenderPubKey = sealed.srcPubkey.Bytes()
	buf.Signature = sealed.signature
	buf.Encoding = sealed.encoding
	ret, err := proto.Marshal(buf)
	bufPool.Put(buf)
	return ret, err
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
		return ErrEmptyActionPool
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
	case iotextypes.Encoding_ETHEREUM_RLP:
		// verify action type can support RLP-encoding
		tx, err := actionToRLP(elp.Action())
		if err != nil {
			return err
		}
		if _, err = rlpSignedHash(tx, config.EVMNetworkID(), pbAct.GetSignature()); err != nil {
			return err
		}
		sealed.evmNetworkID = config.EVMNetworkID()
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
	elp.Action().SetEnvelopeContext(*sealed)
	return nil
}

func actionToRLP(action Action) (rlpTransaction, error) {
	var (
		err error
		tx  rlpTransaction
	)
	switch act := action.(type) {
	case *Transfer:
		tx = (*Transfer)(act)
	case *Execution:
		tx = (*Execution)(act)
	case *CreateStake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingCreateAddrHash[:], act.Proto())
	case *DepositToStake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingAddDepositAddrHash[:], act.Proto())
	case *ChangeCandidate:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingChangeCandAddrHash[:], act.Proto())
	case *Unstake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingUnstakeAddrHash[:], act.Proto())
	case *WithdrawStake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingWithdrawAddrHash[:], act.Proto())
	case *Restake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingRestakeAddrHash[:], act.Proto())
	case *TransferStake:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingTransferAddrHash[:], act.Proto())
	case *CandidateRegister:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingRegisterCandAddrHash[:], act.Proto())
	case *CandidateUpdate:
		tx, err = wrapStakingActionIntoExecution(act.AbstractAction, address.StakingUpdateCandAddrHash[:], act.Proto())
	default:
		return nil, errors.Errorf("invalid action type %T not supported", act)
	}
	return tx, err
}

func wrapStakingActionIntoExecution(ab AbstractAction, toAddr []byte, pb proto.Message) (rlpTransaction, error) {
	addr, err := address.FromBytes(toAddr[:])
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return &Execution{
		AbstractAction: ab,
		contract:       addr.String(),
		data:           data,
	}, nil
}
