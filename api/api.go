// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
	// ErrReceipt indicates the error of receipt
	ErrReceipt = errors.New("invalid receipt")
	// ErrAction indicates the error of action
	ErrAction = errors.New("invalid action")
)

// BroadcastOutbound sends a broadcast message to the whole network
type BroadcastOutbound func(ctx context.Context, chainID uint32, msg proto.Message) error

// Config represents the config to setup api
type Config struct {
	broadcastHandler  BroadcastOutbound
	electionCommittee committee.Committee
}

// Option is the option to override the api config
type Option func(cfg *Config) error

// WithBroadcastOutbound is the option to broadcast msg outbound
func WithBroadcastOutbound(broadcastHandler BroadcastOutbound) Option {
	return func(cfg *Config) error {
		cfg.broadcastHandler = broadcastHandler
		return nil
	}
}

// WithNativeElection is the option to return native election data through API.
func WithNativeElection(committee committee.Committee) Option {
	return func(cfg *Config) error {
		cfg.electionCommittee = committee
		return nil
	}
}

// Server provides api for user to query blockchain data
type Server struct {
	bc                blockchain.Blockchain
	sf                factory.Factory
	dao               blockdao.BlockDAO
	indexer           blockindex.Indexer
	bfIndexer         blockindex.BloomFilterIndexer
	ap                actpool.ActPool
	gs                *gasstation.GasStation
	broadcastHandler  BroadcastOutbound
	cfg               config.Config
	registry          *protocol.Registry
	chainListener     Listener
	grpcServer        *grpc.Server
	hasActionIndex    bool
	electionCommittee committee.Committee
}

// NewServer creates a new server
func NewServer(
	cfg config.Config,
	chain blockchain.Blockchain,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	actPool actpool.ActPool,
	registry *protocol.Registry,
	opts ...Option,
) (*Server, error) {
	apiCfg := Config{}
	for _, opt := range opts {
		if err := opt(&apiCfg); err != nil {
			return nil, err
		}
	}

	if cfg.API == (config.API{}) {
		log.L().Warn("API server is not configured.")
		cfg.API = config.Default.API
	}

	if cfg.API.RangeQueryLimit < uint64(cfg.API.TpsWindow) {
		return nil, errors.New("range query upper limit cannot be less than tps window")
	}

	svr := &Server{
		bc:                chain,
		sf:                sf,
		dao:               dao,
		indexer:           indexer,
		bfIndexer:         bfIndexer,
		ap:                actPool,
		broadcastHandler:  apiCfg.broadcastHandler,
		cfg:               cfg,
		registry:          registry,
		chainListener:     NewChainListener(),
		gs:                gasstation.NewGasStation(chain, sf.SimulateExecution, dao, cfg.API),
		electionCommittee: apiCfg.electionCommittee,
	}
	if _, ok := cfg.Plugins[config.GatewayPlugin]; ok {
		svr.hasActionIndex = true
	}
	svr.grpcServer = grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	iotexapi.RegisterAPIServiceServer(svr.grpcServer, svr)
	grpc_prometheus.Register(svr.grpcServer)
	reflection.Register(svr.grpcServer)

	return svr, nil
}

// GetAccount returns the metadata of an account
func (api *Server) GetAccount(ctx context.Context, in *iotexapi.GetAccountRequest) (*iotexapi.GetAccountResponse, error) {
	if in.Address == address.RewardingPoolAddr || in.Address == address.StakingBucketPoolAddr {
		return api.getProtocolAccount(ctx, in.Address)
	}

	state, tipHeight, err := accountutil.AccountStateWithHeight(api.sf, in.Address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	pendingNonce, err := api.ap.GetPendingNonce(in.Address)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if api.indexer == nil {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	addr, err := address.FromString(in.Address)
	if err != nil {
		return nil, err
	}
	numActions, err := api.indexer.GetActionCountByAddress(hash.BytesToHash160(addr.Bytes()))
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	accountMeta := &iotextypes.AccountMeta{
		Address:      in.Address,
		Balance:      state.Balance.String(),
		Nonce:        state.Nonce,
		PendingNonce: pendingNonce,
		NumActions:   numActions,
		IsContract:   state.IsContract(),
	}
	header, err := api.bc.BlockHeaderByHeight(tipHeight)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	hash := header.HashBlock()
	return &iotexapi.GetAccountResponse{AccountMeta: accountMeta, BlockIdentifier: &iotextypes.BlockIdentifier{
		Hash:   hex.EncodeToString(hash[:]),
		Height: tipHeight,
	}}, nil
}

// GetActions returns actions
func (api *Server) GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error) {
	if (!api.hasActionIndex || api.indexer == nil) && (in.GetByHash() != nil || in.GetByAddr() != nil) {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		return api.getActions(request.Start, request.Count)
	case in.GetByHash() != nil:
		request := in.GetByHash()
		return api.getSingleAction(request.ActionHash, request.CheckPending)
	case in.GetByAddr() != nil:
		request := in.GetByAddr()
		return api.getActionsByAddress(request.Address, request.Start, request.Count)
	case in.GetUnconfirmedByAddr() != nil:
		request := in.GetUnconfirmedByAddr()
		return api.getUnconfirmedActionsByAddress(request.Address, request.Start, request.Count)
	case in.GetByBlk() != nil:
		request := in.GetByBlk()
		return api.getActionsByBlock(request.BlkHash, request.Start, request.Count)
	default:
		return nil, status.Error(codes.NotFound, "invalid GetActionsRequest type")
	}
}

// GetBlockMetas returns block metadata
func (api *Server) GetBlockMetas(ctx context.Context, in *iotexapi.GetBlockMetasRequest) (*iotexapi.GetBlockMetasResponse, error) {
	switch {
	case in.GetByIndex() != nil:
		request := in.GetByIndex()
		return api.getBlockMetas(request.Start, request.Count)
	case in.GetByHash() != nil:
		request := in.GetByHash()
		return api.getBlockMeta(request.BlkHash)
	default:
		return nil, status.Error(codes.NotFound, "invalid GetBlockMetasRequest type")
	}
}

// GetChainMeta returns blockchain metadata
func (api *Server) GetChainMeta(ctx context.Context, in *iotexapi.GetChainMetaRequest) (*iotexapi.GetChainMetaResponse, error) {
	tipHeight := api.bc.TipHeight()
	if tipHeight == 0 {
		return &iotexapi.GetChainMetaResponse{
			ChainMeta: &iotextypes.ChainMeta{
				Epoch: &iotextypes.EpochData{},
			},
		}, nil
	}
	if api.indexer == nil {
		// TODO: in case indexer does not exist, may consider return a value like 0 instead of exit
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	totalActions, err := api.indexer.GetTotalActions()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	blockLimit := int64(api.cfg.API.TpsWindow)
	if blockLimit <= 0 {
		return nil, status.Errorf(codes.Internal, "block limit is %d", blockLimit)
	}

	// avoid genesis block
	if int64(tipHeight) < blockLimit {
		blockLimit = int64(tipHeight)
	}
	r, err := api.getBlockMetas(tipHeight-uint64(blockLimit)+1, uint64(blockLimit))
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blks := r.BlkMetas

	if len(blks) == 0 {
		return nil, status.Error(codes.NotFound, "get 0 blocks! not able to calculate aps")
	}

	var numActions int64
	for _, blk := range blks {
		numActions += blk.NumActions
	}

	t1 := time.Unix(blks[0].Timestamp.GetSeconds(), int64(blks[0].Timestamp.GetNanos()))
	t2 := time.Unix(blks[len(blks)-1].Timestamp.GetSeconds(), int64(blks[len(blks)-1].Timestamp.GetNanos()))
	// duration of time difference in milli-seconds
	// TODO: use config.Genesis.BlockInterval after PR1289 merges
	timeDiff := (t2.Sub(t1) + 10*time.Second) / time.Millisecond
	tps := float32(numActions*1000) / float32(timeDiff)

	chainMeta := &iotextypes.ChainMeta{
		Height:     tipHeight,
		NumActions: int64(totalActions),
		Tps:        int64(math.Ceil(float64(tps))),
		TpsFloat:   tps,
	}

	rp := rolldpos.FindProtocol(api.registry)
	if rp != nil {
		epochNum := rp.GetEpochNum(tipHeight)
		epochHeight := rp.GetEpochHeight(epochNum)
		gravityChainStartHeight, err := api.getGravityChainStartHeight(epochHeight)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		chainMeta.Epoch = &iotextypes.EpochData{
			Num:                     epochNum,
			Height:                  epochHeight,
			GravityChainStartHeight: gravityChainStartHeight,
		}
	}
	return &iotexapi.GetChainMetaResponse{ChainMeta: chainMeta}, nil
}

// GetServerMeta gets the server metadata
func (api *Server) GetServerMeta(ctx context.Context,
	in *iotexapi.GetServerMetaRequest) (*iotexapi.GetServerMetaResponse, error) {
	return &iotexapi.GetServerMetaResponse{ServerMeta: &iotextypes.ServerMeta{
		PackageVersion:  version.PackageVersion,
		PackageCommitID: version.PackageCommitID,
		GitStatus:       version.GitStatus,
		GoVersion:       version.GoVersion,
		BuildTime:       version.BuildTime,
	}}, nil
}

// SendAction is the API to send an action to blockchain.
func (api *Server) SendAction(ctx context.Context, in *iotexapi.SendActionRequest) (*iotexapi.SendActionResponse, error) {
	log.L().Debug("receive send action request")
	var selp action.SealedEnvelope
	var err error
	if err = selp.LoadProto(in.Action); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// Add to local actpool
	ctx = protocol.WithRegistry(ctx, api.registry)
	if err = api.ap.Add(ctx, selp); err != nil {
		log.L().Debug(err.Error())
		var desc string
		switch errors.Cause(err) {
		case action.ErrBalance:
			desc = "Invalid balance"
		case action.ErrInsufficientBalanceForGas:
			desc = "Insufficient balance for gas"
		case action.ErrNonce:
			desc = "Invalid nonce"
		case action.ErrAddress:
			desc = "Blacklisted address"
		case action.ErrActPool:
			desc = "Invalid actpool"
		case action.ErrGasPrice:
			desc = "Invalid gas price"
		default:
			desc = "Unknown"
		}
		st := status.New(codes.Internal, err.Error())
		v := &errdetails.BadRequest_FieldViolation{
			Field:       "Action rejected",
			Description: desc,
		}
		br := &errdetails.BadRequest{}
		br.FieldViolations = append(br.FieldViolations, v)
		st, err := st.WithDetails(br)
		if err != nil {
			log.S().Panicf("Unexpected error attaching metadata: %v", err)
		}
		return nil, st.Err()
	}
	// If there is no error putting into local actpool,
	// Broadcast it to the network
	if err = api.broadcastHandler(context.Background(), api.bc.ChainID(), in.Action); err != nil {
		log.L().Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	hash := selp.Hash()
	return &iotexapi.SendActionResponse{ActionHash: hex.EncodeToString(hash[:])}, nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (api *Server) GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error) {
	if !api.hasActionIndex || api.indexer == nil {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	actHash, err := hash.HexStringToHash256(in.ActionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	receipt, err := api.GetReceiptByActionHash(actHash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := api.getBlockHashByActionHash(actHash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &iotexapi.GetReceiptByActionResponse{
		ReceiptInfo: &iotexapi.ReceiptInfo{
			Receipt: receipt.ConvertToReceiptPb(),
			BlkHash: hex.EncodeToString(blkHash[:]),
		},
	}, nil
}

// ReadContract reads the state in a contract address specified by the slot
func (api *Server) ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest) (*iotexapi.ReadContractResponse, error) {
	log.L().Debug("receive read smart contract request")

	sc := &action.Execution{}
	if err := sc.LoadProto(in.Execution); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	state, err := accountutil.AccountState(api.sf, in.CallerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sc, _ = action.NewExecution(
		sc.Contract(),
		state.Nonce+1,
		sc.Amount(),
		api.cfg.Genesis.BlockGasLimit,
		big.NewInt(0),
		sc.Data(),
	)

	callerAddr, err := address.FromString(in.CallerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	ctx, err = api.bc.Context()
	if err != nil {
		return nil, err
	}
	retval, receipt, err := api.sf.SimulateExecution(ctx, callerAddr, sc, api.dao.GetBlockHash)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.ReadContractResponse{
		Data:    hex.EncodeToString(retval),
		Receipt: receipt.ConvertToReceiptPb(),
	}, nil
}

// ReadState reads state on blockchain
func (api *Server) ReadState(ctx context.Context, in *iotexapi.ReadStateRequest) (*iotexapi.ReadStateResponse, error) {
	p, ok := api.registry.Find(string(in.ProtocolID))
	if !ok {
		return nil, status.Errorf(codes.Internal, "protocol %s isn't registered", string(in.ProtocolID))
	}
	data, readStateHeight, err := api.readState(ctx, p, in.GetHeight(), in.MethodName, in.Arguments...)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := api.dao.GetBlockHash(readStateHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := iotexapi.ReadStateResponse{
		Data: data,
		BlockIdentifier: &iotextypes.BlockIdentifier{
			Height: readStateHeight,
			Hash:   hex.EncodeToString(blkHash[:]),
		},
	}
	return &out, nil
}

// SuggestGasPrice suggests gas price
func (api *Server) SuggestGasPrice(ctx context.Context, in *iotexapi.SuggestGasPriceRequest) (*iotexapi.SuggestGasPriceResponse, error) {
	suggestPrice, err := api.gs.SuggestGasPrice()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.SuggestGasPriceResponse{GasPrice: suggestPrice}, nil
}

// EstimateGasForAction estimates gas for action
func (api *Server) EstimateGasForAction(ctx context.Context, in *iotexapi.EstimateGasForActionRequest) (*iotexapi.EstimateGasForActionResponse, error) {
	estimateGas, err := api.gs.EstimateGasForAction(in.Action)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.EstimateGasForActionResponse{Gas: estimateGas}, nil
}

// EstimateActionGasConsumption estimate gas consume for action without signature
func (api *Server) EstimateActionGasConsumption(ctx context.Context, in *iotexapi.EstimateActionGasConsumptionRequest) (respone *iotexapi.EstimateActionGasConsumptionResponse, err error) {
	respone = &iotexapi.EstimateActionGasConsumptionResponse{}
	switch {
	case in.GetExecution() != nil:
		request := in.GetExecution()
		return api.estimateActionGasConsumptionForExecution(request, in.GetCallerAddress())
	case in.GetTransfer() != nil:
		respone.Gas = uint64(len(in.GetTransfer().Payload))*action.TransferPayloadGas + action.TransferBaseIntrinsicGas
	case in.GetStakeCreate() != nil:
		respone.Gas = uint64(len(in.GetStakeCreate().Payload))*action.CreateStakePayloadGas + action.CreateStakeBaseIntrinsicGas
	case in.GetStakeUnstake() != nil:
		respone.Gas = uint64(len(in.GetStakeUnstake().Payload))*action.ReclaimStakePayloadGas + action.ReclaimStakeBaseIntrinsicGas
	case in.GetStakeWithdraw() != nil:
		respone.Gas = uint64(len(in.GetStakeWithdraw().Payload))*action.ReclaimStakePayloadGas + action.ReclaimStakeBaseIntrinsicGas
	case in.GetStakeAddDeposit() != nil:
		respone.Gas = uint64(len(in.GetStakeAddDeposit().Payload))*action.DepositToStakePayloadGas + action.DepositToStakeBaseIntrinsicGas
	case in.GetStakeRestake() != nil:
		respone.Gas = uint64(len(in.GetStakeRestake().Payload))*action.RestakePayloadGas + action.RestakeBaseIntrinsicGas
	case in.GetStakeChangeCandidate() != nil:
		respone.Gas = uint64(len(in.GetStakeChangeCandidate().Payload))*action.MoveStakePayloadGas + action.MoveStakeBaseIntrinsicGas
	case in.GetStakeTransferOwnership() != nil:
		respone.Gas = uint64(len(in.GetStakeTransferOwnership().Payload))*action.MoveStakePayloadGas + action.MoveStakeBaseIntrinsicGas
	case in.GetCandidateRegister() != nil:
		respone.Gas = uint64(len(in.GetCandidateRegister().Payload))*action.CandidateRegisterPayloadGas + action.CandidateRegisterBaseIntrinsicGas
	case in.GetCandidateUpdate() != nil:
		respone.Gas = action.CandidateUpdateBaseIntrinsicGas
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid argument")
	}
	return
}

// GetEpochMeta gets epoch metadata
func (api *Server) GetEpochMeta(
	ctx context.Context,
	in *iotexapi.GetEpochMetaRequest,
) (*iotexapi.GetEpochMetaResponse, error) {
	rp := rolldpos.FindProtocol(api.registry)
	if rp == nil {
		return &iotexapi.GetEpochMetaResponse{}, nil
	}
	if in.EpochNumber < 1 {
		return nil, status.Error(codes.InvalidArgument, "epoch number cannot be less than one")
	}
	epochHeight := rp.GetEpochHeight(in.EpochNumber)
	gravityChainStartHeight, err := api.getGravityChainStartHeight(epochHeight)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	epochData := &iotextypes.EpochData{
		Num:                     in.EpochNumber,
		Height:                  epochHeight,
		GravityChainStartHeight: gravityChainStartHeight,
	}

	pp := poll.FindProtocol(api.registry)
	if pp == nil {
		return nil, status.Error(codes.Internal, "poll protocol is not registered")
	}

	methodName := []byte("ActiveBlockProducersByEpoch")
	arguments := [][]byte{[]byte(strconv.FormatUint(in.EpochNumber, 10))}
	height := strconv.FormatUint(epochHeight, 10)
	data, _, err := api.readState(context.Background(), pp, height, methodName, arguments...)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var activeConsensusBlockProducers state.CandidateList
	if err := activeConsensusBlockProducers.Deserialize(data); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	numBlks, produce, err := api.getProductivityByEpoch(rp, in.EpochNumber, api.bc.TipHeight(), activeConsensusBlockProducers)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	methodName = []byte("BlockProducersByEpoch")
	data, _, err = api.readState(context.Background(), pp, height, methodName, arguments...)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var BlockProducers state.CandidateList
	if err := BlockProducers.Deserialize(data); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var blockProducersInfo []*iotexapi.BlockProducerInfo
	for _, bp := range BlockProducers {
		var active bool
		var blockProduction uint64
		if production, ok := produce[bp.Address]; ok {
			active = true
			blockProduction = production
		}
		blockProducersInfo = append(blockProducersInfo, &iotexapi.BlockProducerInfo{
			Address:    bp.Address,
			Votes:      bp.Votes.String(),
			Active:     active,
			Production: blockProduction,
		})
	}

	return &iotexapi.GetEpochMetaResponse{
		EpochData:          epochData,
		TotalBlocks:        numBlks,
		BlockProducersInfo: blockProducersInfo,
	}, nil
}

// GetRawBlocks gets raw block data
func (api *Server) GetRawBlocks(
	ctx context.Context,
	in *iotexapi.GetRawBlocksRequest,
) (*iotexapi.GetRawBlocksResponse, error) {
	if in.Count == 0 || in.Count > api.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	tipHeight := api.bc.TipHeight()
	if in.StartHeight > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	var res []*iotexapi.BlockInfo
	for height := int(in.StartHeight); height <= int(tipHeight); height++ {
		if uint64(len(res)) >= in.Count {
			break
		}
		blk, err := api.dao.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		var receiptsPb []*iotextypes.Receipt
		if in.WithReceipts {
			receipts, err := api.dao.GetReceipts(uint64(height))
			if err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			for _, receipt := range receipts {
				receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
			}
		}
		var transactionLogs *iotextypes.TransactionLogs
		if in.WithTransactionLogs {
			if transactionLogs, err = api.dao.TransactionLogs(uint64(height)); err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
		}
		res = append(res, &iotexapi.BlockInfo{
			Block:           blk.ConvertToBlockPb(),
			Receipts:        receiptsPb,
			TransactionLogs: transactionLogs,
		})
	}

	return &iotexapi.GetRawBlocksResponse{Blocks: res}, nil
}

// GetLogs get logs filtered by contract address and topics
func (api *Server) GetLogs(
	ctx context.Context,
	in *iotexapi.GetLogsRequest,
) (*iotexapi.GetLogsResponse, error) {
	if in.GetFilter() == nil {
		return nil, status.Error(codes.InvalidArgument, "empty filter")
	}

	var (
		logs []*iotextypes.Log
		err  error
	)
	switch {
	case in.GetByBlock() != nil:
		req := in.GetByBlock()
		startBlock, err := api.dao.GetBlockHeight(hash.BytesToHash256(req.BlockHash))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid block hash")
		}
		logs, err = api.getLogsInBlock(logfilter.NewLogFilter(in.GetFilter(), nil, nil), startBlock)
	case in.GetByRange() != nil:
		req := in.GetByRange()
		if req.FromBlock > api.bc.TipHeight() {
			return nil, status.Error(codes.InvalidArgument, "start block > tip height")
		}
		if req.Count > 1000 {
			return nil, status.Error(codes.InvalidArgument, "maximum query range is 1000 blocks")
		}
		startBlock := req.FromBlock
		count := req.Count
		logs, err = api.getLogsInRange(logfilter.NewLogFilter(in.GetFilter(), nil, nil), startBlock, startBlock+count-1)
	case in.GetByLongRange() != nil:
		req := in.GetByLongRange()
		if req.FromBlock > api.bc.TipHeight() {
			return nil, status.Error(codes.InvalidArgument, "start block > tip height")
		}
		startBlock := req.GetFromBlock()
		endBlock := req.GetToBlock()
		logs, err = api.getLogsInRange(logfilter.NewLogFilter(in.GetFilter(), nil, nil), startBlock, endBlock)
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid GetLogsRequest type")
	}

	return &iotexapi.GetLogsResponse{Logs: logs}, err
}

// StreamBlocks streams blocks
func (api *Server) StreamBlocks(in *iotexapi.StreamBlocksRequest, stream iotexapi.APIService_StreamBlocksServer) error {
	errChan := make(chan error)
	if err := api.chainListener.AddResponder(NewBlockListener(stream, errChan)); err != nil {
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
func (api *Server) StreamLogs(in *iotexapi.StreamLogsRequest, stream iotexapi.APIService_StreamLogsServer) error {
	if in.GetFilter() == nil {
		return status.Error(codes.InvalidArgument, "empty filter")
	}
	errChan := make(chan error)
	// register the log filter so it will match logs in new blocks
	if err := api.chainListener.AddResponder(logfilter.NewLogFilter(in.GetFilter(), stream, errChan)); err != nil {
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
func (api *Server) GetElectionBuckets(
	ctx context.Context,
	in *iotexapi.GetElectionBucketsRequest,
) (*iotexapi.GetElectionBucketsResponse, error) {
	if api.electionCommittee == nil {
		return nil, status.Error(codes.Unavailable, "Native election no supported")
	}
	buckets, err := api.electionCommittee.NativeBucketsByEpoch(in.GetEpochNum())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	re := make([]*iotextypes.ElectionBucket, len(buckets))
	for i, b := range buckets {
		startTime, err := ptypes.TimestampProto(b.StartTime())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		re[i] = &iotextypes.ElectionBucket{
			Voter:     b.Voter(),
			Candidate: b.Candidate(),
			Amount:    b.Amount().Bytes(),
			StartTime: startTime,
			Duration:  ptypes.DurationProto(b.Duration()),
			Decay:     b.Decay(),
		}
	}
	return &iotexapi.GetElectionBucketsResponse{Buckets: re}, nil
}

// GetReceiptByActionHash returns receipt by action hash
func (api *Server) GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	if !api.hasActionIndex || api.indexer == nil {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	actIndex, err := api.indexer.GetActionIndex(h[:])
	if err != nil {
		return nil, err
	}
	return api.dao.GetReceiptByActionHash(h, actIndex.BlockHeight())
}

// GetActionByActionHash returns action by action hash
func (api *Server) GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error) {
	if !api.hasActionIndex || api.indexer == nil {
		return action.SealedEnvelope{}, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	selp, _, _, err := api.getActionByActionHash(h)
	return selp, err
}

// GetEvmTransfersByActionHash returns evm transfers by action hash
func (api *Server) GetEvmTransfersByActionHash(ctx context.Context, in *iotexapi.GetEvmTransfersByActionHashRequest) (*iotexapi.GetEvmTransfersByActionHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByActionHash instead")
}

// GetEvmTransfersByBlockHeight returns evm transfers by block height
func (api *Server) GetEvmTransfersByBlockHeight(ctx context.Context, in *iotexapi.GetEvmTransfersByBlockHeightRequest) (*iotexapi.GetEvmTransfersByBlockHeightResponse, error) {
	return nil, status.Error(codes.Unimplemented, "evm transfer index is deprecated, call GetSystemLogByBlockHeight instead")
}

// GetTransactionLogByActionHash returns transaction log by action hash
func (api *Server) GetTransactionLogByActionHash(
	ctx context.Context,
	in *iotexapi.GetTransactionLogByActionHashRequest) (*iotexapi.GetTransactionLogByActionHashResponse, error) {
	if !api.hasActionIndex || api.indexer == nil {
		return nil, status.Error(codes.Unimplemented, blockindex.ErrActionIndexNA.Error())
	}
	if !api.dao.ContainsTransactionLog() {
		return nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	h, err := hex.DecodeString(in.ActionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	actIndex, err := api.indexer.GetActionIndex(h)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	sysLog, err := api.dao.TransactionLogs(actIndex.BlockHeight())
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, log := range sysLog.Logs {
		if bytes.Compare(h, log.ActionHash) == 0 {
			return &iotexapi.GetTransactionLogByActionHashResponse{
				TransactionLog: log,
			}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "transaction log not found for action %s", in.ActionHash)
}

// GetTransactionLogByBlockHeight returns transaction log by block height
func (api *Server) GetTransactionLogByBlockHeight(
	ctx context.Context,
	in *iotexapi.GetTransactionLogByBlockHeightRequest) (*iotexapi.GetTransactionLogByBlockHeightResponse, error) {
	if !api.dao.ContainsTransactionLog() {
		return nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	tip, err := api.dao.Height()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if in.BlockHeight < 1 || in.BlockHeight > tip {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block height = %d", in.BlockHeight)
	}

	h, err := api.dao.GetBlockHash(in.BlockHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	res := &iotexapi.GetTransactionLogByBlockHeightResponse{
		BlockIdentifier: &iotextypes.BlockIdentifier{
			Hash:   hex.EncodeToString(h[:]),
			Height: in.BlockHeight,
		},
	}
	sysLog, err := api.dao.TransactionLogs(in.BlockHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			// should return empty, no transaction happened in block
			res.TransactionLogs = &iotextypes.TransactionLogs{}
			return res, nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	res.TransactionLogs = sysLog
	return res, nil
}

// Start starts the API server
func (api *Server) Start() error {
	portStr := ":" + strconv.Itoa(api.cfg.API.Port)
	lis, err := net.Listen("tcp", portStr)
	if err != nil {
		log.L().Error("API server failed to listen.", zap.Error(err))
		return errors.Wrap(err, "API server failed to listen")
	}
	log.L().Info("API server is listening.", zap.String("addr", lis.Addr().String()))

	go func() {
		if err := api.grpcServer.Serve(lis); err != nil {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	if err := api.bc.AddSubscriber(api.chainListener); err != nil {
		return errors.Wrap(err, "failed to subscribe to block creations")
	}
	if err := api.chainListener.Start(); err != nil {
		return errors.Wrap(err, "failed to start blockchain listener")
	}
	return nil
}

// Stop stops the API server
func (api *Server) Stop() error {
	api.grpcServer.Stop()
	if err := api.bc.RemoveSubscriber(api.chainListener); err != nil {
		return errors.Wrap(err, "failed to unsubscribe blockchain listener")
	}
	return api.chainListener.Stop()
}

func (api *Server) readState(ctx context.Context, p protocol.Protocol, height string, methodName []byte, arguments ...[]byte) ([]byte, uint64, error) {
	// TODO: need to complete the context
	tipHeight := api.bc.TipHeight()
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: tipHeight,
	})
	ctx = protocol.WithBlockchainCtx(
		protocol.WithRegistry(ctx, api.registry),
		protocol.BlockchainCtx{
			Genesis: api.cfg.Genesis,
		},
	)

	rp := rolldpos.FindProtocol(api.registry)
	if rp == nil {
		return nil, uint64(0), errors.New("rolldpos is not registered")
	}

	tipEpochNum := rp.GetEpochNum(tipHeight)
	if height != "" {
		inputHeight, err := strconv.ParseUint(height, 0, 64)
		if err != nil {
			return nil, uint64(0), err
		}
		inputEpochNum := rp.GetEpochNum(inputHeight)
		if inputEpochNum < tipEpochNum {
			// old data, wrap to history state reader
			return p.ReadState(ctx, factory.NewHistoryStateReader(api.sf, rp.GetEpochHeight(inputEpochNum)), methodName, arguments...)
		}
	}

	// TODO: need to distinguish user error and system error
	return p.ReadState(ctx, api.sf, methodName, arguments...)
}

func (api *Server) getActionsFromIndex(totalActions, start, count uint64) (*iotexapi.GetActionsResponse, error) {
	var actionInfo []*iotexapi.ActionInfo
	hashes, err := api.indexer.GetActionHashFromIndex(start, count)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	for i := range hashes {
		act, err := api.getAction(hash.BytesToHash256(hashes[i]), false)
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		actionInfo = append(actionInfo, act)
	}
	return &iotexapi.GetActionsResponse{
		Total:      totalActions,
		ActionInfo: actionInfo,
	}, nil
}

// GetActions returns actions within the range
func (api *Server) getActions(start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > api.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	totalActions, err := api.indexer.GetTotalActions()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if start >= totalActions {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}
	if totalActions == uint64(0) || count == 0 {
		return &iotexapi.GetActionsResponse{}, nil
	}
	if start+count > totalActions {
		count = totalActions - start
	}
	if api.hasActionIndex {
		return api.getActionsFromIndex(totalActions, start, count)
	}
	// Finding actions in reverse order saves time for querying most recent actions
	reverseStart := totalActions - (start + count)
	if totalActions < start+count {
		reverseStart = uint64(0)
		count = totalActions - start
	}

	var res []*iotexapi.ActionInfo
	var hit bool
	for height := api.bc.TipHeight(); height >= 1 && count > 0; height-- {
		blk, err := api.dao.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if !hit && reverseStart >= uint64(len(blk.Actions)) {
			reverseStart -= uint64(len(blk.Actions))
			continue
		}
		// now reverseStart < len(blk.Actions), we are going to fetch actions from this block
		hit = true
		act := api.reverseActionsInBlock(blk, reverseStart, count)
		res = append(act, res...)
		count -= uint64(len(act))
		reverseStart = 0
	}
	return &iotexapi.GetActionsResponse{
		Total:      totalActions,
		ActionInfo: res,
	}, nil
}

// getSingleAction returns action by action hash
func (api *Server) getSingleAction(actionHash string, checkPending bool) (*iotexapi.GetActionsResponse, error) {
	actHash, err := hash.HexStringToHash256(actionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	act, err := api.getAction(actHash, checkPending)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &iotexapi.GetActionsResponse{
		Total:      1,
		ActionInfo: []*iotexapi.ActionInfo{act},
	}, nil
}

// getActionsByAddress returns all actions associated with an address
func (api *Server) getActionsByAddress(addrStr string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > api.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, err
	}
	actions, err := api.indexer.GetActionsByAddress(hash.BytesToHash160(addr.Bytes()), start, count)
	if err != nil && (errors.Cause(err) == db.ErrBucketNotExist || errors.Cause(err) == db.ErrNotExist) {
		// no actions associated with address, return nil
		return &iotexapi.GetActionsResponse{}, nil
	}
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	res := &iotexapi.GetActionsResponse{Total: uint64(len(actions))}
	for i := range actions {
		act, err := api.getAction(hash.BytesToHash256(actions[i]), false)
		if err != nil {
			continue
		}
		res.ActionInfo = append(res.ActionInfo, act)
	}
	return res, nil
}

// getBlockHashByActionHash returns block hash by action hash
func (api *Server) getBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	actIndex, err := api.indexer.GetActionIndex(h[:])
	if err != nil {
		return hash.ZeroHash256, err
	}
	return api.dao.GetBlockHash(actIndex.BlockHeight())
}

// getActionByActionHash returns action by action hash
func (api *Server) getActionByActionHash(h hash.Hash256) (action.SealedEnvelope, hash.Hash256, uint64, error) {
	actIndex, err := api.indexer.GetActionIndex(h[:])
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, err
	}

	blk, err := api.dao.GetBlockByHeight(actIndex.BlockHeight())
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, err
	}

	selp, err := api.dao.GetActionByActionHash(h, actIndex.BlockHeight())
	return selp, blk.HashBlock(), actIndex.BlockHeight(), err
}

// getUnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (api *Server) getUnconfirmedActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > api.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	selps := api.ap.GetUnconfirmedActs(address)
	if len(selps) == 0 {
		return &iotexapi.GetActionsResponse{}, nil
	}
	if start >= uint64(len(selps)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	var res []*iotexapi.ActionInfo
	for i := start; i < uint64(len(selps)) && i < start+count; i++ {
		act, err := api.pendingAction(selps[i])
		if err != nil {
			continue
		}
		res = append(res, act)
	}
	return &iotexapi.GetActionsResponse{
		Total:      uint64(len(selps)),
		ActionInfo: res,
	}, nil
}

// getActionsByBlock returns all actions in a block
func (api *Server) getActionsByBlock(blkHash string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > api.cfg.API.RangeQueryLimit && count != math.MaxUint64 {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blk, err := api.dao.GetBlock(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if start >= uint64(len(blk.Actions)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	return &iotexapi.GetActionsResponse{
		Total:      uint64(len(blk.Actions)),
		ActionInfo: api.actionsInBlock(blk, start, count),
	}, nil
}

// getBlockMetas returns blockmetas response within the height range
func (api *Server) getBlockMetas(start uint64, count uint64) (*iotexapi.GetBlockMetasResponse, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > api.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	tipHeight := api.bc.TipHeight()
	if start > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	var res []*iotextypes.BlockMeta
	for height := start; height <= tipHeight && count > 0; height++ {
		blockMeta, err := api.getBlockMetaByHeight(height)
		if err != nil {
			return nil, err
		}
		res = append(res, blockMeta)
		count--
	}
	return &iotexapi.GetBlockMetasResponse{
		Total:    tipHeight,
		BlkMetas: res,
	}, nil
}

// getBlockMeta returns blockmetas response by block hash
func (api *Server) getBlockMeta(blkHash string) (*iotexapi.GetBlockMetasResponse, error) {
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blockMeta, err := api.getBlockMetaByHash(hash)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetBlockMetasResponse{
		Total:    1,
		BlkMetas: []*iotextypes.BlockMeta{blockMeta},
	}, nil
}

// getBlockMetaByHeight gets block meta by height
func (api *Server) getBlockMetaByHeight(height uint64) (*iotextypes.BlockMeta, error) {
	if api.indexer != nil {
		blockMeta, err := api.getBlockMetasByHeader(height)
		if errors.Cause(err) != db.ErrNotExist {
			return blockMeta, err
		}
	}
	return api.getBlockMetasByBlock(height)
}

// getBlockMetaByHash gets block meta by hash
func (api *Server) getBlockMetaByHash(h hash.Hash256) (*iotextypes.BlockMeta, error) {
	if api.indexer != nil {
		blockMeta, err := api.getBlockMetaByHeader(h)
		if errors.Cause(err) != db.ErrNotExist {
			return blockMeta, err
		}
	}
	return api.getBlockMetaByBlock(h)
}

// putBlockMetaUpgradeByBlock puts numActions and transferAmount for blockmeta by block
func (api *Server) putBlockMetaUpgradeByBlock(blk *block.Block, blockMeta *iotextypes.BlockMeta) *iotextypes.BlockMeta {
	blockMeta.NumActions = int64(len(blk.Actions))
	blockMeta.TransferAmount = blk.CalculateTransferAmount().String()
	return blockMeta
}

// putBlockMetaUpgradeByHeader puts numActions and transferAmount for blockmeta by header height
func (api *Server) putBlockMetaUpgradeByHeader(height uint64, blockMeta *iotextypes.BlockMeta) (*iotextypes.BlockMeta, error) {
	index, err := api.indexer.GetBlockIndex(height)
	if err != nil {
		return nil, errors.Wrapf(err, "missing block index at height %d", height)
	}
	blockMeta.NumActions = int64(index.NumAction())
	blockMeta.TransferAmount = index.TsfAmount().String()
	return blockMeta, nil
}

// getBlockMetasByHeader gets block header by height
func (api *Server) getBlockMetasByHeader(height uint64) (*iotextypes.BlockMeta, error) {
	header, err := api.bc.BlockHeaderByHeight(height)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blockMeta := api.getCommonBlockMeta(header)
	blockMeta, err = api.putBlockMetaUpgradeByHeader(header.Height(), blockMeta)
	if err != nil {
		return nil, err
	}
	return blockMeta, nil
}

// getBlockMetasByBlock gets block by height
func (api *Server) getBlockMetasByBlock(height uint64) (*iotextypes.BlockMeta, error) {
	blk, err := api.dao.GetBlockByHeight(height)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blockMeta := api.getCommonBlockMeta(blk)
	blockMeta = api.putBlockMetaUpgradeByBlock(blk, blockMeta)
	return blockMeta, nil
}

// getBlockMetaByHeader gets block header by hash
func (api *Server) getBlockMetaByHeader(h hash.Hash256) (*iotextypes.BlockMeta, error) {
	header, err := api.dao.Header(h)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blockMeta := api.getCommonBlockMeta(header)
	blockMeta, err = api.putBlockMetaUpgradeByHeader(header.Height(), blockMeta)
	if err != nil {
		return nil, err
	}
	return blockMeta, nil
}

// getBlockMetaByBlock gets block by hash
func (api *Server) getBlockMetaByBlock(h hash.Hash256) (*iotextypes.BlockMeta, error) {
	blk, err := api.dao.GetBlock(h)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blockMeta := api.getCommonBlockMeta(blk)
	blockMeta = api.putBlockMetaUpgradeByBlock(blk, blockMeta)
	return blockMeta, nil
}

// getCommonBlockMeta gets blockmeta by empty interface
func (api *Server) getCommonBlockMeta(common interface{}) *iotextypes.BlockMeta {
	header, ok := common.(*block.Header)
	if !ok {
		blk := common.(*block.Block)
		header = &blk.Header
	}
	hash := header.HashBlock()
	height := header.Height()
	ts, _ := ptypes.TimestampProto(header.Timestamp())
	producerAddress := header.ProducerAddress()
	txRoot := header.TxRoot()
	receiptRoot := header.ReceiptRoot()
	deltaStateDigest := header.DeltaStateDigest()
	logsBloom := header.LogsBloomfilter()
	prevHash := header.PrevHash()

	blockMeta := &iotextypes.BlockMeta{
		Hash:              hex.EncodeToString(hash[:]),
		Height:            height,
		Timestamp:         ts,
		ProducerAddress:   producerAddress,
		TxRoot:            hex.EncodeToString(txRoot[:]),
		ReceiptRoot:       hex.EncodeToString(receiptRoot[:]),
		DeltaStateDigest:  hex.EncodeToString(deltaStateDigest[:]),
		PreviousBlockHash: hex.EncodeToString(prevHash[:]),
	}
	if logsBloom != nil {
		blockMeta.LogsBloom = hex.EncodeToString(logsBloom.Bytes())
	}
	return blockMeta
}

func (api *Server) getGravityChainStartHeight(epochHeight uint64) (uint64, error) {
	gravityChainStartHeight := epochHeight
	if pp := poll.FindProtocol(api.registry); pp != nil {
		methodName := []byte("GetGravityChainStartHeight")
		arguments := [][]byte{[]byte(strconv.FormatUint(epochHeight, 10))}
		data, _, err := api.readState(context.Background(), pp, "", methodName, arguments...)
		if err != nil {
			return 0, err
		}
		if len(data) == 0 {
			return 0, nil
		}
		if gravityChainStartHeight, err = strconv.ParseUint(string(data), 10, 64); err != nil {
			return 0, err
		}
	}
	return gravityChainStartHeight, nil
}

func (api *Server) committedAction(selp action.SealedEnvelope, blkHash hash.Hash256, blkHeight uint64) (
	*iotexapi.ActionInfo, error) {
	actHash := selp.Hash()
	header, err := api.dao.Header(blkHash)
	if err != nil {
		return nil, err
	}
	sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
	receipt, err := api.dao.GetReceiptByActionHash(actHash, blkHeight)
	if err != nil {
		return nil, err
	}
	gas := new(big.Int)
	gas = gas.Mul(selp.GasPrice(), big.NewInt(int64(receipt.GasConsumed)))
	return &iotexapi.ActionInfo{
		Action:    selp.Proto(),
		ActHash:   hex.EncodeToString(actHash[:]),
		BlkHash:   hex.EncodeToString(blkHash[:]),
		BlkHeight: header.Height(),
		Sender:    sender.String(),
		GasFee:    gas.String(),
		Timestamp: header.BlockHeaderCoreProto().Timestamp,
	}, nil
}

func (api *Server) pendingAction(selp action.SealedEnvelope) (*iotexapi.ActionInfo, error) {
	actHash := selp.Hash()
	sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
	return &iotexapi.ActionInfo{
		Action:    selp.Proto(),
		ActHash:   hex.EncodeToString(actHash[:]),
		BlkHash:   hex.EncodeToString(hash.ZeroHash256[:]),
		BlkHeight: 0,
		Sender:    sender.String(),
		Timestamp: nil,
	}, nil
}

func (api *Server) getAction(actHash hash.Hash256, checkPending bool) (*iotexapi.ActionInfo, error) {
	selp, blkHash, blkHeight, err := api.getActionByActionHash(actHash)
	if err == nil {
		return api.committedAction(selp, blkHash, blkHeight)
	}
	// Try to fetch pending action from actpool
	if checkPending {
		selp, err = api.ap.GetActionByHash(actHash)
	}
	if err != nil {
		return nil, err
	}
	return api.pendingAction(selp)
}

func (api *Server) actionsInBlock(blk *block.Block, start, count uint64) []*iotexapi.ActionInfo {
	var res []*iotexapi.ActionInfo
	if len(blk.Actions) == 0 || start >= uint64(len(blk.Actions)) {
		return res
	}

	h := blk.HashBlock()
	blkHash := hex.EncodeToString(h[:])
	blkHeight := blk.Height()
	ts := blk.Header.BlockHeaderCoreProto().Timestamp

	lastAction := start + count
	if count == math.MaxUint64 {
		// count = -1 means to get all actions
		lastAction = uint64(len(blk.Actions))
	} else {
		if lastAction >= uint64(len(blk.Actions)) {
			lastAction = uint64(len(blk.Actions))
		}
	}
	for i := start; i < lastAction; i++ {
		selp := blk.Actions[i]
		actHash := selp.Hash()
		sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
		res = append(res, &iotexapi.ActionInfo{
			Action:    selp.Proto(),
			ActHash:   hex.EncodeToString(actHash[:]),
			BlkHash:   blkHash,
			BlkHeight: blkHeight,
			Sender:    sender.String(),
			Timestamp: ts,
		})
	}
	return res
}

func (api *Server) reverseActionsInBlock(blk *block.Block, reverseStart, count uint64) []*iotexapi.ActionInfo {
	h := blk.HashBlock()
	blkHash := hex.EncodeToString(h[:])
	blkHeight := blk.Height()
	ts := blk.Header.BlockHeaderCoreProto().Timestamp

	var res []*iotexapi.ActionInfo
	for i := reverseStart; i < uint64(len(blk.Actions)) && i < reverseStart+count; i++ {
		ri := uint64(len(blk.Actions)) - 1 - i
		selp := blk.Actions[ri]
		actHash := selp.Hash()
		sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
		res = append([]*iotexapi.ActionInfo{
			{
				Action:    selp.Proto(),
				ActHash:   hex.EncodeToString(actHash[:]),
				BlkHash:   blkHash,
				BlkHeight: blkHeight,
				Sender:    sender.String(),
				Timestamp: ts,
			},
		}, res...)
	}
	return res
}

func (api *Server) getUnindexedLogsInRange(filter *logfilter.LogFilter, start, count uint64) ([]*iotextypes.Log, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	// filter logs within start --> end
	var logs []*iotextypes.Log
	end := start + count - 1
	if end > api.bc.TipHeight() {
		end = api.bc.TipHeight()
	}
	for i := start; i <= end; i++ {
		logsInBlock, err := api.getLogsInBlock(filter, i)
		if err != nil {
			return logs, err
		}
		if logsInBlock != nil {
			logs = append(logs, logsInBlock...)
		}
	}
	return logs, nil
}

func (api *Server) getLogsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error) {
	logBloomFilter, err := api.bfIndexer.BloomFilterByHeight(blockNumber)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !filter.ExistInBloomFilterv2(logBloomFilter) {
		return nil, nil
	}
	receipts, err := api.dao.GetReceipts(blockNumber)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return filter.MatchLogs(receipts), nil
}

// TODO: improve using goroutine
func (api *Server) getLogsInRange(filter *logfilter.LogFilter, start, end uint64) ([]*iotextypes.Log, error) {
	if end > api.bc.TipHeight() || end == 0 {
		end = api.bc.TipHeight()
	}

	size := api.bfIndexer.RangeBloomFilterSize()
	logs := []*iotextypes.Log{}

	// getLogs for unindexed height in range one by one
	if start%size != 0 {
		unindexedEnd := size * (start/size + 1) //round up
		if end < unindexedEnd {
			unindexedEnd = end
		}
		unindexed, err := api.getUnindexedLogsInRange(filter, start, unindexedEnd-start+1)
		if err != nil {
			return nil, err
		}
		logs = append(logs, unindexed...)
		if len(logs) > int(api.cfg.API.RangeQueryLimit) {
			return nil, status.Error(codes.InvalidArgument, "range results exceed the limit")
		}
		start = unindexedEnd
	}
	if start == end {
		return logs, nil
	}

	// getLogs via range Blooom filter
	blockNumbers, err := api.bfIndexer.FilterBlocksInRange(filter, start, end)
	if err != nil {
		return nil, err
	}
	for _, i := range blockNumbers {
		indexed, err := api.getLogsInBlock(filter, i)
		if err != nil {
			return nil, err
		}
		if indexed != nil {
			logs = append(logs, indexed...)
		}
		if len(logs) > int(api.cfg.API.RangeQueryLimit) {
			return nil, status.Error(codes.InvalidArgument, "range results exceed the limit")
		}
	}

	return logs, nil
}

// TODO: Since GasConsumed on the receipt may not be enough for the gas limit, we use binary search for the gas estimate. Need a better way to address it later.
func (api *Server) estimateActionGasConsumptionForExecution(exec *iotextypes.Execution, sender string) (*iotexapi.EstimateActionGasConsumptionResponse, error) {
	sc := &action.Execution{}
	if err := sc.LoadProto(exec); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	state, err := accountutil.AccountState(api.sf, sender)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	nonce := state.Nonce + 1

	callerAddr, err := address.FromString(sender)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sc, _ = action.NewExecution(
		sc.Contract(),
		nonce,
		sc.Amount(),
		api.cfg.Genesis.BlockGasLimit,
		big.NewInt(0),
		sc.Data(),
	)

	ctx, err := api.bc.Context()
	if err != nil {
		return nil, err
	}
	_, receipt, err := api.sf.SimulateExecution(ctx, callerAddr, sc, api.dao.GetBlockHash)

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
		return nil, status.Error(codes.Internal, "execution simulation gets failure status")
	}
	estimatedGas := receipt.GasConsumed
	enough, err := api.isGasLimitEnough(callerAddr, sc, nonce, estimatedGas)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !enough {
		low, high := estimatedGas, api.cfg.Genesis.BlockGasLimit
		estimatedGas = high
		for low <= high {
			mid := (low + high) / 2
			enough, err = api.isGasLimitEnough(callerAddr, sc, nonce, mid)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if enough {
				estimatedGas = mid
				break
			}
			low = mid + 1
		}
	}

	return &iotexapi.EstimateActionGasConsumptionResponse{
		Gas: estimatedGas,
	}, nil
}

func (api *Server) estimateActionGasConsumptionForTransfer(transfer *iotextypes.Transfer) (*iotexapi.EstimateActionGasConsumptionResponse, error) {
	payloadSize := uint64(len(transfer.Payload))
	return &iotexapi.EstimateActionGasConsumptionResponse{
		Gas: payloadSize*action.TransferPayloadGas + action.TransferBaseIntrinsicGas,
	}, nil
}

func (api *Server) isGasLimitEnough(
	caller address.Address,
	sc *action.Execution,
	nonce uint64,
	gasLimit uint64,
) (bool, error) {
	sc, _ = action.NewExecution(
		sc.Contract(),
		nonce,
		sc.Amount(),
		gasLimit,
		big.NewInt(0),
		sc.Data(),
	)
	ctx, err := api.bc.Context()
	if err != nil {
		return false, err
	}
	_, receipt, err := api.sf.SimulateExecution(ctx, caller, sc, api.dao.GetBlockHash)
	if err != nil {
		return false, err
	}
	return receipt.Status == uint64(iotextypes.ReceiptStatus_Success), nil
}

func (api *Server) getProductivityByEpoch(
	rp *rolldpos.Protocol,
	epochNum uint64,
	tipHeight uint64,
	abps state.CandidateList,
) (uint64, map[string]uint64, error) {
	num, produce, err := rp.ProductivityByEpoch(epochNum, tipHeight, func(start uint64, end uint64) (map[string]uint64, error) {
		return blockchain.Productivity(api.bc, start, end)
	})
	if err != nil {
		return 0, nil, status.Error(codes.NotFound, err.Error())
	}
	// check if there is any active block producer who didn't prodcue any block
	for _, abp := range abps {
		if _, ok := produce[abp.Address]; !ok {
			produce[abp.Address] = 0
		}
	}
	return num, produce, nil
}

func (api *Server) getProtocolAccount(ctx context.Context, addr string) (ret *iotexapi.GetAccountResponse, err error) {
	var req *iotexapi.ReadStateRequest
	var balance string
	var out *iotexapi.ReadStateResponse
	switch addr {
	case address.RewardingPoolAddr:
		req = &iotexapi.ReadStateRequest{
			ProtocolID: []byte("rewarding"),
			MethodName: []byte("TotalBalance"),
		}
		out, err = api.ReadState(ctx, req)
		if err != nil {
			return
		}
		val, ok := big.NewInt(0).SetString(string(out.GetData()), 10)
		if !ok {
			err = errors.New("balance convert error")
			return
		}
		balance = val.String()
	case address.StakingBucketPoolAddr:
		methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
			Method: iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT,
		})
		if err != nil {
			return nil, err
		}
		arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
			Request: &iotexapi.ReadStakingDataRequest_TotalStakingAmount_{
				TotalStakingAmount: &iotexapi.ReadStakingDataRequest_TotalStakingAmount{},
			},
		})
		if err != nil {
			return nil, err
		}
		req = &iotexapi.ReadStateRequest{
			ProtocolID: []byte("staking"),
			MethodName: methodName,
			Arguments:  [][]byte{arg},
		}
		out, err = api.ReadState(ctx, req)
		if err != nil {
			return nil, err
		}
		acc := iotextypes.AccountMeta{}
		if err := proto.Unmarshal(out.GetData(), &acc); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal account meta")
		}
		balance = acc.GetBalance()
	}

	ret = &iotexapi.GetAccountResponse{
		AccountMeta: &iotextypes.AccountMeta{
			Address: addr,
			Balance: balance,
		},
		BlockIdentifier: out.GetBlockIdentifier(),
	}
	return
}

// GetActPoolActions returns the all Transaction Identifiers in the mempool
func (api *Server) GetActPoolActions(ctx context.Context, in *iotexapi.GetActPoolActionsRequest) (*iotexapi.GetActPoolActionsResponse, error) {
	ret := new(iotexapi.GetActPoolActionsResponse)

	if len(in.ActionHashes) < 1 {
		for _, sealeds := range api.ap.PendingActionMap() {
			for _, sealed := range sealeds {
				ret.Actions = append(ret.Actions, sealed.Proto())
			}
		}
		return ret, nil
	}

	for _, hashStr := range in.ActionHashes {
		hs, err := hash.HexStringToHash256(hashStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, errors.Wrap(err, "failed to hex string to hash256").Error())
		}
		sealed, err := api.ap.GetActionByHash(hs)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		ret.Actions = append(ret.Actions, sealed.Proto())
	}

	return ret, nil
}