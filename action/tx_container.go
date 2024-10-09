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
	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var (
	_ TxContainer = (*txContainer)(nil)
	_ Envelope    = (*txContainer)(nil)
)

type txContainer struct {
	chainID uint32
	raw     []byte
	tx      *types.Transaction
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

func (etx *txContainer) Version() uint32 {
	txType, _ := convertEthTxType(etx.tx.Type())
	return uint32(txType)
}

func (etx *txContainer) ChainID() uint32 {
	return etx.chainID
}

func (etx *txContainer) Nonce() uint64 {
	return etx.tx.Nonce()
}

func (etx *txContainer) Gas() uint64 {
	return etx.tx.Gas()
}

func (etx *txContainer) GasPrice() *big.Int {
	return etx.tx.GasPrice()
}

func (etx *txContainer) EffectiveGasPrice(baseFee *big.Int) *big.Int {
	// TODO
	return nil
}

func (etx *txContainer) AccessList() types.AccessList {
	return etx.tx.AccessList()
}

func (etx *txContainer) GasTipCap() *big.Int {
	return etx.tx.GasTipCap()
}

func (etx *txContainer) GasFeeCap() *big.Int {
	return etx.tx.GasFeeCap()
}

func (etx *txContainer) BlobGas() uint64 {
	return etx.tx.BlobGas()
}

func (etx *txContainer) BlobGasFeeCap() *big.Int {
	return etx.tx.BlobGasFeeCap()
}

func (etx *txContainer) BlobHashes() []common.Hash {
	return etx.tx.BlobHashes()
}

func (etx *txContainer) BlobTxSidecar() *types.BlobTxSidecar {
	return etx.tx.BlobTxSidecar()
}

func (etx *txContainer) Value() *big.Int {
	return etx.tx.Value()
}

func (etx *txContainer) To() *common.Address {
	return etx.tx.To()
}

func (etx *txContainer) Data() []byte {
	return etx.tx.Data()
}

func (etx *txContainer) Destination() (string, bool) {
	// TODO
	return "", true
}

func (etx *txContainer) Size() uint32 {
	// TODO
	return 0
}

func (etx *txContainer) Action() Action { return etx }

func (etx *txContainer) ToEthTx(evmNetworkID uint32, encoding iotextypes.Encoding) (*types.Transaction, error) {
	return etx.tx, nil
}

func (etx *txContainer) ProtoForHash() *iotextypes.ActionCore {
	// TODO
	return nil
}

func (etx *txContainer) Proto() *iotextypes.ActionCore {
	// TODO
	return nil
}

func (etx *txContainer) LoadProto(pbAct *iotextypes.ActionCore) error {
	if pbAct == nil || pbAct.GetTxContainer() == nil {
		return ErrNilProto
	}
	if etx == nil {
		return ErrNilAction
	}
	var (
		raw = pbAct.GetTxContainer().GetRaw()
		tx  = types.Transaction{}
	)
	if len(raw) == 0 {
		return ErrNilProto
	}
	if err := tx.UnmarshalBinary(raw); err != nil {
		return err
	}
	etx.chainID = pbAct.GetChainID()
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

func (etx *txContainer) SetNonce(n uint64) {}

func (etx *txContainer) SetGas(gas uint64) {}

func (etx *txContainer) SetChainID(chainID uint32) {}

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

func (etx *txContainer) ValidateSidecar() error {
	if etx.tx.Type() != types.BlobTxType {
		return nil
	}
	var (
		size    = len(etx.tx.BlobHashes())
		sidecar = etx.tx.BlobTxSidecar()
	)
	if sidecar == nil || size == 0 {
		return errors.New("blobless blob transaction")
	}
	if permitted := params.MaxBlobGasPerBlock / params.BlobTxBlobGasPerBlob; size > permitted {
		return errors.Errorf("too many blobs in transaction: have %d, permitted %d", size, params.MaxBlobGasPerBlock/params.BlobTxBlobGasPerBlob)
	}
	return verifySidecar(sidecar, etx.tx.BlobHashes())
}
