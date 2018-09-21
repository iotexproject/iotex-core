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

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
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
	Hash() hash.Hash32B
	IntrinsicGas() (uint64, error)
}

type action struct {
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
func (act *action) Version() uint32 { return act.version }

// Nonce returns the nonce
func (act *action) Nonce() uint64 { return act.nonce }

// SrcAddr returns the source address
func (act *action) SrcAddr() string { return act.srcAddr }

// SrcPubkey returns the source public key
func (act *action) SrcPubkey() keypair.PublicKey { return act.srcPubkey }

// SetSrcPubkey sets the source public key
func (act *action) SetSrcPubkey(srcPubkey keypair.PublicKey) { act.srcPubkey = srcPubkey }

// DstAddr returns the destination address
func (act *action) DstAddr() string { return act.dstAddr }

// GasLimit returns the gas limit
func (act *action) GasLimit() uint64 { return act.gasLimit }

// GasPrice returns the gas price
func (act *action) GasPrice() *big.Int { return act.gasPrice }

// Signature returns signature bytes
func (act *action) Signature() []byte { return act.signature }

// SetSignature sets the signature bytes
func (act *action) SetSignature(signature []byte) { act.signature = signature }

// Sign signs the action using sender's private key
func Sign(act Action, sender *iotxaddress.Address) error {
	// check the sender is correct
	if act.SrcAddr() != sender.RawAddress {
		return errors.Wrapf(ErrAction, "signer addr %s does not match with action addr %s",
			sender.RawAddress, act.SrcAddr())
	}
	// check the public key is actually owned by sender
	pkhash, err := iotxaddress.GetPubkeyHash(sender.RawAddress)
	if err != nil {
		return errors.Wrap(err, "error when getting the pubkey hash")
	}
	if !bytes.Equal(pkhash, keypair.HashPubKey(sender.PublicKey)) {
		return errors.Wrapf(ErrAction, "signing addr %s does not own correct public key",
			sender.RawAddress)
	}
	act.SetSrcPubkey(sender.PublicKey)
	hash := act.Hash()
	if act.SetSignature(crypto.EC283.Sign(sender.PrivateKey, hash[:])); act.Signature() == nil {
		return errors.Wrapf(ErrAction, "Failed to sign action hash = %x", hash)
	}
	return nil
}

// Verify verifies the action using sender's public key
func Verify(act Action, sender *iotxaddress.Address) error {
	hash := act.Hash()
	if success := crypto.EC283.Verify(sender.PublicKey, hash[:], act.Signature()); success {
		return nil
	}
	return errors.Wrapf(ErrAction, "Failed to verify action signature = %x", act.Signature())
}
