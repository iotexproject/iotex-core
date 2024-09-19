// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var (
	_ TxContainer = (*txContainer)(nil)
)

type txContainer struct {
	raw []byte
	tx  *types.Transaction
}

func (etx *txContainer) hash() hash.Hash256 {
	h := etx.tx.Hash()
	return hash.BytesToHash256(h[:])
}

func (etx *txContainer) typeToEncoding() (iotextypes.Encoding, error) {
	switch etx.tx.Type() {
	case types.LegacyTxType:
		if !etx.tx.Protected() {
			// tx has pre-EIP155 signature
			return iotextypes.Encoding_ETHEREUM_UNPROTECTED, nil
		}
	case types.AccessListTxType, types.DynamicFeeTxType, types.BlobTxType:
	default:
		return 0, ErrNotSupported
	}
	return iotextypes.Encoding_ETHEREUM_EIP155, nil
}

func (etx *txContainer) proto() *iotextypes.TxContainer {
	if len(etx.raw) == 0 {
		var err error
		etx.raw, err = etx.tx.MarshalBinary()
		if err != nil {
			panic(err.Error())
		}
	}
	return &iotextypes.TxContainer{
		Raw: etx.raw,
	}
}

func (etx *txContainer) loadProto(pbAct *iotextypes.TxContainer) error {
	if pbAct == nil {
		return ErrNilProto
	}
	if etx == nil {
		return ErrNilAction
	}
	var (
		raw = pbAct.GetRaw()
		tx  = types.Transaction{}
	)
	if err := tx.UnmarshalBinary(raw); err != nil {
		return err
	}
	etx.raw = make([]byte, len(raw))
	copy(etx.raw, raw)
	etx.tx = &tx
	return nil
}

func (etx *txContainer) Unfold(selp *SealedEnvelope, ctx context.Context, checker func(context.Context, *common.Address) (bool, bool, bool, error)) error {
	var (
		elp        Envelope
		elpBuilder = (&EnvelopeBuilder{}).SetChainID(selp.ChainID())
	)
	isContract, isStaking, isRewarding, err := checker(ctx, etx.tx.To())
	if err != nil {
		return err
	}
	if isContract {
		elp, err = elpBuilder.BuildExecution(etx.tx)
	} else if isStaking {
		elp, err = elpBuilder.BuildStakingAction(etx.tx)
	} else if isRewarding {
		elp, err = elpBuilder.BuildRewardingAction(etx.tx)
	} else {
		elp, err = elpBuilder.BuildTransfer(etx.tx)
	}
	if err != nil {
		return err
	}
	encoding, err := etx.typeToEncoding()
	if err != nil {
		return err
	}
	selp.Envelope = elp
	selp.encoding = encoding
	selp.hash = hash.ZeroHash256
	selp.srcAddress = nil
	return nil
}

func (etx *txContainer) Cost() (*big.Int, error) {
	maxExecFee := big.NewInt(0).Mul(etx.tx.GasPrice(), big.NewInt(0).SetUint64(etx.tx.Gas()))
	return maxExecFee.Add(etx.tx.Value(), maxExecFee), nil
}

func (etx *txContainer) IntrinsicGas() (uint64, error) {
	gas, err := CalculateIntrinsicGas(ExecutionBaseIntrinsicGas, ExecutionDataGas, uint64(len(etx.tx.Data())))
	if err != nil {
		return gas, err
	}
	if acl := etx.tx.AccessList(); len(acl) > 0 {
		gas += uint64(len(acl)) * TxAccessListAddressGas
		gas += uint64(acl.StorageKeys()) * TxAccessListStorageKeyGas
	}
	return gas, nil
}

func (etx *txContainer) SanityCheck() error {
	// Reject execution of negative amount
	if etx.tx.Value().Sign() < 0 {
		return errors.Wrap(ErrNegativeValue, "negative value")
	}
	if price := etx.tx.GasPrice(); price != nil && price.Sign() < 0 {
		return errors.Wrap(ErrNegativeValue, "negative gas price")
	}
	if tipCap := etx.tx.GasTipCap(); tipCap != nil && tipCap.Sign() < 0 {
		return errors.Wrap(ErrNegativeValue, "negative gas tip cap")
	}
	if feeCap := etx.tx.GasFeeCap(); feeCap != nil && feeCap.Sign() < 0 {
		return errors.Wrap(ErrNegativeValue, "negative gas fee cap")
	}
	return nil
}
