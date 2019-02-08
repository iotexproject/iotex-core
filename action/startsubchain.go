// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// StartSubChainIntrinsicGas is the instrinsic gas for start sub chain action
	StartSubChainIntrinsicGas = uint64(1000)
)

// StartSubChain represents start sub-chain message
type StartSubChain struct {
	AbstractAction

	chainID            uint32
	securityDeposit    *big.Int
	operationDeposit   *big.Int
	startHeight        uint64
	parentHeightOffset uint64
}

// NewStartSubChain instantiates a start sub-chain action struct
func NewStartSubChain(
	nonce uint64,
	chainID uint32,
	securityDeposit *big.Int,
	operationDeposit *big.Int,
	startHeight uint64,
	parentHeightOffset uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) *StartSubChain {
	return &StartSubChain{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		chainID:            chainID,
		securityDeposit:    securityDeposit,
		operationDeposit:   operationDeposit,
		startHeight:        startHeight,
		parentHeightOffset: parentHeightOffset,
	}
}

// LoadProto converts a proto message into start sub-chain action
func (start *StartSubChain) LoadProto(startPb *iproto.StartSubChainPb) error {
	if startPb == nil {
		return errors.New("empty action proto to load")
	}
	if start == nil {
		return errors.New("nil action to load proto")
	}
	*start = StartSubChain{}

	start.chainID = startPb.ChainID
	start.startHeight = startPb.StartHeight
	start.parentHeightOffset = startPb.ParentHeightOffset
	start.securityDeposit = big.NewInt(0)
	start.securityDeposit.SetBytes(startPb.GetSecurityDeposit())
	start.operationDeposit = big.NewInt(0)
	start.operationDeposit.SetBytes(startPb.GetOperationDeposit())
	return nil
}

// ChainID returns chain ID
func (start *StartSubChain) ChainID() uint32 { return start.chainID }

// SecurityDeposit returns security deposit
func (start *StartSubChain) SecurityDeposit() *big.Int { return start.securityDeposit }

// OperationDeposit returns operation deposit
func (start *StartSubChain) OperationDeposit() *big.Int { return start.operationDeposit }

// StartHeight returns start height
func (start *StartSubChain) StartHeight() uint64 { return start.startHeight }

// ParentHeightOffset returns parent height offset
func (start *StartSubChain) ParentHeightOffset() uint64 { return start.parentHeightOffset }

// OwnerPublicKey returns the owner public key, which is the wrapper of SrcPubkey
func (start *StartSubChain) OwnerPublicKey() keypair.PublicKey { return start.SrcPubkey() }

// ByteStream returns the byte representation of sub-chain action
func (start *StartSubChain) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(start.Proto()))
}

// Proto converts start sub-chain action into a proto message
func (start *StartSubChain) Proto() *iproto.StartSubChainPb {
	// used by account-based model
	act := &iproto.StartSubChainPb{
		ChainID:            start.chainID,
		StartHeight:        start.startHeight,
		ParentHeightOffset: start.parentHeightOffset,
	}

	if start.securityDeposit != nil && len(start.securityDeposit.Bytes()) > 0 {
		act.SecurityDeposit = start.securityDeposit.Bytes()
	}
	if start.operationDeposit != nil && len(start.operationDeposit.Bytes()) > 0 {
		act.OperationDeposit = start.operationDeposit.Bytes()
	}
	return act
}

// IntrinsicGas returns the intrinsic gas of a start sub-chain action
func (start *StartSubChain) IntrinsicGas() (uint64, error) {
	return StartSubChainIntrinsicGas, nil
}

// Cost returns the total cost of a start sub-chain action
func (start *StartSubChain) Cost() (*big.Int, error) {
	intrinsicGas, err := start.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the start-sub chain action")
	}
	fee := big.NewInt(0).Mul(start.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
