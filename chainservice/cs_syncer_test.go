package chainservice

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
	"github.com/iotexproject/iotex-core/v2/blocksync"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

func TestWitnessStoreLoadOrFetch(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	store := witness.NewStore(filepath.Join(t.TempDir(), "witness.db"))
	require.NoError(store.Start(ctx))
	defer func() {
		require.NoError(store.Stop(ctx))
	}()

	txHash := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	contractAddr := common.HexToAddress("0x3333333333333333333333333333333333333333")
	slotKey := common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444")
	slotValue := common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555")
	storageRoot := common.HexToHash("0x6666666666666666666666666666666666666666666666666666666666666666")
	blockHash, err := hash.HexStringToHash256("1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(err)

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactions":[{"txHash":"` + txHash.Hex() + `","witnesses":[{"address":"` + contractAddr.Hex() + `","storageRoot":"` + storageRoot.Hex() + `","entries":[{"key":"` + slotKey.Hex() + `","value":"` + slotValue.Hex() + `"}],"proofNodes":["0x1234"]}]}]}}`))
	}))
	defer srv.Close()

	rpc := newStatelessWitnessRPCClient(srv.URL)
	svCtx, err := loadOrFetchWitness(ctx, store, blockHash, 123, rpc)
	require.NoError(err)
	require.True(svCtx.Enabled)
	require.Equal(1, requests)

	blockHash2, raw, err := store.GetRawByHeight(123)
	require.NoError(err)
	require.Equal(blockHash, blockHash2)
	cachedCtx, err := witness.ParseValidationContext(raw)
	require.NoError(err)
	require.Equal(svCtx, cachedCtx)

	_, err = store.GetRawByHash(blockHash)
	require.NoError(err)
	_, _, err = store.GetRawByHeight(123)
	require.NoError(err)

	svCtx, err = loadOrFetchWitness(ctx, store, blockHash, 123, rpc)
	require.NoError(err)
	require.True(svCtx.Enabled)
	require.Equal(1, requests)

	_, _, err = store.GetRawByHeight(999)
	require.Error(err)
	require.Equal(db.ErrNotExist, errors.Cause(err))
}

type hangingAPIService struct {
	iotexapi.UnimplementedAPIServiceServer
}

func (*hangingAPIService) GetChainMeta(ctx context.Context, req *iotexapi.GetChainMetaRequest) (*iotexapi.GetChainMetaResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCSSyncerStartDoesNotBlockOnInitialSync(t *testing.T) {
	require := require.New(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(err)

	grpcServer := grpc.NewServer()
	iotexapi.RegisterAPIServiceServer(grpcServer, &hangingAPIService{})
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	witnessSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unexpected witness request", http.StatusInternalServerError)
	}))
	defer witnessSrv.Close()

	syncer, err := newCSSyncer(
		blocksync.Config{Interval: time.Second},
		lis.Addr().String(),
		witnessSrv.URL,
		filepath.Join(t.TempDir(), "witness.db"),
		func() uint64 { return 0 },
		func(*block.Block, evm.StatelessValidationContext) error { return nil },
		4692,
	)
	require.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- syncer.Start(ctx)
	}()

	select {
	case err := <-done:
		require.NoError(err)
	case <-time.After(2 * time.Second):
		t.Fatal("cs syncer Start blocked on initial sync")
	}

	require.NoError(syncer.Stop(context.Background()))
}
