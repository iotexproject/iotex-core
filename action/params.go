package action

import "github.com/ethereum/go-ethereum/params"

const (
	MaxBlobGasPerBlock               = 6 * params.BlobTxBlobGasPerBlob
	BlobTxTargetBlobGasPerBlock      = 3 * params.BlobTxBlobGasPerBlob
	BlobTxBlobGaspriceUpdateFraction = 3338477

	TxTokenPerNonZeroByte uint64 = 1     // Token cost per non-zero byte as specified by EIP-7623.
	TxGas                 uint64 = 10000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxCostFloorPerToken   uint64 = 250   // Cost floor per byte of data as specified by EIP-7623.

	// MaxTxGas is the largest gas limit a single action may declare once
	// EIP-7825 is active (2^24 = 16,777,216).
	MaxTxGas = params.MaxTxGas
)
