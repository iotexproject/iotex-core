package chainservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
)

func TestStatelessWitnessRPCClientBlockWitnessByHash(t *testing.T) {
	require := require.New(t)

	blockHash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	txHash := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	contractAddr := common.HexToAddress("0x3333333333333333333333333333333333333333")
	slotKey := common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444")
	slotValue := common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555")
	storageRoot := common.HexToHash("0x6666666666666666666666666666666666666666666666666666666666666666")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactions":[{"txHash":"` + txHash.Hex() + `","witnesses":[{"address":"` + contractAddr.Hex() + `","storageRoot":"` + storageRoot.Hex() + `","entries":[{"key":"` + slotKey.Hex() + `","value":"` + slotValue.Hex() + `"}],"proofNodes":["0x1234"]}]}]}}`))
	}))
	defer srv.Close()

	client := newStatelessWitnessRPCClient(srv.URL)
	svCtx, err := client.blockWitnessByHash(context.Background(), hash.BytesToHash256(blockHash[:]))
	require.NoError(err)
	require.True(svCtx.Enabled)

	actionWitnesses := svCtx.ContractStorageWitnessesForAction(hash.BytesToHash256(txHash[:]))
	require.Len(actionWitnesses, 1)
	contractWitness, ok := actionWitnesses[contractAddr]
	require.True(ok)
	require.Equal(storageRoot[:], contractWitness.StorageRoot[:])
	require.Len(contractWitness.Entries, 1)
	require.Equal(slotKey[:], contractWitness.Entries[0].Key[:])
	require.Equal(slotValue[:], contractWitness.Entries[0].Value)
	require.Len(contractWitness.ProofNodes, 1)
	require.Equal([]byte{0x12, 0x34}, contractWitness.ProofNodes[0])
}

func TestStatelessWitnessRPCClientBlockWitnessByHashNotFoundReturnsEmptyContext(t *testing.T) {
	require := require.New(t)

	blockHash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"not exist in DB"}}`))
	}))
	defer srv.Close()

	client := newStatelessWitnessRPCClient(srv.URL)
	svCtx, err := client.blockWitnessByHash(context.Background(), hash.BytesToHash256(blockHash[:]))
	require.NoError(err)
	require.True(svCtx.Enabled)
	require.Empty(svCtx.ActionWitnesses)
}

func TestParseStatelessValidationContext(t *testing.T) {
	require := require.New(t)
	raw := []byte(`{"transactions":[{"txHash":"0x2222222222222222222222222222222222222222222222222222222222222222","witnesses":[{"address":"0x3333333333333333333333333333333333333333","storageRoot":"0x6666666666666666666666666666666666666666666666666666666666666666","entries":[{"key":"0x4444444444444444444444444444444444444444444444444444444444444444","value":"0x5555555555555555555555555555555555555555555555555555555555555555"}],"proofNodes":["0x1234"]}]}]}`)
	svCtx, err := witness.ParseValidationContext(raw)
	require.NoError(err)
	require.True(svCtx.Enabled)
	require.Len(svCtx.ActionWitnesses, 1)
}
