package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBlockWitnessSummaryMismatch(t *testing.T) {
	require := require.New(t)

	err := validateBlockWitness(&blockResult{
		Hash:         "0xabc",
		Transactions: []string{"0x1"},
	}, &blockWitnessResult{
		Summary: &witnessSummary{Contracts: 1},
		Transactions: []txWitnessEnvelope{
			{TxHash: "0x1"},
		},
	})
	require.Error(err)
	require.Contains(err.Error(), "summary mismatch")
}

func TestDecodeHexUint64(t *testing.T) {
	require := require.New(t)

	v, err := decodeHexUint64("0x2a")
	require.NoError(err)
	require.Equal(uint64(42), v)
}
