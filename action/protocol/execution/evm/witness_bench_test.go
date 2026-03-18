package evm

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/state"
)

// randomHash returns a random 32-byte hash.
func randomHash() hash.Hash256 {
	var h hash.Hash256
	_, _ = rand.Read(h[:])
	return h
}

// randomValue returns a random value of the given length.
func randomValue(n int) []byte {
	v := make([]byte, n)
	_, _ = rand.Read(v)
	return v
}

// buildContractWithEntries creates a contract, inserts `nEntries` storage slots,
// commits, re-reads them all (to populate committed map), then returns the contract
// ready for BuildStorageWitness.
func buildContractWithEntries(t testing.TB, nEntries int) (Contract, []common.Hash) {
	ctrl := gomock.NewController(t)
	sm, err := initMockStateManager(ctrl)
	require.NoError(t, err)

	addr := hash.BytesToHash160(_c1[:])
	cntr, err := newContract(addr, &state.Account{}, sm, false)
	require.NoError(t, err)

	keys := make([]hash.Hash256, nEntries)
	for i := 0; i < nEntries; i++ {
		keys[i] = randomHash()
		require.NoError(t, cntr.SetState(keys[i], randomValue(32)))
	}
	require.NoError(t, cntr.Commit())

	// Re-read all keys so they are in the committed map (required by BuildStorageWitness).
	for _, k := range keys {
		_, err := cntr.GetState(k)
		require.NoError(t, err)
	}

	// Convert to []common.Hash for ContractStorageAccess.
	reads := make([]common.Hash, nEntries)
	for i, k := range keys {
		reads[i] = common.BytesToHash(k[:])
	}
	return cntr, reads
}

// TestWitnessSizeByEntries measures witness binary size (proof bytes) for varying
// numbers of storage entries and prints a summary table.
func TestWitnessSizeByEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping size measurement in short mode")
	}
	sizes := []int{1, 5, 10, 20, 50, 100, 200, 500}

	fmt.Printf("\n%-10s %12s %12s %14s %14s\n",
		"entries", "proof_nodes", "proof_bytes", "proof_KB", "bytes/entry")
	fmt.Printf("%s\n", fmt.Sprintf("%s", "------------------------------------------------------------"))

	for _, n := range sizes {
		cntr, reads := buildContractWithEntries(t, n)
		witness, err := cntr.BuildStorageWitness(ContractStorageAccess{Reads: reads})
		require.NoError(t, err)

		totalBytes := 0
		for _, node := range witness.ProofNodes {
			totalBytes += len(node)
		}
		perEntry := 0
		if n > 0 {
			perEntry = totalBytes / n
		}
		fmt.Printf("%-10d %12d %12d %14.2f %14d\n",
			n, len(witness.ProofNodes), totalBytes,
			float64(totalBytes)/1024.0, perEntry)
	}
}

// BenchmarkWitnessVerify benchmarks VerifyContractStorageWitness for varying entry counts.
func BenchmarkWitnessVerify1(b *testing.B)   { benchmarkVerify(b, 1) }
func BenchmarkWitnessVerify5(b *testing.B)   { benchmarkVerify(b, 5) }
func BenchmarkWitnessVerify10(b *testing.B)  { benchmarkVerify(b, 10) }
func BenchmarkWitnessVerify20(b *testing.B)  { benchmarkVerify(b, 20) }
func BenchmarkWitnessVerify50(b *testing.B)  { benchmarkVerify(b, 50) }
func BenchmarkWitnessVerify100(b *testing.B) { benchmarkVerify(b, 100) }
func BenchmarkWitnessVerify200(b *testing.B) { benchmarkVerify(b, 200) }
func BenchmarkWitnessVerify500(b *testing.B) { benchmarkVerify(b, 500) }

func benchmarkVerify(b *testing.B, nEntries int) {
	cntr, reads := buildContractWithEntries(b, nEntries)
	witness, err := cntr.BuildStorageWitness(ContractStorageAccess{Reads: reads})
	if err != nil {
		b.Fatal(err)
	}
	addr := common.BytesToAddress(_c1[:])
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := VerifyContractStorageWitness(addr, witness); err != nil {
			b.Fatal(err)
		}
	}
}
