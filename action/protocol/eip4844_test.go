package protocol

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestCalcExcessBlobGas(t *testing.T) {
	require := require.New(t)
	var tests = []struct {
		excess uint64
		blobs  uint64
		want   uint64
	}{
		// The excess blob gas should not increase from zero if the used blob
		// slots are below - or equal - to the target.
		{0, 0, 0},
		{0, 1, 0},
		{0, params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob, 0},

		// If the target blob gas is exceeded, the excessBlobGas should increase
		// by however much it was overshot
		{0, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 1, params.BlobTxBlobGasPerBlob},
		{1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 1, params.BlobTxBlobGasPerBlob + 1},
		{1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) + 2, 2*params.BlobTxBlobGasPerBlob + 1},

		// The excess blob gas should decrease by however much the target was
		// under-shot, capped at zero.
		{params.BlobTxTargetBlobGasPerBlock, params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob, params.BlobTxTargetBlobGasPerBlock},
		{params.BlobTxTargetBlobGasPerBlock, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 1, params.BlobTxTargetBlobGasPerBlock - params.BlobTxBlobGasPerBlob},
		{params.BlobTxTargetBlobGasPerBlock, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 2, params.BlobTxTargetBlobGasPerBlock - (2 * params.BlobTxBlobGasPerBlob)},
		{params.BlobTxBlobGasPerBlob - 1, (params.BlobTxTargetBlobGasPerBlock / params.BlobTxBlobGasPerBlob) - 1, 0},
	}
	for i, tt := range tests {
		result := CalcExcessBlobGas(tt.excess, tt.blobs*params.BlobTxBlobGasPerBlob)
		require.Equal(tt.want, result, "test %d: excess blob gas mismatch", i)
	}
}

func TestCalcBlobFee(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		excessBlobGas uint64
		blobfee       int64
	}{
		{0, 1},
		{2314057, 1},
		{2314058, 2},
		{10 * 1024 * 1024, 23},
	}
	for i, tt := range tests {
		have := CalcBlobFee(tt.excessBlobGas)
		require.Equal(tt.blobfee, have.Int64(), "test %d: blobfee mismatch", i)
	}
}

func TestFakeExponential(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		factor      int64
		numerator   int64
		denominator int64
		want        int64
	}{
		// When numerator == 0 the return value should always equal the value of factor
		{1, 0, 1, 1},
		{38493, 0, 1000, 38493},
		{0, 1234, 2345, 0}, // should be 0
		{1, 2, 1, 6},       // approximate 7.389
		{1, 4, 2, 6},
		{1, 3, 1, 16}, // approximate 20.09
		{1, 6, 2, 18},
		{1, 4, 1, 49}, // approximate 54.60
		{1, 8, 2, 50},
		{10, 8, 2, 542}, // approximate 540.598
		{11, 8, 2, 596}, // approximate 600.58
		{1, 5, 1, 136},  // approximate 148.4
		{1, 5, 2, 11},   // approximate 12.18
		{2, 5, 2, 23},   // approximate 24.36
		{1, 50000000, 2225652, 5709098764},
	}
	for i, tt := range tests {
		f, n, d := big.NewInt(tt.factor), big.NewInt(tt.numerator), big.NewInt(tt.denominator)
		original := fmt.Sprintf("%d %d %d", f, n, d)
		have := fakeExponential(f, n, d)
		require.Equal(tt.want, have.Int64(), "test %d: fake exponential mismatch", i)
		later := fmt.Sprintf("%d %d %d", f, n, d)
		require.Equal(original, later, "test %d: fake exponential modified arguments", i)
	}
}

func TestEIP4844Params(t *testing.T) {
	require := require.New(t)
	require.Equal(1<<17, params.BlobTxBlobGasPerBlob)
	require.Equal(3*params.BlobTxBlobGasPerBlob, params.BlobTxTargetBlobGasPerBlock)
	require.Equal(6*params.BlobTxBlobGasPerBlob, params.MaxBlobGasPerBlock)
	require.Equal(1, params.BlobTxMinBlobGasprice)
	require.Equal(3338477, params.BlobTxBlobGaspriceUpdateFraction)
	require.Equal(4096, params.BlobTxFieldElementsPerBlob)
	require.Equal(32, params.BlobTxBytesPerFieldElement)
}
