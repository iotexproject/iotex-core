// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// SecretProposal defines the struct of DKG secret proposal
type SecretProposal struct {
	AbstractAction
	secret []uint32
}

// NewSecretProposal returns a SecretProposal instance
func NewSecretProposal(
	nonce uint64,
	sender string,
	recipient string,
	secret []uint32,
) (*SecretProposal, error) {
	if len(sender) == 0 || len(recipient) == 0 {
		return nil, errors.Wrap(ErrAddress, "address of sender or recipient is empty")
	}
	return &SecretProposal{
		AbstractAction: AbstractAction{
			version: version.ProtocolVersion,
			nonce:   nonce,
			srcAddr: sender,
			dstAddr: recipient,
		},
		secret: secret,
	}, nil
}

// Secret returns the secret
func (sp *SecretProposal) Secret() []uint32 { return sp.secret }

// ByteStream returns a raw byte stream of this SecretProposal
func (sp *SecretProposal) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(sp.Proto()))
}

// Proto converts SecretProposal to protobuf's ActionPb
func (sp *SecretProposal) Proto() *iproto.SecretProposalPb {
	// used by account-based model
	return &iproto.SecretProposalPb{
		Recipient: sp.dstAddr,
		Secret:    sp.secret,
	}
}

// LoadProto converts a protobuf's ActionPb to SecretProposal
func (sp *SecretProposal) LoadProto(pbSp *iproto.SecretProposalPb) error {
	if pbSp == nil {
		return errors.New("empty action proto to load")
	}
	if sp == nil {
		return errors.New("nil action to load proto")
	}
	*sp = SecretProposal{}

	sp.secret = pbSp.Secret
	return nil
}

// IntrinsicGas returns the intrinsic gas of a secret proposal
func (sp *SecretProposal) IntrinsicGas() (uint64, error) { return 0, nil }
