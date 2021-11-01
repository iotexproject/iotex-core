package api

import (
	"context"
	"fmt"
	"net"
	"strconv"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

type GrpcServer struct {
	grpcServer  *grpc.Server
	port        string
	coreService *CoreService
}

func NewGRPCServer(core *CoreService, grpcPort int) *GrpcServer {
	gSvr := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_prometheus.StreamServerInterceptor,
			otelgrpc.StreamServerInterceptor(),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_prometheus.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		)),
	)
	svr := &GrpcServer{
		grpcServer:  gSvr,
		coreService: core,
		port:        ":" + strconv.Itoa(grpcPort),
	}

	//serviceName: grpc.health.v1.Health
	grpc_health_v1.RegisterHealthServer(gSvr, health.NewServer())
	iotexapi.RegisterAPIServiceServer(gSvr, svr)
	grpc_prometheus.Register(gSvr)
	reflection.Register(gSvr)
	return svr
}

func (api *GrpcServer) Start() error {
	lis, err := net.Listen("tcp", api.port)
	if err != nil {
		log.L().Error("grpc server failed to listen.", zap.Error(err))
		return errors.Wrap(err, "grpc server failed to listen")
	}
	log.L().Info("grpc server is listening.", zap.String("addr", lis.Addr().String()))
	go func() {
		if err := api.grpcServer.Serve(lis); err != nil {
			log.L().Fatal("grpc failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

func (api *GrpcServer) Stop() {
	api.grpcServer.Stop()
}

// SuggestGasPrice suggests gas price
func (svr *GrpcServer) SuggestGasPrice(ctx context.Context, in *iotexapi.SuggestGasPriceRequest) (*iotexapi.SuggestGasPriceResponse, error) {
	suggestPrice, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.SuggestGasPriceResponse{GasPrice: suggestPrice}, nil
}

// GetAccount returns the metadata of an account
func (svr *GrpcServer) GetAccount(ctx context.Context, in *iotexapi.GetAccountRequest) (*iotexapi.GetAccountResponse, error) {
	accountMeta, blockIdentifier, err := svr.coreService.GetAccount(in.Address)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetAccountResponse{
			AccountMeta:     accountMeta,
			BlockIdentifier: blockIdentifier},
		nil
}

// GetActions returns actions
func (svr *GrpcServer) GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error) {
	if (!svr.coreService.hasActionIndex || svr.coreService.indexer == nil) && (in.GetByHash() != nil || in.GetByAddr() != nil) {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	var (
		ret []*iotexapi.ActionInfo
		err error
	)
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		ret, err = svr.coreService.GetActions(request.Start, request.Count)
	case in.GetByHash() != nil:
		request := in.GetByHash()
		ret, err = svr.coreService.GetSingleAction(request.ActionHash, request.CheckPending)
	case in.GetByAddr() != nil:
		request := in.GetByAddr()
		ret, err = svr.coreService.GetActionsByAddress(request.Address, request.Start, request.Count)
	case in.GetUnconfirmedByAddr() != nil:
		request := in.GetUnconfirmedByAddr()
		ret, err = svr.coreService.GetUnconfirmedActionsByAddress(request.Address, request.Start, request.Count)
	case in.GetByBlk() != nil:
		request := in.GetByBlk()
		ret, err = svr.coreService.GetActionsByBlock(request.BlkHash, request.Start, request.Count)
	default:
		return nil, status.Error(codes.NotFound, "invalid GetActionsRequest type")
	}
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetActionsResponse{
		Total:      uint64(len(ret)),
		ActionInfo: ret,
	}, nil
}

// GetBlockMetas returns block metadata
func (svr *GrpcServer) GetBlockMetas(ctx context.Context, in *iotexapi.GetBlockMetasRequest) (*iotexapi.GetBlockMetasResponse, error) {
	var (
		ret []*iotextypes.BlockMeta
		err error
	)
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		ret, err = svr.coreService.GetBlockMetas(request.Start, request.Count)
	case in.GetByHash() != nil:
		request := in.GetByHash()
		ret, err = svr.coreService.GetBlockMetaByHash(request.BlkHash)
	default:
		return nil, status.Error(codes.NotFound, "invalid GetBlockMetasRequest type")
	}
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetBlockMetasResponse{
		Total:    uint64(len(ret)),
		BlkMetas: ret,
	}, nil
}

// GetChainMeta returns blockchain metadata
func (svr *GrpcServer) GetChainMeta(ctx context.Context, in *iotexapi.GetChainMetaRequest) (*iotexapi.GetChainMetaResponse, error) {
	chainMeta, syncStatus, err := svr.coreService.GetChainMeta()
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetChainMetaResponse{ChainMeta: chainMeta, SyncStage: syncStatus}, nil
}

// GetServerMeta gets the server metadata
func (svr *GrpcServer) GetServerMeta(ctx context.Context, in *iotexapi.GetServerMetaRequest) (*iotexapi.GetServerMetaResponse, error) {
	return svr.coreService.GetServerMeta()
}

// SendAction is the API to send an action to blockchain.
func (svr *GrpcServer) SendAction(ctx context.Context, in *iotexapi.SendActionRequest) (*iotexapi.SendActionResponse, error) {
	span := tracer.SpanFromContext(ctx)
	// tags output
	span.SetAttributes(attribute.String("actType", fmt.Sprintf("%T", in.GetAction().GetCore())))
	defer span.End()
	actHash, err := svr.coreService.SendAction(in.GetAction(), in.GetAction().Core.GetChainID(), context.Background())
	if err != nil {
		return nil, err
	}
	return &iotexapi.SendActionResponse{ActionHash: actHash}, nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (svr *GrpcServer) GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error) {
	receipt, blkHash, err := svr.coreService.GetReceiptByAction(in.ActionHash)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetReceiptByActionResponse{
		ReceiptInfo: &iotexapi.ReceiptInfo{
			Receipt: receipt.ConvertToReceiptPb(),
			BlkHash: blkHash,
		},
	}, nil
}

// ReadContract reads the state in a contract address specified by the slot
func (svr *GrpcServer) ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest) (*iotexapi.ReadContractResponse, error) {
	return svr.coreService.ReadContract(in.Execution, in.CallerAddress, in.GasLimit, ctx)
}

// ReadState reads state on blockchain
func (svr *GrpcServer) ReadState(ctx context.Context, in *iotexapi.ReadStateRequest) (*iotexapi.ReadStateResponse, error) {
	data, blockIdentifier, err := svr.coreService.ReadState(string(in.ProtocolID), in.GetHeight(), in.MethodName, in.Arguments)
	if err != nil {
		return nil, err
	}
	return &iotexapi.ReadStateResponse{
		Data:            data,
		BlockIdentifier: blockIdentifier,
	}, nil
}

// EstimateGasForAction estimates gas for action
func (svr *GrpcServer) EstimateGasForAction(ctx context.Context, in *iotexapi.EstimateGasForActionRequest) (*iotexapi.EstimateGasForActionResponse, error) {
	estimateGas, err := svr.coreService.EstimateGasForAction(in.Action)
	if err != nil {
		return nil, err
	}
	return &iotexapi.EstimateGasForActionResponse{Gas: estimateGas}, nil
}

// EstimateActionGasConsumption estimate gas consume for action without signature
func (svr *GrpcServer) EstimateActionGasConsumption(ctx context.Context, in *iotexapi.EstimateActionGasConsumptionRequest) (respone *iotexapi.EstimateActionGasConsumptionResponse, err error) {
	ret, err := svr.coreService.EstimateActionGasConsumption(ctx, in)
	if err != nil {
		return nil, err
	}
	return &iotexapi.EstimateActionGasConsumptionResponse{Gas: ret}, nil
}

// GetEpochMeta gets epoch metadata
func (svr *GrpcServer) GetEpochMeta(ctx context.Context, in *iotexapi.GetEpochMetaRequest) (*iotexapi.GetEpochMetaResponse, error) {
	return svr.coreService.GetEpochMeta(in.EpochNumber)
}

// GetRawBlocks gets raw block data
func (svr *GrpcServer) GetRawBlocks(ctx context.Context, in *iotexapi.GetRawBlocksRequest) (*iotexapi.GetRawBlocksResponse, error) {
	ret, err := svr.coreService.GetRawBlocks(in.StartHeight, in.Count, in.WithReceipts, in.WithTransactionLogs)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetRawBlocksResponse{Blocks: ret}, nil
}

// StreamBlocks streams blocks
func (svr *GrpcServer) StreamBlocks(in *iotexapi.StreamBlocksRequest, stream iotexapi.APIService_StreamBlocksServer) error {
	return svr.coreService.StreamBlocks(stream)
}

// StreamLogs streams logs that match the filter condition
func (svr *GrpcServer) StreamLogs(in *iotexapi.StreamLogsRequest, stream iotexapi.APIService_StreamLogsServer) error {
	return svr.coreService.StreamLogs(in.GetFilter(), stream)
}

// GetElectionBuckets returns the native election buckets.
func (svr *GrpcServer) GetElectionBuckets(ctx context.Context, in *iotexapi.GetElectionBucketsRequest) (*iotexapi.GetElectionBucketsResponse, error) {
	ret, err := svr.coreService.GetElectionBuckets(in.GetEpochNum())
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetElectionBucketsResponse{Buckets: ret}, nil
}

// GetEvmTransfersByActionHash returns evm transfers by action hash
func (svr *GrpcServer) GetEvmTransfersByActionHash(ctx context.Context, in *iotexapi.GetEvmTransfersByActionHashRequest) (*iotexapi.GetEvmTransfersByActionHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByActionHash instead")
}

// GetEvmTransfersByBlockHeight returns evm transfers by block height
func (svr *GrpcServer) GetEvmTransfersByBlockHeight(ctx context.Context, in *iotexapi.GetEvmTransfersByBlockHeightRequest) (*iotexapi.GetEvmTransfersByBlockHeightResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByBlockHeight instead")
}

// GetTransactionLogByActionHash returns transaction log by action hash
func (svr *GrpcServer) GetTransactionLogByActionHash(ctx context.Context, in *iotexapi.GetTransactionLogByActionHashRequest) (*iotexapi.GetTransactionLogByActionHashResponse, error) {
	ret, err := svr.coreService.GetTransactionLogByActionHash(in.ActionHash)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetTransactionLogByActionHashResponse{
		TransactionLog: ret,
	}, nil
}

// GetTransactionLogByBlockHeight returns transaction log by block height
func (svr *GrpcServer) GetTransactionLogByBlockHeight(ctx context.Context, in *iotexapi.GetTransactionLogByBlockHeightRequest) (*iotexapi.GetTransactionLogByBlockHeightResponse, error) {
	blockIdentifier, transactionLogs, err := svr.coreService.GetTransactionLogByBlockHeight(in.BlockHeight)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetTransactionLogByBlockHeightResponse{
		BlockIdentifier: blockIdentifier,
		TransactionLogs: transactionLogs,
	}, nil
}

// GetActPoolActions returns the all Transaction Identifiers in the mempool
func (svr *GrpcServer) GetActPoolActions(ctx context.Context, in *iotexapi.GetActPoolActionsRequest) (*iotexapi.GetActPoolActionsResponse, error) {
	ret, err := svr.coreService.GetActPoolActions(in.ActionHashes)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetActPoolActionsResponse{
		Actions: ret,
	}, nil
}

// GetLogs get logs filtered by contract address and topics
func (svr *GrpcServer) GetLogs(ctx context.Context, in *iotexapi.GetLogsRequest) (*iotexapi.GetLogsResponse, error) {
	ret, err := svr.coreService.GetLogs(in)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetLogsResponse{Logs: ret}, err
}
