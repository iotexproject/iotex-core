package blobpool

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

func TxDifference(a, b []*action.SealedEnvelope) []*action.SealedEnvelope {
	keep := make([]*action.SealedEnvelope, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		h, _ := tx.Hash()
		remove[common.BytesToHash(h[:])] = struct{}{}
	}

	for _, tx := range a {
		h, _ := tx.Hash()
		if _, ok := remove[common.BytesToHash(h[:])]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

func eip1559CalcBaseFee() *big.Int {
	// TODO: implement EIP-1559 base fee calculation
	return big.NewInt(0)
}

func eip4844CalcBlobFee() *big.Int {
	// TODO: implement EIP-4844 blob fee calculation
	return big.NewInt(0)
}

func WithoutBlobTxSidecar(tx *action.SealedEnvelope) *action.SealedEnvelope {
	// TODO: implement WithoutBlobTxSidecar
	panic("not implemented")
}

func BlobTxSidecar(tx *action.SealedEnvelope) *action.SealedEnvelope {
	// TODO: implement BlobTxSidecar
	panic("not implemented")
}

func ExcessBlobGas(tx *block.Header) *uint64 {
	// TODO: implement ExcessBlobGas
	panic("not implemented")
}

func GasFeeCapIntCmp(*action.SealedEnvelope, *big.Int) int {
	// TODO: implement GasFeeCapIntCmp
	panic("not implemented")
}
func GasTipCapIntCmp(*action.SealedEnvelope, *big.Int) int {
	// TODO: implement GasTipCapIntCmp
	panic("not implemented")
}

func BlobGasFeeCapIntCmp(*action.SealedEnvelope, *big.Int) int {
	// TODO: implement BlobGasFeeCapIntCmp
	panic("not implemented")
}

func BlobGasFeeCap(tx *action.SealedEnvelope) *big.Int {
	// TODO: implement BlobGasFeeCap
	panic("not implemented")
}

func BlobGasLimit(tx *action.SealedEnvelope) uint64 {
	// TODO: implement BlobGasLimit
	panic("not implemented")
}

func TxType(tx *action.SealedEnvelope) uint8 {
	// TODO: implement TxType
	panic("not implemented")
}
