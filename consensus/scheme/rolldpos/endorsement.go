// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	endorseProposal = uint8(0)
	endorseLock     = uint8(1)
)

type endorsement struct {
	topic          uint8
	height         uint64
	blkHash        hash.Hash32B
	endorser       string
	endorserPubkey keypair.PublicKey
	signature      []byte
}

// BinaryMarshaler returns a raw byte stream
func (en *endorsement) BinaryMarshaler() []byte {
	stream := make([]byte, 8)
	enc.MachineEndian.PutUint64(stream, en.height)
	switch en.topic {
	case endorseLock:
		stream = append(stream, 1)
	case endorseProposal:
		stream = append(stream, 0)
	}
	stream = append(stream, en.blkHash[:]...)
	return stream
}

// Hash returns the hash of the endorse for signature
func (en *endorsement) Hash() hash.Hash32B {
	return blake2b.Sum256(en.BinaryMarshaler())
}

// Sign signs with endorser's private key
func (en *endorsement) Sign(endorser *iotxaddress.Address) error {
	if endorser.PrivateKey == keypair.ZeroPrivateKey {
		return errors.New("The endorser's private key is empty")
	}
	hash := en.Hash()
	en.endorser = endorser.RawAddress
	en.endorserPubkey = endorser.PublicKey
	en.signature = crypto.EC283.Sign(endorser.PrivateKey, hash[:])
	return nil
}

// VerifySignature verifies that the endorse with pubkey
func (en *endorsement) VerifySignature() bool {
	hash := en.Hash()
	return crypto.EC283.Verify(en.endorserPubkey, hash[:], en.signature)
}

func (en *endorsement) toProtoMsg() *iproto.EndorsePb {
	var topic iproto.EndorsePb_EndorsementTopic
	switch en.topic {
	case endorseProposal:
		topic = iproto.EndorsePb_PROPOSAL
	case endorseLock:
		topic = iproto.EndorsePb_LOCK
	}
	return &iproto.EndorsePb{
		Height:         en.height,
		BlockHash:      en.blkHash[:],
		Topic:          topic,
		Endorser:       en.endorser,
		EndorserPubKey: en.endorserPubkey[:],
		Decision:       true,
		Signature:      en.signature[:],
	}
}

func (en *endorsement) fromProtoMsg(endorsePb *iproto.EndorsePb) error {
	copy(en.blkHash[:], endorsePb.BlockHash)
	switch endorsePb.Topic {
	case iproto.EndorsePb_PROPOSAL:
		en.topic = endorseProposal
	case iproto.EndorsePb_LOCK:
		en.topic = endorseLock
	default:
		return errors.New("Invalid topic")
	}
	pubKey, err := keypair.BytesToPublicKey(endorsePb.EndorserPubKey)
	if err != nil {
		logger.Error().
			Err(err).
			Bytes("endorserPubKey", endorsePb.EndorserPubKey).
			Msg("error when constructing endorse from proto message")
		return err
	}
	en.endorserPubkey = pubKey
	en.height = endorsePb.Height
	en.endorser = endorsePb.Endorser
	copy(en.signature, endorsePb.Signature)
	return nil
}
