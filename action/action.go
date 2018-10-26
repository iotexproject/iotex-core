// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/crypto"
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

// Action is the generic interface of all types of actions, and defines the common methods of them
type Action interface {
	Version() uint32
	Nonce() uint64
	SrcAddr() string
	SrcPubkey() keypair.PublicKey
	SetSrcPubkey(srcPubkey keypair.PublicKey)
	DstAddr() string
	GasLimit() uint64
	GasPrice() *big.Int
	Signature() []byte
	SetSignature(signature []byte)
	ByteStream() []byte
	Hash() hash.Hash32B
	IntrinsicGas() (uint64, error)
	Cost() (*big.Int, error)
	Proto() *iproto.ActionPb
	LoadProto(*iproto.ActionPb)
}

// abstractAction is an abstract implementation of Action interface
type abstractAction struct {
	version   uint32
	nonce     uint64
	srcAddr   string
	srcPubkey keypair.PublicKey
	dstAddr   string
	gasLimit  uint64
	gasPrice  *big.Int
	signature []byte
}

// Version returns the version
func (act *abstractAction) Version() uint32 { return act.version }

// Nonce returns the nonce
func (act *abstractAction) Nonce() uint64 { return act.nonce }

// SrcAddr returns the source address
func (act *abstractAction) SrcAddr() string { return act.srcAddr }

// SrcPubkey returns the source public key
func (act *abstractAction) SrcPubkey() keypair.PublicKey { return act.srcPubkey }

// SetSrcPubkey sets the source public key
func (act *abstractAction) SetSrcPubkey(srcPubkey keypair.PublicKey) { act.srcPubkey = srcPubkey }

// DstAddr returns the destination address
func (act *abstractAction) DstAddr() string { return act.dstAddr }

// GasLimit returns the gas limit
func (act *abstractAction) GasLimit() uint64 { return act.gasLimit }

// GasPrice returns the gas price
func (act *abstractAction) GasPrice() *big.Int { return act.gasPrice }

// Signature returns signature bytes
func (act *abstractAction) Signature() []byte { return act.signature }

// SetSignature sets the signature bytes
func (act *abstractAction) SetSignature(signature []byte) { act.signature = signature }

// BasicActionSize returns the basic size of action
func (act *abstractAction) BasicActionSize() uint32 {
	// VersionSizeInBytes + NonceSizeInBytes + GasSizeInBytes
	size := 4 + 8 + 8
	size += len(act.srcAddr) + len(act.dstAddr) + len(act.srcPubkey) + len(act.signature)
	if act.gasPrice != nil && len(act.gasPrice.Bytes()) > 0 {
		size += len(act.gasPrice.Bytes())
	}

	return uint32(size)
}

// BasicActionByteStream returns the basic byte stream of action
func (act *abstractAction) BasicActionByteStream() []byte {
	stream := byteutil.Uint32ToBytes(act.version)
	stream = append(stream, byteutil.Uint64ToBytes(act.nonce)...)
	stream = append(stream, act.srcAddr...)
	stream = append(stream, act.dstAddr...)
	stream = append(stream, act.srcPubkey[:]...)
	stream = append(stream, byteutil.Uint64ToBytes(act.gasLimit)...)
	if act.gasPrice != nil && len(act.gasPrice.Bytes()) > 0 {
		stream = append(stream, act.gasPrice.Bytes()...)
	}

	return stream
}

// Sign signs the action using sender's private key
func Sign(act Action, sk keypair.PrivateKey) error {
	// TODO: remove this conversion once we deprecate old address format
	srcAddr, err := address.IotxAddressToAddress(act.SrcAddr())
	if err != nil {
		return errors.Wrapf(err, "error when converting from old address format")
	}
	// TODO: we should avoid generate public key from private key in each signature
	pk, err := crypto.EC283.NewPubKey(sk)
	if err != nil {
		return errors.Wrapf(err, "error when deriving public key from private key")
	}
	pkHash := keypair.HashPubKey(pk)
	// TODO: abstract action shouldn't be aware that the playload is the hash of public key
	if !bytes.Equal(srcAddr.Payload(), pkHash[:]) {
		return errors.Wrapf(
			ErrAction,
			"signer public key hash %x does not match action source address payload %x",
			pkHash,
			srcAddr.Payload(),
		)
	}
	act.SetSrcPubkey(pk)
	hash := act.Hash()
	if act.SetSignature(crypto.EC283.Sign(sk, hash[:])); act.Signature() == nil {
		return errors.Wrapf(ErrAction, "failed to sign action hash = %x", hash)
	}
	return nil
}

// Verify verifies the action using sender's public key
func Verify(act Action) error {
	hash := act.Hash()
	if success := crypto.EC283.Verify(act.SrcPubkey(), hash[:], act.Signature()); success {
		return nil
	}
	return errors.Wrapf(
		ErrAction,
		"failed to verify action hash = %x and signature = %x",
		act.Hash(),
		act.Signature(),
	)
}

// ClassifyActions classfies actions
func ClassifyActions(actions []Action) ([]*Transfer, []*Vote, []*Execution) {
	transfers := make([]*Transfer, 0)
	votes := make([]*Vote, 0)
	executions := make([]*Execution, 0)
	for _, act := range actions {
		switch act.(type) {
		case *Transfer:
			transfers = append(transfers, act.(*Transfer))
		case *Vote:
			votes = append(votes, act.(*Vote))
		case *Execution:
			executions = append(executions, act.(*Execution))
		}
	}
	return transfers, votes, executions
}
