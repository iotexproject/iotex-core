package chainservice

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
	"github.com/iotexproject/iotex-core/v2/blocksync"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/routine"
)

type blockCommitter func(*block.Block, evm.StatelessValidationContext) error

type csSyncer struct {
	lifecycle.Readiness

	blocksyncCfg blocksync.Config
	tipHeight    blocksync.TipHeight
	commitBlock  blockCommitter
	deserializer *block.Deserializer

	grpcConn   *grpc.ClientConn
	grpcClient iotexapi.APIServiceClient
	witnessRPC *statelessWitnessRPCClient
	witnessDB  *witness.Store
	task       *routine.RecurringTask

	startingHeight    uint64
	targetHeight      uint64
	syncStageHeight   uint64
	syncBlockIncrease uint64
	syncing           atomic.Bool
}

func newCSSyncer(
	cfg blocksync.Config,
	grpcEndpoint string,
	witnessEndpoint string,
	witnessDBPath string,
	tipHeight blocksync.TipHeight,
	commitBlock blockCommitter,
	evmNetworkID uint32,
) (*csSyncer, error) {
	conn, err := grpc.NewClient(grpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CS sync gRPC client")
	}
	return &csSyncer{
		blocksyncCfg: cfg,
		tipHeight:    tipHeight,
		commitBlock:  commitBlock,
		deserializer: block.NewDeserializer(evmNetworkID),
		grpcConn:     conn,
		grpcClient:   iotexapi.NewAPIServiceClient(conn),
		witnessRPC:   newStatelessWitnessRPCClient(witnessEndpoint),
		witnessDB:    witness.NewStore(witnessDBPath),
	}, nil
}

func (s *csSyncer) Start(ctx context.Context) error {
	if err := s.witnessDB.Start(ctx); err != nil {
		return err
	}
	current := s.tipHeight()
	s.startingHeight = current
	s.syncStageHeight = current
	if s.blocksyncCfg.Interval > 0 {
		s.task = routine.NewRecurringTask(s.sync, s.blocksyncCfg.Interval)
		if err := s.task.Start(ctx); err != nil {
			return err
		}
	}
	if err := s.TurnOn(); err != nil {
		return err
	}
	go s.sync()
	return nil
}

func (s *csSyncer) Stop(ctx context.Context) error {
	if err := s.TurnOff(); err != nil {
		return err
	}
	var retErr error
	if s.task != nil {
		retErr = s.task.Stop(ctx)
	}
	if err := s.grpcConn.Close(); err != nil && retErr == nil {
		retErr = err
	}
	if err := s.witnessDB.Stop(ctx); err != nil && retErr == nil {
		retErr = err
	}
	return retErr
}

func (s *csSyncer) TargetHeight() uint64 {
	return atomic.LoadUint64(&s.targetHeight)
}

func (*csSyncer) ProcessSyncRequest(context.Context, peer.AddrInfo, uint64, uint64) error {
	return nil
}

func (*csSyncer) ProcessBlock(context.Context, string, *block.Block) error {
	return nil
}

func (s *csSyncer) SyncStatus() (uint64, uint64, uint64, string) {
	syncBlockIncrease := atomic.LoadUint64(&s.syncBlockIncrease)
	syncSpeedDesc := "synced to blockchain tip"
	switch {
	case s.blocksyncCfg.Interval == 0:
		syncSpeedDesc = "manual sync mode"
	case syncBlockIncrease != 1:
		syncSpeedDesc = fmt.Sprintf("sync in progress at %.1f blocks/sec", float64(syncBlockIncrease)/s.blocksyncCfg.Interval.Seconds())
	}
	return s.startingHeight, s.tipHeight(), s.TargetHeight(), syncSpeedDesc
}

func (s *csSyncer) BuildReport() string {
	startingHeight, tipHeight, targetHeight, syncSpeedDesc := s.SyncStatus()
	return fmt.Sprintf(
		"CSSync startingHeight: %d, tipHeight: %d, targetHeight: %d, %s",
		startingHeight,
		tipHeight,
		targetHeight,
		syncSpeedDesc,
	)
}

func (s *csSyncer) sync() {
	if !s.syncing.CompareAndSwap(false, true) {
		return
	}
	defer s.syncing.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(s.blocksyncCfg.Interval, 30*time.Second))
	defer cancel()

	resp, err := s.grpcClient.GetChainMeta(ctx, &iotexapi.GetChainMetaRequest{})
	if err != nil {
		log.L().Error("CS sync failed to fetch remote chain meta.", zap.Error(err))
		return
	}
	remoteTip := resp.GetChainMeta().GetHeight()
	atomic.StoreUint64(&s.targetHeight, remoteTip)

	localTip := s.tipHeight()
	for height := localTip + 1; height <= remoteTip; height++ {
		blk, err := s.fetchBlock(ctx, height)
		if err != nil {
			log.L().Error("CS sync failed to fetch block.", zap.Uint64("height", height), zap.Error(err))
			return
		}
		svCtx, err := loadOrFetchWitness(ctx, s.witnessDB, blk.HashBlock(), blk.Height(), s.witnessRPC)
		if err != nil {
			log.L().Error("CS sync failed to load witness.", zap.Uint64("height", blk.Height()), zap.Error(err))
			return
		}
		if err := s.commitBlock(blk, svCtx); err != nil {
			log.L().Error("CS sync failed to commit block.", zap.Uint64("height", blk.Height()), zap.Error(err))
			return
		}
		localTip = blk.Height()
	}
	atomic.StoreUint64(&s.syncBlockIncrease, localTip-s.syncStageHeight)
	s.syncStageHeight = localTip
}

func (s *csSyncer) fetchBlock(ctx context.Context, height uint64) (*block.Block, error) {
	resp, err := s.grpcClient.GetRawBlocks(ctx, &iotexapi.GetRawBlocksRequest{
		StartHeight: height,
		Count:       1,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.GetBlocks()) != 1 {
		return nil, errors.Errorf("unexpected raw block count for height %d: %d", height, len(resp.GetBlocks()))
	}
	return s.deserializer.FromBlockProto(resp.GetBlocks()[0].GetBlock())
}

func loadOrFetchWitness(ctx context.Context, store *witness.Store, blockHash hash.Hash256, height uint64, rpc *statelessWitnessRPCClient) (evm.StatelessValidationContext, error) {
	raw, err := store.GetRawByHash(blockHash)
	if err == nil {
		return witness.ParseValidationContext(raw)
	}
	if errors.Cause(err) != db.ErrNotExist {
		return evm.StatelessValidationContext{}, err
	}
	raw, err = rpc.blockWitnessByHashRaw(ctx, blockHash)
	if err != nil {
		return evm.StatelessValidationContext{}, err
	}
	if err := store.PutRaw(blockHash, height, raw); err != nil {
		return evm.StatelessValidationContext{}, err
	}
	return witness.ParseValidationContext(raw)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

var _ blocksync.BlockSync = (*csSyncer)(nil)
