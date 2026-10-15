package api

import (
	"context"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

// End to end: a real gRPC client that subscribes to StreamBlocks and never
// calls Recv must not stall block production. When the stream write is coupled
// to the fan-out goroutine, this stalls CommitBlock while it holds the chain
// write lock, which also blocks MintNewBlock and ValidateBlock. Decoupling the
// write keeps the node committing and drops the idle subscription.
func TestStreamConsumerDoesNotStallCommitBlock(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	const bufSize = 2

	goroutinesBefore := runtime.NumGoroutine()

	// --- a real chain, with a deliberately tiny streaming buffer ---
	chainCfg := blockchain.DefaultConfig
	chainCfg.StreamingBlockBufferSize = bufSize
	g := genesis.TestDefault()
	ctx := genesis.WithGenesisContext(context.Background(), g)

	registry := protocol.NewRegistry()
	r.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	r.NoError(rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs).Register(registry))

	sf, err := factory.NewStateDB(factory.GenerateConfig(chainCfg, g), db.NewMemKVStore(),
		factory.RegistryStateDBOption(registry))
	r.NoError(err)
	ap, err := actpool.NewActPool(g, sf, actpool.DefaultConfig)
	r.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	r.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, db.DefaultConfig.MaxCacheSize)
	bc := blockchain.NewBlockchain(chainCfg, g, dao, factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState))))
	r.NoError(bc.Start(ctx))
	defer func() { _ = bc.Stop(ctx) }()

	// --- a real gRPC server in front of it ---
	cl := NewChainListener(10)
	r.NoError(bc.AddSubscriber(cl))
	core := NewMockCoreService(ctrl)
	core.EXPECT().ChainListener().Return(cl).AnyTimes()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	r.NoError(err)
	srv := grpc.NewServer()
	iotexapi.RegisterAPIServiceServer(srv, &gRPCHandler{coreService: core})
	go srv.Serve(lis)
	defer srv.Stop()

	// --- a client that subscribes and then never reads ---
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithInitialWindowSize(65536),
		grpc.WithInitialConnWindowSize(65536))
	r.NoError(err)
	defer conn.Close()
	streamCtx, cancelStream := context.WithCancel(context.Background())
	defer cancelStream()
	_, err = iotexapi.NewAPIServiceClient(conn).StreamBlocks(streamCtx, &iotexapi.StreamBlocksRequest{})
	r.NoError(err)
	r.Eventually(func() bool { return cl.(*chainListener).streamMap.Count() == 1 },
		10*time.Second, 20*time.Millisecond, "subscription never registered")

	// --- the node must keep committing ---
	// must exceed the client's flow-control window (~335 empty blocks at a
	// 64 KiB stream window) plus streamSendQueueSize, so the subscription is
	// actually driven into overflow rather than absorbed by the window
	const blocks = 600
	var committed int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		ts := time.Now()
		for i := 0; i < blocks; i++ {
			ts = ts.Add(time.Second)
			blk, err := bc.MintNewBlock(ts)
			if err != nil {
				t.Errorf("mint failed at block %d: %v", i, err)
				return
			}
			if err := bc.CommitBlock(blk); err != nil {
				t.Errorf("commit failed at block %d: %v", i, err)
				return
			}
			atomic.AddInt64(&committed, 1)
		}
	}()
	select {
	case <-done:
	case <-time.After(120 * time.Second):
		t.Fatalf("CommitBlock stalled behind a non-reading stream consumer (committed=%d/%d)",
			atomic.LoadInt64(&committed), blocks)
	}
	r.EqualValues(blocks, atomic.LoadInt64(&committed))

	// the chain lock is never held for longer than a fan-out enqueue
	_, err = bc.MintNewBlock(time.Now().Add(time.Hour))
	r.NoError(err)

	// the non-reading subscription was dropped rather than tolerated
	r.Eventually(func() bool { return cl.(*chainListener).streamMap.Count() == 0 },
		30*time.Second, 50*time.Millisecond, "non-reading subscription was never dropped")

	// and its writer goroutine does not leak once the RPC is torn down
	cancelStream()
	srv.Stop()
	conn.Close()
	r.Eventually(func() bool { return runtime.NumGoroutine() < goroutinesBefore+20 },
		30*time.Second, 100*time.Millisecond,
		"goroutines leaked: before=%d now=%d", goroutinesBefore, runtime.NumGoroutine())
}
