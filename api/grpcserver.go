// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"
	"math"
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
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/version"
)

// GRPCServer contains grpc server and the pointer to api coreservice
type GRPCServer struct {
	port            string
	grpcServer      *grpc.Server
	coreService     CoreService
	chainListener   Listener
	rangeQueryLimit uint64
	tpsWindow       uint64
}

// NewGRPCServer creates a new grpc server
func NewGRPCServer(core CoreService, grpcPort int, queryLimit, tpsWindow uint64) *GRPCServer {
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
	// TODO: range query upper limit cannot be less than tps window
	svr := &GRPCServer{
		port:            ":" + strconv.Itoa(grpcPort),
		grpcServer:      gSvr,
		coreService:     core,
		chainListener:   NewChainListener(500),
		rangeQueryLimit: queryLimit,
		tpsWindow:       tpsWindow,
	}

	//serviceName: grpc.health.v1.Health
	grpc_health_v1.RegisterHealthServer(gSvr, health.NewServer())
	iotexapi.RegisterAPIServiceServer(gSvr, svr)
	grpc_prometheus.Register(gSvr)
	reflection.Register(gSvr)
	return svr
}

// Start starts the GRPC server
func (svr *GRPCServer) Start(_ context.Context) error {
	if err := svr.chainListener.Start(); err != nil {
		return errors.Wrap(err, "failed to start blockchain listener")
	}
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
func (svr *GRPCServer) Stop(_ context.Context) error {
	svr.grpcServer.Stop()
	return svr.chainListener.Stop()
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
	span := tracer.SpanFromContext(ctx)
	defer span.End()
	addr, err := address.FromString(in.Address)
	if err != nil {
		return nil, err
	}
	accountMeta, blockIdentifier, err := svr.coreService.Account(addr)
	if err != nil {
		return nil, err
	}
	span.AddEvent("response")
	span.SetAttributes(attribute.String("addr", in.Address))
	return &iotexapi.GetAccountResponse{
		AccountMeta:     accountMeta,
		BlockIdentifier: blockIdentifier,
	}, nil
}

// GetActions returns actions
func (svr *GRPCServer) GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error) {
	var (
		ret []*iotexapi.ActionInfo
		err error
	)
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		if err := svr.validateQueryLimit(request.Count); err != nil {
			return nil, err
		}
		ret, err = svr.coreService.Actions(request.Start, request.Count)
	case in.GetByHash() != nil:
		var act *iotexapi.ActionInfo
		request := in.GetByHash()
		act, err = svr.coreService.Action(request.ActionHash, request.CheckPending)
		ret = []*iotexapi.ActionInfo{act}
	case in.GetByAddr() != nil:
		request := in.GetByAddr()
		count := request.Count
		if err := svr.validateQueryLimit(count); err != nil {
			return nil, err
		}
		var addr address.Address
		addr, err = address.FromString(request.Address)
		if err != nil {
			return nil, err
		}
		ret, err = svr.coreService.ActionsByAddress(addr, request.Start, count)
	case in.GetUnconfirmedByAddr() != nil:
		request := in.GetUnconfirmedByAddr()
		count := request.Count
		if err := svr.validateQueryLimit(count); err != nil {
			return nil, err
		}
		ret, err = svr.coreService.UnconfirmedActionsByAddress(request.Address, request.Start, count)
	case in.GetByBlk() != nil:
		request := in.GetByBlk()
		count := request.Count
		if count == math.MaxUint64 {
			count = svr.rangeQueryLimit
		}
		if err := svr.validateQueryLimit(count); err != nil {
			return nil, err
		}
		ret, err = svr.coreService.ActionsByBlock(request.BlkHash, request.Start, count)
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
		count := request.Count
		if err := svr.validateQueryLimit(count); err != nil {
			return nil, err
		}
		ret, err = svr.coreService.BlockMetas(request.Start, count)
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
	chainMeta, syncStatus, err := svr.coreService.ChainMeta(svr.tpsWindow)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetChainMetaResponse{ChainMeta: chainMeta, SyncStage: syncStatus}, nil
}

// GetServerMeta gets the server metadata
func (svr *GRPCServer) GetServerMeta(ctx context.Context, in *iotexapi.GetServerMetaRequest) (*iotexapi.GetServerMetaResponse, error) {
	return &iotexapi.GetServerMetaResponse{ServerMeta: &iotextypes.ServerMeta{
		PackageVersion:  version.PackageVersion,
		PackageCommitID: version.PackageCommitID,
		GitStatus:       version.GitStatus,
		GoVersion:       version.GoVersion,
		BuildTime:       version.BuildTime,
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
	sc := &action.Execution{}
	if err := sc.LoadProto(in.GetExecution()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	sc.SetGasLimit(in.GetGasLimit())

	data, receipt, err := svr.coreService.ReadContract(ctx, callerAddr, sc)
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
		sc := &action.Execution{}
		if err := sc.LoadProto(in.GetExecution()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		ret, err := svr.coreService.EstimateExecutionGasConsumption(ctx, sc, callerAddr)
		if err != nil {
			return nil, err
		}
		return &iotexapi.EstimateActionGasConsumptionResponse{Gas: ret}, nil
	}
	var act action.Action
	switch {
	case in.GetTransfer() != nil:
		tmpAct := &action.Transfer{}
		if err := tmpAct.LoadProto(in.GetTransfer()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeCreate() != nil:
		tmpAct := &action.CreateStake{}
		if err := tmpAct.LoadProto(in.GetStakeCreate()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeUnstake() != nil:
		tmpAct := &action.Unstake{}
		if err := tmpAct.LoadProto(in.GetStakeUnstake()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeWithdraw() != nil:
		tmpAct := &action.WithdrawStake{}
		if err := tmpAct.LoadProto(in.GetStakeWithdraw()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeAddDeposit() != nil:
		tmpAct := &action.DepositToStake{}
		if err := tmpAct.LoadProto(in.GetStakeAddDeposit()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeRestake() != nil:
		tmpAct := &action.Restake{}
		if err := tmpAct.LoadProto(in.GetStakeRestake()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeChangeCandidate() != nil:
		tmpAct := &action.ChangeCandidate{}
		if err := tmpAct.LoadProto(in.GetStakeChangeCandidate()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetStakeTransferOwnership() != nil:
		tmpAct := &action.TransferStake{}
		if err := tmpAct.LoadProto(in.GetStakeTransferOwnership()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetCandidateRegister() != nil:
		tmpAct := &action.CandidateRegister{}
		if err := tmpAct.LoadProto(in.GetCandidateRegister()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	case in.GetCandidateUpdate() != nil:
		tmpAct := &action.CandidateUpdate{}
		if err := tmpAct.LoadProto(in.GetCandidateUpdate()); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		act = tmpAct
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid argument")
	}
	estimatedGas, err := svr.coreService.EstimateGasForNonExecution(act)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &iotexapi.EstimateActionGasConsumptionResponse{Gas: estimatedGas}, nil
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
	if in.Count == 0 || in.Count > svr.rangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}
	ret, err := svr.coreService.RawBlocks(in.StartHeight, in.Count, in.WithReceipts, in.WithTransactionLogs)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetRawBlocksResponse{Blocks: ret}, nil
}

// GetLogs get logs filtered by contract address and topics
func (svr *GRPCServer) GetLogs(ctx context.Context, in *iotexapi.GetLogsRequest) (*iotexapi.GetLogsResponse, error) {
	if in.GetFilter() == nil {
		return nil, status.Error(codes.InvalidArgument, "empty filter")
	}
	var (
		ret []*iotextypes.Log
		err error
	)
	switch {
	case in.GetByBlock() != nil:
		ret, err = svr.coreService.LogsInBlockByHash(logfilter.NewLogFilter(in.GetFilter(), nil, nil), hash.BytesToHash256(in.GetByBlock().BlockHash))
	case in.GetByRange() != nil:
		req := in.GetByRange()
		ret, err = svr.coreService.LogsInRange(logfilter.NewLogFilter(in.GetFilter(), nil, nil), req.GetFromBlock(), req.GetToBlock(), req.GetPaginationSize())
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid GetLogsRequest type")
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &iotexapi.GetLogsResponse{Logs: ret}, err
}

// StreamBlocks streams blocks
func (svr *GRPCServer) StreamBlocks(in *iotexapi.StreamBlocksRequest, stream iotexapi.APIService_StreamBlocksServer) error {
	errChan := make(chan error)
	if err := svr.chainListener.AddResponder(NewBlockListener(stream, errChan)); err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for {
		select {
		case err := <-errChan:
			if err != nil {
				err = status.Error(codes.Aborted, err.Error())
			}
			return err
		}
	}
}

// StreamLogs streams logs that match the filter condition
func (svr *GRPCServer) StreamLogs(in *iotexapi.StreamLogsRequest, stream iotexapi.APIService_StreamLogsServer) error {
	if in == nil {
		return status.Error(codes.InvalidArgument, "empty filter")
	}
	errChan := make(chan error)
	// register the log filter so it will match logs in new blocks
	if err := svr.chainListener.AddResponder(logfilter.NewLogFilter(in.GetFilter(), stream, errChan)); err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for {
		select {
		case err := <-errChan:
			if err != nil {
				err = status.Error(codes.Aborted, err.Error())
			}
			return err
		}
	}
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
	callerAddr, err := address.FromString(actInfo.Sender)
	if err != nil {
		return nil, err
	}
	tracer := vm.NewStructLogger(nil)
	ctx = protocol.WithVMConfigCtx(ctx, vm.Config{
		Debug:     true,
		Tracer:    tracer,
		NoBaseFee: true,
	})
	amount, ok := new(big.Int).SetString(exec.Execution.GetAmount(), 10)
	if !ok {
		return nil, errors.New("failed to set execution amount")
	}
	sc, err := action.NewExecution(
		exec.Execution.GetContract(),
		actInfo.Action.Core.Nonce,
		amount,
		actInfo.Action.Core.GasLimit,
		big.NewInt(0),
		exec.Execution.GetData(),
	)
	if err != nil {
		return nil, err
	}

	_, _, err = svr.coreService.SimulateExecution(ctx, callerAddr, sc)
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

func (svr *GRPCServer) validateQueryLimit(count uint64) error {
	if count == 0 {
		return status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > svr.rangeQueryLimit {
		return status.Error(codes.InvalidArgument, "range exceeds the limit")
	}
	return nil
}
