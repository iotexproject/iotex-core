// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrAction indicates error for an action
	ErrAction = errors.New("action error")
	// ErrAddress indicates error of address
	ErrAddress = errors.New("address error")
)

// Action is the action can be Executed in protocols. The method is added to avoid mistakenly used empty interface as action.
type Action interface {
	SetEnvelopeContext(SealedEnvelope)
}

type actionPayload interface {
	ByteStream() []byte
	Cost() (*big.Int, error)
	IntrinsicGas() (uint64, error)
	SetEnvelopeContext(SealedEnvelope)
}

// Envelope defines an envelope wrapped on action with some envelope metadata.
type Envelope struct {
	version  uint32
	nonce    uint64
	dstAddr  string
	gasLimit uint64
	payload  actionPayload
	gasPrice *big.Int
}

// SealedEnvelope is a signed action envelope.
type SealedEnvelope struct {
	Envelope

	srcAddr   string
	srcPubkey keypair.PublicKey
	signature []byte
}

// Version returns the version
func (act *Envelope) Version() uint32 { return act.version }

// Nonce returns the nonce
func (act *Envelope) Nonce() uint64 { return act.nonce }

// DstAddr returns the destination address
func (act *Envelope) DstAddr() string { return act.dstAddr }

// GasLimit returns the gas limit
func (act *Envelope) GasLimit() uint64 { return act.gasLimit }

// GasPrice returns the gas price
func (act *Envelope) GasPrice() *big.Int {
	p := &big.Int{}
	if act.gasPrice == nil {
		return p
	}
	return p.Set(act.gasPrice)
}

// Cost returns cost of actions
func (act *Envelope) Cost() (*big.Int, error) {
	return act.payload.Cost()
}

// IntrinsicGas returns intrinsic gas of action.
func (act *Envelope) IntrinsicGas() (uint64, error) {
	return act.payload.IntrinsicGas()
}

// Action returns the action payload.
func (act *Envelope) Action() Action { return act.payload }

// ByteStream returns encoded binary.
func (act *Envelope) ByteStream() []byte {
	stream := byteutil.Uint32ToBytes(act.version)
	stream = append(stream, byteutil.Uint64ToBytes(act.nonce)...)
	stream = append(stream, act.dstAddr...)
	stream = append(stream, byteutil.Uint64ToBytes(act.gasLimit)...)
	if act.gasPrice != nil {
		stream = append(stream, act.gasPrice.Bytes()...)
	}
	payload := act.payload.ByteStream()
	stream = append(stream, payload...)
	return stream
}

// Hash returns the hash value of SealedEnvelope.
func (sealed *SealedEnvelope) Hash() hash.Hash32B {
	stream := sealed.Envelope.ByteStream()
	stream = append(stream, sealed.srcAddr...)
	stream = append(stream, sealed.srcPubkey[:]...)
	return blake2b.Sum256(stream)
}

// SrcAddr returns the source address
func (sealed *SealedEnvelope) SrcAddr() string { return sealed.srcAddr }

// SrcPubkey returns the source public key
func (sealed *SealedEnvelope) SrcPubkey() keypair.PublicKey { return sealed.srcPubkey }

// Signature returns signature bytes
func (sealed *SealedEnvelope) Signature() []byte {
	sig := make([]byte, len(sealed.signature))
	copy(sig, sealed.signature)
	return sig
}

// Proto converts it to it's proto scheme.
func (sealed SealedEnvelope) Proto() *iproto.ActionPb {
	elp := sealed.Envelope
	actPb := &iproto.ActionPb{
		Version:      elp.version,
		Nonce:        elp.nonce,
		GasLimit:     elp.gasLimit,
		Sender:       sealed.srcAddr,
		SenderPubKey: sealed.srcPubkey[:],
		Signature:    sealed.signature,
	}
	if elp.gasPrice != nil {
		actPb.GasPrice = elp.gasPrice.Bytes()
	}

	// TODO assert each action
	act := sealed.Action()
	switch act := act.(type) {
	case *Transfer:
		actPb.Action = &iproto.ActionPb_Transfer{Transfer: act.Proto()}
	case *Vote:
		actPb.Action = &iproto.ActionPb_Vote{Vote: act.Proto()}
	case *Execution:
		actPb.Action = &iproto.ActionPb_Execution{Execution: act.Proto()}
	case *PutBlock:
		actPb.Action = &iproto.ActionPb_PutBlock{PutBlock: act.Proto()}
	case *StartSubChain:
		actPb.Action = &iproto.ActionPb_StartSubChain{StartSubChain: act.Proto()}
	case *StopSubChain:
		actPb.Action = &iproto.ActionPb_StopSubChain{StopSubChain: act.Proto()}
	case *CreateDeposit:
		actPb.Action = &iproto.ActionPb_CreateDeposit{CreateDeposit: act.Proto()}
	case *SettleDeposit:
		actPb.Action = &iproto.ActionPb_SettleDeposit{SettleDeposit: act.Proto()}
	default:
		logger.Panic().Msgf("cannot convert type of action %T \r\n", act)
	}
	return actPb
}

// LoadProto loads from proto scheme.
func (sealed *SealedEnvelope) LoadProto(pbAct *iproto.ActionPb) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	srcPub, err := keypair.BytesToPublicKey(pbAct.SenderPubKey)
	if err != nil {
		return err
	}
	if sealed == nil {
		return errors.New("nil action to load proto")
	}
	*sealed = SealedEnvelope{}

	sealed.srcAddr = pbAct.Sender
	sealed.srcPubkey = srcPub
	sealed.signature = make([]byte, len(pbAct.Signature))
	copy(sealed.signature, pbAct.Signature)
	sealed.version = pbAct.Version
	sealed.nonce = pbAct.Nonce
	sealed.gasLimit = pbAct.GasLimit
	sealed.gasPrice = &big.Int{}
	sealed.gasPrice.SetBytes(pbAct.GetGasPrice())

	if pbAct.GetTransfer() != nil {
		sealed.dstAddr = pbAct.GetTransfer().Recipient
		act := &Transfer{}
		if err := act.LoadProto(pbAct.GetTransfer()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetVote() != nil {
		sealed.dstAddr = pbAct.GetVote().VoteeAddress
		act := &Vote{}
		if err := act.LoadProto(pbAct.GetVote()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetExecution() != nil {
		sealed.dstAddr = pbAct.GetExecution().Contract
		act := &Execution{}
		if err := act.LoadProto(pbAct.GetExecution()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetPutBlock() != nil {
		sealed.dstAddr = pbAct.GetPutBlock().SubChainAddress
		act := &PutBlock{}
		if err := act.LoadProto(pbAct.GetPutBlock()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetStartSubChain() != nil {
		act := &StartSubChain{}
		if err := act.LoadProto(pbAct.GetStartSubChain()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetStopSubChain() != nil {
		sealed.dstAddr = pbAct.GetStopSubChain().SubChainAddress
		act := &StopSubChain{}
		if err := act.LoadProto(pbAct.GetStopSubChain()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetCreateDeposit() != nil {
		sealed.dstAddr = pbAct.GetCreateDeposit().Recipient
		act := &CreateDeposit{}
		if err := act.LoadProto(pbAct.GetCreateDeposit()); err != nil {
			return err
		}
		sealed.payload = act
	} else if pbAct.GetSettleDeposit() != nil {
		sealed.dstAddr = pbAct.GetSettleDeposit().Recipient
		act := &SettleDeposit{}
		if err := act.LoadProto(pbAct.GetSettleDeposit()); err != nil {
			return err
		}
		sealed.payload = act
	} else {
		return errors.New("no appliable action to handle in action proto")
	}
	sealed.payload.SetEnvelopeContext(*sealed)
	return nil
}

// Sign signs the action using sender's private key
func Sign(act Envelope, addr string, sk keypair.PrivateKey) (SealedEnvelope, error) {
	sealed := SealedEnvelope{Envelope: act}

	// TODO: we should avoid generate public key from private key in each signature
	pk, err := crypto.EC283.NewPubKey(sk)
	if err != nil {
		return sealed, errors.Wrapf(err, "error when deriving public key from private key")
	}

	sealed.srcAddr = addr
	sealed.srcPubkey = pk
	// the reason to set context here is because some actions use envelope information in their proto define. for example transfer use des addr as Receipt. This will change hash value.
	sealed.payload.SetEnvelopeContext(sealed)

	hash := sealed.Hash()
	sig := crypto.EC283.Sign(sk, hash[:])
	if len(sig) == 0 {
		return sealed, errors.Wrapf(ErrAction, "failed to sign action hash = %x", hash)
	}
	sealed.signature = sig
	return sealed, nil
}

// FakeSeal creates a SealedActionEnvelope without signature.
// This method should be only used in tests.
func FakeSeal(act Envelope, addr string, pubk keypair.PublicKey) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcAddr:   addr,
		srcPubkey: pubk,
	}
	sealed.payload.SetEnvelopeContext(sealed)
	return sealed
}

// AssembleSealedEnvelope assembles a SealedEnvelope use Envelope, Sender Address and Signature.
// This method should be only used in tests.
func AssembleSealedEnvelope(act Envelope, addr string, pk keypair.PublicKey, sig []byte) SealedEnvelope {
	sealed := SealedEnvelope{
		Envelope:  act,
		srcAddr:   addr,
		srcPubkey: pk,
		signature: sig,
	}
	sealed.payload.SetEnvelopeContext(sealed)
	return sealed
}

// Verify verifies the action using sender's public key
func Verify(sealed SealedEnvelope) error {
	hash := sealed.Hash()
	if success := crypto.EC283.Verify(sealed.SrcPubkey(), hash[:], sealed.Signature()); success {
		return nil
	}
	return errors.Wrapf(
		ErrAction,
		"failed to verify action hash = %x and signature = %x",
		hash,
		sealed.Signature(),
	)
}

// ClassifyActions classfies actions
func ClassifyActions(actions []SealedEnvelope) ([]*Transfer, []*Vote, []*Execution) {
	tsfs := make([]*Transfer, 0)
	votes := make([]*Vote, 0)
	exes := make([]*Execution, 0)
	for _, elp := range actions {
		act := elp.Action()
		switch act := act.(type) {
		case *Transfer:
			tsfs = append(tsfs, act)
		case *Vote:
			votes = append(votes, act)
		case *Execution:
			exes = append(exes, act)
		}
	}
	return tsfs, votes, exes
}
