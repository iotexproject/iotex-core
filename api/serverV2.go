package api

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/iotexproject/iotex-core/action"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

// CoreService provides api interface for user to interact with blockchain data
type CoreService interface {
	// Account returns the metadata of an account
	Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
	// ChainMeta returns blockchain metadata
	ChainMeta(uint64) (*iotextypes.ChainMeta, string, error)
	// SendAction is the API to send an action to blockchain.
	SendAction(ctx context.Context, in *iotextypes.Action) (string, error)
	// ReceiptByAction gets receipt with corresponding action hash
	ReceiptByAction(actHash hash.Hash256) (*action.Receipt, string, error)
	// ReadContract reads the state in a contract address specified by the slot
	ReadContract(ctx context.Context, callerAddr address.Address, sc *action.Execution) (string, *iotextypes.Receipt, error)
	// ReadState reads state on blockchain
	ReadState(protocolID string, height string, methodName []byte, arguments [][]byte) (*iotexapi.ReadStateResponse, error)
	// SuggestGasPrice suggests gas price
	SuggestGasPrice() (uint64, error)
	// EstimateGasForAction estimates gas for action
	EstimateGasForAction(in *iotextypes.Action) (uint64, error)
	// EpochMeta gets epoch metadata
	EpochMeta(epochNum uint64) (*iotextypes.EpochData, uint64, []*iotexapi.BlockProducerInfo, error)
	// RawBlocks gets raw block data
	RawBlocks(startHeight uint64, count uint64, withReceipts bool, withTransactionLogs bool) ([]*iotexapi.BlockInfo, error)
	// ElectionBuckets returns the native election buckets.
	ElectionBuckets(epochNum uint64) ([]*iotextypes.ElectionBucket, error)
	// ReceiptByActionHash returns receipt by action hash
	ReceiptByActionHash(h hash.Hash256) (*action.Receipt, error)
	// TransactionLogByActionHash returns transaction log by action hash
	TransactionLogByActionHash(actHash string) (*iotextypes.TransactionLog, error)
	// TransactionLogByBlockHeight returns transaction log by block height
	TransactionLogByBlockHeight(blockHeight uint64) (*iotextypes.BlockIdentifier, *iotextypes.TransactionLogs, error)

	// Start starts the API server
	Start(ctx context.Context) error
	// Stop stops the API server
	Stop(ctx context.Context) error
	// Actions returns actions within the range
	Actions(start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// Action returns action by action hash
	Action(actionHash string, checkPending bool) (*iotexapi.ActionInfo, error)
	// ActionsByAddress returns all actions associated with an address
	ActionsByAddress(addr address.Address, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// ActionByActionHash returns action by action hash
	ActionByActionHash(h hash.Hash256) (action.SealedEnvelope, hash.Hash256, uint64, uint32, error)
	// ActionsByBlock returns all actions in a block
	ActionsByBlock(blkHash string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// ActPoolActions returns the all Transaction Identifiers in the mempool
	ActPoolActions(actHashes []string) ([]*iotextypes.Action, error)
	// UnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
	UnconfirmedActionsByAddress(address string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// EstimateGasForNonExecution  estimates action gas except execution
	EstimateGasForNonExecution(action.Action) (uint64, error)
	// EstimateExecutionGasConsumption estimate gas consumption for execution action
	EstimateExecutionGasConsumption(ctx context.Context, sc *action.Execution, callerAddr address.Address) (uint64, error)
	// BlockMetas returns blockmetas response within the height range
	BlockMetas(start uint64, count uint64) ([]*iotextypes.BlockMeta, error)
	// BlockMetaByHash returns blockmeta response by block hash
	BlockMetaByHash(blkHash string) (*iotextypes.BlockMeta, error)
	// LogsInBlock filter logs in the block x
	LogsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error)
	// LogsInBlockByHash filter logs in the block by hash
	LogsInBlockByHash(filter *logfilter.LogFilter, blockHash hash.Hash256) ([]*iotextypes.Log, error)
	// LogsInRange filter logs among [start, end] blocks
	LogsInRange(filter *logfilter.LogFilter, start, end, paginationSize uint64) ([]*iotextypes.Log, error)
	// EVMNetworkID returns the network id of evm
	EVMNetworkID() uint32
	// ChainID returns the chain id of evm
	ChainID() uint32
	// ReadContractStorage reads contract's storage
	ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error)
	// SimulateExecution simulates execution
	SimulateExecution(context.Context, address.Address, *action.Execution) ([]byte, *action.Receipt, error)
	// TipHeight returns the tip of the chain
	TipHeight() uint64
	// PendingNonce returns the pending nonce of an account
	PendingNonce(address.Address) (uint64, error)
	// ReceiveBlock broadcasts the block to api subscribers
	ReceiveBlock(blk *block.Block) error
}

// ServerV2 provides api for user to interact with blockchain data
type ServerV2 struct {
	core       CoreService
	GrpcServer *GRPCServer
	web3Server *Web3Server
	tracer     *tracesdk.TracerProvider
}

// NewServerV2 creates a new server with coreService and GRPC Server
func NewServerV2(
	cfg config.API,
	coreAPI CoreService,
) (*ServerV2, error) {
	if cfg == (config.API{}) {
		log.L().Warn("API server is not configured.")
		cfg = config.Default.API
	}
	tp, err := tracer.NewProvider(
		tracer.WithServiceName(cfg.Tracer.ServiceName),
		tracer.WithEndpoint(cfg.Tracer.EndPoint),
		tracer.WithInstanceID(cfg.Tracer.InstanceID),
		tracer.WithSamplingRatio(cfg.Tracer.SamplingRatio),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot config tracer provider")
	}
	return &ServerV2{
		core:       coreAPI,
		GrpcServer: NewGRPCServer(coreAPI, cfg.Port, cfg.RangeQueryLimit, uint64(cfg.TpsWindow)),
		web3Server: NewWeb3Server(coreAPI, cfg.Web3Port, cfg.RedisCacheURL, cfg.RangeQueryLimit),
		tracer:     tp,
	}, nil
}

// Start starts the CoreService and the GRPC server
func (svr *ServerV2) Start(ctx context.Context) error {
	if err := svr.core.Start(ctx); err != nil {
		return err
	}
	if err := svr.GrpcServer.Start(ctx); err != nil {
		return err
	}
	svr.web3Server.Start(ctx)
	return nil
}

// Stop stops the GRPC server and the CoreService
func (svr *ServerV2) Stop(ctx context.Context) error {
	if svr.tracer != nil {
		if err := svr.tracer.Shutdown(context.Background()); err != nil {
			return errors.Wrap(err, "failed to shutdown api tracer")
		}
	}
	if err := svr.web3Server.Stop(ctx); err != nil {
		return err
	}
	if err := svr.GrpcServer.Stop(ctx); err != nil {
		return err
	}
	if err := svr.core.Stop(ctx); err != nil {
		return err
	}
	return nil
}

// ReceiveBlock receives the new block
func (svr *ServerV2) ReceiveBlock(blk *block.Block) error {
	if err := svr.core.ReceiveBlock(blk); err != nil {
		return err
	}
	return svr.GrpcServer.chainListener.ReceiveBlock(blk)
}
