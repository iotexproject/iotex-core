package api

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"strconv"

	"github.com/ethereum/go-ethereum/core/vm"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

// GRPCServer contains grpc server and the pointer to api coreservice
type GRPCServer struct {
	port        string
	grpcServer  *grpc.Server
	coreService *coreService
}

// NewGRPCServer creates a new grpc server
func NewGRPCServer(core *coreService, grpcPort int) *GRPCServer {
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
	svr := &GRPCServer{
		port:        ":" + strconv.Itoa(grpcPort),
		grpcServer:  gSvr,
		coreService: core,
	}

	//serviceName: grpc.health.v1.Health
	grpc_health_v1.RegisterHealthServer(gSvr, health.NewServer())
	iotexapi.RegisterAPIServiceServer(gSvr, svr)
	grpc_prometheus.Register(gSvr)
	reflection.Register(gSvr)
	return svr
}

// Start starts the GRPC server
func (svr *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", svr.port)
	if err != nil {
		log.L().Error("grpc server failed to listen.", zap.Error(err))
		return errors.Wrap(err, "grpc server failed to listen")
	}
	log.L().Info("grpc server is listening.", zap.String("addr", lis.Addr().String()))
	go func() {
		if err := svr.grpcServer.Serve(lis); err != nil {
			log.L().Fatal("grpc failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the GRPC server
func (svr *GRPCServer) Stop() error {
	svr.grpcServer.Stop()
	return nil
}

// SuggestGasPrice suggests gas price
func (svr *GRPCServer) SuggestGasPrice(ctx context.Context, in *iotexapi.SuggestGasPriceRequest) (*iotexapi.SuggestGasPriceResponse, error) {
	suggestPrice, err := svr.coreService.SuggestGasPrice()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.SuggestGasPriceResponse{GasPrice: suggestPrice}, nil
}

// GetAccount returns the metadata of an account
func (svr *GRPCServer) GetAccount(ctx context.Context, in *iotexapi.GetAccountRequest) (*iotexapi.GetAccountResponse, error) {
	addr, err := address.FromString(in.Address)
	if err != nil {
		return nil, err
	}
	accountMeta, blockIdentifier, err := svr.coreService.Account(addr)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetAccountResponse{
		AccountMeta:     accountMeta,
		BlockIdentifier: blockIdentifier,
	}, nil
}

// GetActions returns actions
func (svr *GRPCServer) GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error) {
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
		ret, err = svr.coreService.Actions(request.Start, request.Count)
	case in.GetByHash() != nil:
		var act *iotexapi.ActionInfo
		request := in.GetByHash()
		act, err = svr.coreService.Action(request.ActionHash, request.CheckPending)
		ret = []*iotexapi.ActionInfo{act}
	case in.GetByAddr() != nil:
		request := in.GetByAddr()
		addr, err := address.FromString(request.Address)
		if err != nil {
			return nil, err
		}
		ret, err = svr.coreService.ActionsByAddress(addr, request.Start, request.Count)
	case in.GetUnconfirmedByAddr() != nil:
		request := in.GetUnconfirmedByAddr()
		ret, err = svr.coreService.UnconfirmedActionsByAddress(request.Address, request.Start, request.Count)
	case in.GetByBlk() != nil:
		request := in.GetByBlk()
		ret, err = svr.coreService.ActionsByBlock(request.BlkHash, request.Start, request.Count)
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
func (svr *GRPCServer) GetBlockMetas(ctx context.Context, in *iotexapi.GetBlockMetasRequest) (*iotexapi.GetBlockMetasResponse, error) {
	var (
		ret []*iotextypes.BlockMeta
		err error
	)
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		ret, err = svr.coreService.BlockMetas(request.Start, request.Count)
	case in.GetByHash() != nil:
		var blkMeta *iotextypes.BlockMeta
		request := in.GetByHash()
		blkMeta, err = svr.coreService.BlockMetaByHash(request.BlkHash)
		ret = []*iotextypes.BlockMeta{blkMeta}
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
func (svr *GRPCServer) GetChainMeta(ctx context.Context, in *iotexapi.GetChainMetaRequest) (*iotexapi.GetChainMetaResponse, error) {
	chainMeta, syncStatus, err := svr.coreService.ChainMeta()
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetChainMetaResponse{ChainMeta: chainMeta, SyncStage: syncStatus}, nil
}

// GetServerMeta gets the server metadata
func (svr *GRPCServer) GetServerMeta(ctx context.Context, in *iotexapi.GetServerMetaRequest) (*iotexapi.GetServerMetaResponse, error) {
	packageVersion, packageCommitID, gitStatus, goVersion, buildTime := svr.coreService.ServerMeta()
	return &iotexapi.GetServerMetaResponse{ServerMeta: &iotextypes.ServerMeta{
		PackageVersion:  packageVersion,
		PackageCommitID: packageCommitID,
		GitStatus:       gitStatus,
		GoVersion:       goVersion,
		BuildTime:       buildTime,
	}}, nil
}

// SendAction is the API to send an action to blockchain.
func (svr *GRPCServer) SendAction(ctx context.Context, in *iotexapi.SendActionRequest) (*iotexapi.SendActionResponse, error) {
	span := tracer.SpanFromContext(ctx)
	// tags output
	span.SetAttributes(attribute.String("actType", fmt.Sprintf("%T", in.GetAction().GetCore())))
	defer span.End()
	actHash, err := svr.coreService.SendAction(ctx, in.GetAction())
	if err != nil {
		return nil, err
	}
	return &iotexapi.SendActionResponse{ActionHash: actHash}, nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (svr *GRPCServer) GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error) {
	actHash, err := hash.HexStringToHash256(in.ActionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	receipt, blkHash, err := svr.coreService.ReceiptByAction(actHash)
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
func (svr *GRPCServer) ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest) (*iotexapi.ReadContractResponse, error) {
	from := in.CallerAddress
	if from == action.EmptyAddress {
		from = address.ZeroAddress
	}
	callerAddr, err := address.FromString(from)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	data, receipt, err := svr.coreService.ReadContract(ctx, in.Execution, callerAddr, in.GasLimit)
	if err != nil {
		return nil, err
	}
	return &iotexapi.ReadContractResponse{
		Data:    data,
		Receipt: receipt,
	}, nil
}

// ReadState reads state on blockchain
func (svr *GRPCServer) ReadState(ctx context.Context, in *iotexapi.ReadStateRequest) (*iotexapi.ReadStateResponse, error) {
	return svr.coreService.ReadState(string(in.ProtocolID), in.GetHeight(), in.MethodName, in.Arguments)
}

// EstimateGasForAction estimates gas for action
func (svr *GRPCServer) EstimateGasForAction(ctx context.Context, in *iotexapi.EstimateGasForActionRequest) (*iotexapi.EstimateGasForActionResponse, error) {
	estimateGas, err := svr.coreService.EstimateGasForAction(in.Action)
	if err != nil {
		return nil, err
	}
	return &iotexapi.EstimateGasForActionResponse{Gas: estimateGas}, nil
}

// EstimateActionGasConsumption estimate gas consume for action without signature
func (svr *GRPCServer) EstimateActionGasConsumption(ctx context.Context, in *iotexapi.EstimateActionGasConsumptionRequest) (*iotexapi.EstimateActionGasConsumptionResponse, error) {
	if in.GetExecution() != nil {
		callerAddr, err := address.FromString(in.GetCallerAddress())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		ret, err := svr.coreService.EstimateExecutionGasConsumption(ctx, in.GetExecution(), callerAddr)
		if err != nil {
			return nil, err
		}
		return &iotexapi.EstimateActionGasConsumptionResponse{Gas: ret}, nil
	}
	var intrinsicGas, payloadGas, payloadSize uint64
	switch {
	case in.GetTransfer() != nil:
		intrinsicGas, payloadGas, payloadSize = action.TransferBaseIntrinsicGas, action.TransferPayloadGas, uint64(len(in.GetTransfer().Payload))
	case in.GetStakeCreate() != nil:
		intrinsicGas, payloadGas, payloadSize = action.CreateStakeBaseIntrinsicGas, action.CreateStakePayloadGas, uint64(len(in.GetStakeCreate().Payload))
	case in.GetStakeUnstake() != nil:
		intrinsicGas, payloadGas, payloadSize = action.ReclaimStakeBaseIntrinsicGas, action.ReclaimStakePayloadGas, uint64(len(in.GetStakeUnstake().Payload))
	case in.GetStakeWithdraw() != nil:
		intrinsicGas, payloadGas, payloadSize = action.ReclaimStakeBaseIntrinsicGas, action.ReclaimStakePayloadGas, uint64(len(in.GetStakeWithdraw().Payload))
	case in.GetStakeAddDeposit() != nil:
		intrinsicGas, payloadGas, payloadSize = action.DepositToStakeBaseIntrinsicGas, action.DepositToStakePayloadGas, uint64(len(in.GetStakeAddDeposit().Payload))
	case in.GetStakeRestake() != nil:
		intrinsicGas, payloadGas, payloadSize = action.RestakeBaseIntrinsicGas, action.RestakePayloadGas, uint64(len(in.GetStakeRestake().Payload))
	case in.GetStakeChangeCandidate() != nil:
		intrinsicGas, payloadGas, payloadSize = action.MoveStakeBaseIntrinsicGas, action.MoveStakePayloadGas, uint64(len(in.GetStakeChangeCandidate().Payload))
	case in.GetStakeTransferOwnership() != nil:
		intrinsicGas, payloadGas, payloadSize = action.MoveStakeBaseIntrinsicGas, action.MoveStakePayloadGas, uint64(len(in.GetStakeTransferOwnership().Payload))
	case in.GetCandidateRegister() != nil:
		intrinsicGas, payloadGas, payloadSize = action.CandidateRegisterBaseIntrinsicGas, action.CandidateRegisterPayloadGas, uint64(len(in.GetCandidateRegister().Payload))
	case in.GetCandidateUpdate() != nil:
		intrinsicGas, payloadGas, payloadSize = action.CandidateUpdateBaseIntrinsicGas, 0, 0
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid argument")
	}
	return &iotexapi.EstimateActionGasConsumptionResponse{
		Gas: svr.coreService.CalculateGasConsumption(intrinsicGas, payloadGas, payloadSize),
	}, nil
}

// GetEpochMeta gets epoch metadata
func (svr *GRPCServer) GetEpochMeta(ctx context.Context, in *iotexapi.GetEpochMetaRequest) (*iotexapi.GetEpochMetaResponse, error) {
	epochData, numBlks, blockProducersInfo, err := svr.coreService.EpochMeta(in.EpochNumber)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetEpochMetaResponse{
		EpochData:          epochData,
		TotalBlocks:        numBlks,
		BlockProducersInfo: blockProducersInfo,
	}, nil
}

// GetRawBlocks gets raw block data
func (svr *GRPCServer) GetRawBlocks(ctx context.Context, in *iotexapi.GetRawBlocksRequest) (*iotexapi.GetRawBlocksResponse, error) {
	ret, err := svr.coreService.RawBlocks(in.StartHeight, in.Count, in.WithReceipts, in.WithTransactionLogs)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetRawBlocksResponse{Blocks: ret}, nil
}

// GetLogs get logs filtered by contract address and topics
func (svr *GRPCServer) GetLogs(ctx context.Context, in *iotexapi.GetLogsRequest) (*iotexapi.GetLogsResponse, error) {
	ret, err := svr.coreService.Logs(in)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetLogsResponse{Logs: ret}, err
}

// StreamBlocks streams blocks
func (svr *GRPCServer) StreamBlocks(in *iotexapi.StreamBlocksRequest, stream iotexapi.APIService_StreamBlocksServer) error {
	return svr.coreService.StreamBlocks(stream)
}

// StreamLogs streams logs that match the filter condition
func (svr *GRPCServer) StreamLogs(in *iotexapi.StreamLogsRequest, stream iotexapi.APIService_StreamLogsServer) error {
	return svr.coreService.StreamLogs(in.GetFilter(), stream)
}

// GetElectionBuckets returns the native election buckets.
func (svr *GRPCServer) GetElectionBuckets(ctx context.Context, in *iotexapi.GetElectionBucketsRequest) (*iotexapi.GetElectionBucketsResponse, error) {
	ret, err := svr.coreService.ElectionBuckets(in.GetEpochNum())
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetElectionBucketsResponse{Buckets: ret}, nil
}

// GetEvmTransfersByActionHash returns evm transfers by action hash
func (svr *GRPCServer) GetEvmTransfersByActionHash(ctx context.Context, in *iotexapi.GetEvmTransfersByActionHashRequest) (*iotexapi.GetEvmTransfersByActionHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByActionHash instead")
}

// GetEvmTransfersByBlockHeight returns evm transfers by block height
func (svr *GRPCServer) GetEvmTransfersByBlockHeight(ctx context.Context, in *iotexapi.GetEvmTransfersByBlockHeightRequest) (*iotexapi.GetEvmTransfersByBlockHeightResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByBlockHeight instead")
}

// GetTransactionLogByActionHash returns transaction log by action hash
func (svr *GRPCServer) GetTransactionLogByActionHash(ctx context.Context, in *iotexapi.GetTransactionLogByActionHashRequest) (*iotexapi.GetTransactionLogByActionHashResponse, error) {
	ret, err := svr.coreService.TransactionLogByActionHash(in.ActionHash)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetTransactionLogByActionHashResponse{
		TransactionLog: ret,
	}, nil
}

// GetTransactionLogByBlockHeight returns transaction log by block height
func (svr *GRPCServer) GetTransactionLogByBlockHeight(ctx context.Context, in *iotexapi.GetTransactionLogByBlockHeightRequest) (*iotexapi.GetTransactionLogByBlockHeightResponse, error) {
	blockIdentifier, transactionLogs, err := svr.coreService.TransactionLogByBlockHeight(in.BlockHeight)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetTransactionLogByBlockHeightResponse{
		BlockIdentifier: blockIdentifier,
		TransactionLogs: transactionLogs,
	}, nil
}

// GetActPoolActions returns the all Transaction Identifiers in the mempool
func (svr *GRPCServer) GetActPoolActions(ctx context.Context, in *iotexapi.GetActPoolActionsRequest) (*iotexapi.GetActPoolActionsResponse, error) {
	ret, err := svr.coreService.ActPoolActions(in.ActionHashes)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetActPoolActionsResponse{
		Actions: ret,
	}, nil
}

// ReadContractStorage reads contract's storage
func (svr *GRPCServer) ReadContractStorage(ctx context.Context, in *iotexapi.ReadContractStorageRequest) (*iotexapi.ReadContractStorageResponse, error) {
	addr, err := address.FromString(in.GetContract())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	b, err := svr.coreService.ReadContractStorage(ctx, addr, in.GetKey())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.ReadContractStorageResponse{Data: b}, nil
}

// TraceTransactionStructLogs get trace transaction struct logs
func (svr *GRPCServer) TraceTransactionStructLogs(ctx context.Context, in *iotexapi.TraceTransactionStructLogsRequest) (*iotexapi.TraceTransactionStructLogsResponse, error) {
	actInfo, err := svr.coreService.Action(util.Remove0xPrefix(in.GetActionHash()), false)
	if err != nil {
		return nil, err
	}
	exec, ok := actInfo.Action.Core.Action.(*iotextypes.ActionCore_Execution)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "the type of action is not supported")
	}

	amount, _ := big.NewInt(0).SetString(exec.Execution.GetAmount(), 10)
	callerAddr, err := address.FromString(actInfo.Sender)
	if err != nil {
		return nil, err
	}
	state, err := accountutil.AccountState(svr.coreService.sf, callerAddr)
	if err != nil {
		return nil, err
	}
	ctx, err = svr.coreService.bc.Context(ctx)
	if err != nil {
		return nil, err
	}
	tracer := vm.NewStructLogger(nil)
	ctx = protocol.WithVMConfigCtx(ctx, vm.Config{
		Debug:     true,
		Tracer:    tracer,
		NoBaseFee: true,
	})
	sc, _ := action.NewExecution(
		exec.Execution.GetContract(),
		state.Nonce+1,
		amount,
		svr.coreService.cfg.Genesis.BlockGasLimit,
		big.NewInt(0),
		exec.Execution.GetData(),
	)

	_, _, err = svr.coreService.sf.SimulateExecution(ctx, callerAddr, sc, svr.coreService.dao.GetBlockHash)
	if err != nil {
		return nil, err
	}

	structLogs := make([]*iotextypes.TransactionStructLog, 0)
	for _, log := range tracer.StructLogs() {
		var stack []string
		for _, s := range log.Stack {
			stack = append(stack, s.String())
		}
		structLogs = append(structLogs, &iotextypes.TransactionStructLog{
			Pc:         log.Pc,
			Op:         uint64(log.Op),
			Gas:        log.Gas,
			GasCost:    log.GasCost,
			Memory:     fmt.Sprintf("%#x", log.Memory),
			MemSize:    int32(log.MemorySize),
			Stack:      stack,
			ReturnData: fmt.Sprintf("%#x", log.ReturnData),
			Depth:      int32(log.Depth),
			Refund:     log.RefundCounter,
			OpName:     log.OpName(),
			Error:      log.ErrorString(),
		})
	}
	return &iotexapi.TraceTransactionStructLogsResponse{
		StructLogs: structLogs,
	}, nil
}
