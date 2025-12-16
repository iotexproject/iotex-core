package action

import "github.com/ethereum/go-ethereum/params"

const (
	MaxBlobGasPerBlock               = 6 * params.BlobTxBlobGasPerBlob
	BlobTxTargetBlobGasPerBlock      = 3 * params.BlobTxBlobGasPerBlob
	BlobTxBlobGaspriceUpdateFraction = 3338477
)
