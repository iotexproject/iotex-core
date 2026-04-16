package witness

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

func TestStorePersistsProtoAndReturnsJSON(t *testing.T) {
	require := require.New(t)

	path := filepath.Join(t.TempDir(), "witness.db")
	store := NewStore(path)
	require.NoError(store.Start(context.Background()))
	defer func() {
		require.NoError(store.Stop(context.Background()))
	}()

	blockHash := hash.Hash256b([]byte("proto-persisted-witness"))
	result := &BlockResult{
		Summary: &Summary{
			Contracts:          1,
			Entries:            2,
			ProofNodes:         3,
			ProofBytes:         4,
			WitnessGenDuration: "12us",
		},
		Transactions: []TransactionResult{{
			TxHash:     "0xaabb",
			Contracts:  1,
			Entries:    2,
			ProofNodes: 3,
			ProofBytes: 4,
			Witnesses: []ContractWitness{{
				Address:     "0x1234",
				StorageRoot: "0x0102",
				Entries: []ContractWitnessKV{{
					Key:   "0x03",
					Value: "0x0405",
				}},
				ProofNodes: []string{"0x060708"},
			}},
		}},
		DebugWriteEntries: []string{"[0] PUT ns=evm key=abcd"},
	}

	require.NoError(store.PutBlock(blockHash, 7, result))

	raw, err := store.db.Get(byHashNamespace, blockHash[:])
	require.NoError(err)
	require.True(isProtoWitness(raw))

	jsonRaw, err := store.GetRawByHash(blockHash)
	require.NoError(err)

	var decoded BlockResult
	require.NoError(json.Unmarshal(jsonRaw, &decoded))
	require.Equal(result, &decoded)
}

func TestStoreReadsLegacyJSON(t *testing.T) {
	require := require.New(t)

	path := filepath.Join(t.TempDir(), "witness.db")
	store := NewStore(path)
	require.NoError(store.Start(context.Background()))
	defer func() {
		require.NoError(store.Stop(context.Background()))
	}()

	blockHash := hash.Hash256b([]byte("legacy-json-witness"))
	raw := json.RawMessage(`{"transactions":[{"txHash":"0x01","witnesses":[{"address":"0x0000000000000000000000000000000000000001","storageRoot":"0x0000000000000000000000000000000000000000000000000000000000000002","entries":[{"key":"0x0000000000000000000000000000000000000000000000000000000000000003","value":"0x04"}],"proofNodes":["0x05"]}]}]}`)
	require.NoError(store.db.Put(byHashNamespace, blockHash[:], raw))
	require.NoError(store.db.Put(hashByHeightNamespace, byteutil.Uint64ToBytesBigEndian(9), blockHash[:]))

	got, err := store.GetRawByHash(blockHash)
	require.NoError(err)
	require.JSONEq(string(raw), string(got))

	blockHashByHeight, gotByHeight, err := store.GetRawByHeight(9)
	require.NoError(err)
	require.Equal(blockHash, blockHashByHeight)
	require.JSONEq(string(raw), string(gotByHeight))

	ctx, err := ParseValidationContext(raw)
	require.NoError(err)
	require.True(ctx.Enabled)
}

func TestParseValidationContextAcceptsProtoPayload(t *testing.T) {
	require := require.New(t)

	raw, err := marshalStoredBlockResult(&BlockResult{
		Transactions: []TransactionResult{{
			TxHash: "0x01",
			Witnesses: []ContractWitness{{
				Address:     "0x0000000000000000000000000000000000000001",
				StorageRoot: "0x0000000000000000000000000000000000000000000000000000000000000002",
				Entries: []ContractWitnessKV{{
					Key:   "0x0000000000000000000000000000000000000000000000000000000000000003",
					Value: "0x04",
				}},
				ProofNodes: []string{"0x05"},
			}},
		}},
	})
	require.NoError(err)

	ctx, err := ParseValidationContext(raw)
	require.NoError(err)
	require.True(ctx.Enabled)
}
