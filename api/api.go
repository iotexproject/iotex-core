// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"net"
	"sort"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state"
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
	broadcastHandler BroadcastOutbound
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

// Server provides api for user to query blockchain data
type Server struct {
	bc               blockchain.Blockchain
	dp               dispatcher.Dispatcher
	ap               actpool.ActPool
	gs               *gasstation.GasStation
	broadcastHandler BroadcastOutbound
	cfg              config.API
	registry         *protocol.Registry
	blockListener    *blockListener
	grpcserver       *grpc.Server
	hasActionIndex   bool
}

// NewServer creates a new server
func NewServer(
	cfg config.Config,
	chain blockchain.Blockchain,
	dispatcher dispatcher.Dispatcher,
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
		bc:               chain,
		dp:               dispatcher,
		ap:               actPool,
		broadcastHandler: apiCfg.broadcastHandler,
		cfg:              cfg.API,
		registry:         registry,
		blockListener:    newBlockListener(),
		gs:               gasstation.NewGasStation(chain, cfg.API, cfg.Genesis.ActionGasLimit),
	}
	if _, ok := cfg.Plugins[config.GatewayPlugin]; ok {
		svr.hasActionIndex = true
	}
	svr.grpcserver = grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	iotexapi.RegisterAPIServiceServer(svr.grpcserver, svr)
	grpc_prometheus.Register(svr.grpcserver)
	reflection.Register(svr.grpcserver)

	return svr, nil
}

// GetAccount returns the metadata of an account
func (api *Server) GetAccount(ctx context.Context, in *iotexapi.GetAccountRequest) (*iotexapi.GetAccountResponse, error) {
	state, err := api.bc.StateByAddr(in.Address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	pendingNonce, err := api.ap.GetPendingNonce(in.Address)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	numActions, err := api.bc.GetActionCountByAddress(in.Address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	accountMeta := &iotextypes.AccountMeta{
		Address:      in.Address,
		Balance:      state.Balance.String(),
		Nonce:        state.Nonce,
		PendingNonce: pendingNonce,
		NumActions:   numActions,
	}
	return &iotexapi.GetAccountResponse{AccountMeta: accountMeta}, nil
}

// GetActions returns actions
func (api *Server) GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error) {
	if !api.hasActionIndex && in.GetByBlk() == nil {
		return nil, status.Error(codes.NotFound, "Action index is not available.")
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
	totalActions, err := api.bc.GetTotalActions()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	blockLimit := int64(api.cfg.TpsWindow)
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

	p, ok := api.registry.Find(rolldpos.ProtocolID)
	if !ok {
		return nil, status.Error(codes.Internal, "rolldpos protocol is not registered")
	}
	rp, ok := p.(*rolldpos.Protocol)
	if !ok {
		return nil, status.Error(codes.Internal, "fail to cast rolldpos protocol")
	}
	epochNum := rp.GetEpochNum(tipHeight)
	epochHeight := rp.GetEpochHeight(epochNum)
	gravityChainStartHeight, err := api.getGravityChainStartHeight(epochHeight)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	timeDuration := blks[len(blks)-1].Timestamp.GetSeconds() - blks[0].Timestamp.GetSeconds()
	// if time duration is less than 1 second, we set it to be 1 second
	if timeDuration < 1 {
		timeDuration = 1
	}

	tps := numActions / timeDuration

	chainMeta := &iotextypes.ChainMeta{
		Height: tipHeight,
		Epoch: &iotextypes.EpochData{
			Num:                     epochNum,
			Height:                  epochHeight,
			GravityChainStartHeight: gravityChainStartHeight,
		},
		NumActions: int64(totalActions),
		Tps:        tps,
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
func (api *Server) SendAction(ctx context.Context, in *iotexapi.SendActionRequest) (res *iotexapi.SendActionResponse, err error) {
	log.L().Debug("receive send action request")

	// broadcast to the network
	if err = api.broadcastHandler(context.Background(), api.bc.ChainID(), in.Action); err != nil {
		log.L().Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	// send to actpool via dispatcher
	api.dp.HandleBroadcast(context.Background(), api.bc.ChainID(), in.Action)

	var selp action.SealedEnvelope
	if err = selp.LoadProto(in.Action); err != nil {
		return
	}
	hash := selp.Hash()

	return &iotexapi.SendActionResponse{ActionHash: hex.EncodeToString(hash[:])}, nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (api *Server) GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error) {
	if !api.hasActionIndex {
		return nil, status.Error(codes.NotFound, "Receipt index is not available")
	}
	actHash, err := hash.HexStringToHash256(in.ActionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	receipt, err := api.bc.GetReceiptByActionHash(actHash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := api.bc.GetBlockHashByActionHash(actHash)
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

	caller, err := api.bc.StateByAddr(in.CallerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sc, _ = action.NewExecution(
		sc.Contract(),
		caller.Nonce+1,
		sc.Amount(),
		api.gs.ActionGasLimit(),
		big.NewInt(0),
		sc.Data(),
	)

	callerAddr, err := address.FromString(in.CallerAddress)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	retval, receipt, err := api.bc.ExecuteContractRead(callerAddr, sc)
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
	res, err := api.readState(ctx, in)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return res, nil
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

// GetEpochMeta gets epoch metadata
func (api *Server) GetEpochMeta(
	ctx context.Context,
	in *iotexapi.GetEpochMetaRequest,
) (*iotexapi.GetEpochMetaResponse, error) {
	if in.EpochNumber < 1 {
		return nil, status.Error(codes.InvalidArgument, "epoch number cannot be less than one")
	}
	p, ok := api.registry.Find(rolldpos.ProtocolID)
	if !ok {
		return nil, status.Error(codes.Internal, "rolldpos protocol is not registered")
	}
	rp, ok := p.(*rolldpos.Protocol)
	if !ok {
		return nil, status.Error(codes.Internal, "fail to cast rolldpos protocol")
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

	numBlks, produce, err := api.bc.ProductivityByEpoch(in.EpochNumber)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	readStateRequest := &iotexapi.ReadStateRequest{
		ProtocolID: []byte(poll.ProtocolID),
		MethodName: []byte("BlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(in.EpochNumber)},
	}
	res, err := api.readState(context.Background(), readStateRequest)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	var BlockProducers state.CandidateList
	if err := BlockProducers.Deserialize(res.Data); err != nil {
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
	if in.Count == 0 || in.Count > api.cfg.RangeQueryLimit {
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
		blk, err := api.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		var receiptsPb []*iotextypes.Receipt
		if in.WithReceipts {
			receipts, err := api.bc.GetReceiptsByHeight(uint64(height))
			if err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			for _, receipt := range receipts {
				receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
			}
		}
		res = append(res, &iotexapi.BlockInfo{
			Block:    blk.ConvertToBlockPb(),
			Receipts: receiptsPb,
		})
	}

	return &iotexapi.GetRawBlocksResponse{Blocks: res}, nil
}

// StreamBlocks streams blocks
func (api *Server) StreamBlocks(in *iotexapi.StreamBlocksRequest, stream iotexapi.APIService_StreamBlocksServer) error {
	errChan := make(chan error)
	if err := api.blockListener.AddStream(stream, errChan); err != nil {
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

// Start starts the API server
func (api *Server) Start() error {
	portStr := ":" + strconv.Itoa(api.cfg.Port)
	lis, err := net.Listen("tcp", portStr)
	if err != nil {
		log.L().Error("API server failed to listen.", zap.Error(err))
		return errors.Wrap(err, "API server failed to listen")
	}
	log.L().Info("API server is listening.", zap.String("addr", lis.Addr().String()))

	go func() {
		if err := api.grpcserver.Serve(lis); err != nil {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	if err := api.bc.AddSubscriber(api.blockListener); err != nil {
		return errors.Wrap(err, "failed to subscribe to block creations")
	}
	if err := api.blockListener.Start(); err != nil {
		return errors.Wrap(err, "failed to start block listener")
	}
	return nil
}

// Stop stops the API server
func (api *Server) Stop() error {
	api.grpcserver.Stop()
	if err := api.bc.RemoveSubscriber(api.blockListener); err != nil {
		return errors.Wrap(err, "failed to unsubscribe block listener")
	}
	return api.blockListener.Stop()
}

func (api *Server) readState(ctx context.Context, in *iotexapi.ReadStateRequest) (*iotexapi.ReadStateResponse, error) {
	p, ok := api.registry.Find(string(in.ProtocolID))
	if !ok {
		return nil, status.Errorf(codes.Internal, "protocol %s isn't registered", string(in.ProtocolID))
	}
	// TODO: need to complete the context
	ctx = protocol.WithRunActionsCtx(ctx, protocol.RunActionsCtx{
		BlockHeight: api.bc.TipHeight(),
		Registry:    api.registry,
	})
	ws, err := api.bc.GetFactory().NewWorkingSet()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	data, err := p.ReadState(ctx, ws, in.MethodName, in.Arguments...)
	// TODO: need to distinguish user error and system error
	if err != nil {
		return nil, err
	}
	out := iotexapi.ReadStateResponse{
		Data: data,
	}
	return &out, nil
}

// GetActions returns actions within the range
// This is a workaround for the slow access issue if the start index is very big
func (api *Server) getActions(start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count > api.cfg.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	totalActions, err := api.bc.GetTotalActions()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if totalActions == uint64(0) {
		return &iotexapi.GetActionsResponse{}, nil
	}
	if start >= totalActions {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
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
		blk, err := api.bc.GetBlockByHeight(height)
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
func (api *Server) getActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count > api.cfg.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	actions, err := api.getTotalActionsByAddress(address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if len(actions) == 0 {
		return &iotexapi.GetActionsResponse{}, nil
	}
	if start >= uint64(len(actions)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	var res []*iotexapi.ActionInfo
	for i := start; i < uint64(len(actions)) && i < start+count; i++ {
		act, err := api.getAction(actions[i], false)
		if err != nil {
			continue
		}
		res = append(res, act)
	}
	// sort action by timestamp
	sort.Slice(res, func(i, j int) bool {
		return res[i].Timestamp.Seconds < res[j].Timestamp.Seconds
	})
	return &iotexapi.GetActionsResponse{
		Total:      uint64(len(actions)),
		ActionInfo: res,
	}, nil
}

// getUnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (api *Server) getUnconfirmedActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	if count > api.cfg.RangeQueryLimit {
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
	if count > api.cfg.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if len(blk.Actions) == 0 {
		return &iotexapi.GetActionsResponse{}, nil
	}
	if start >= uint64(len(blk.Actions)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	res := api.actionsInBlock(blk, start, count)
	return &iotexapi.GetActionsResponse{
		Total:      uint64(len(blk.Actions)),
		ActionInfo: res,
	}, nil
}

// getBlockMetas gets block within the height range
func (api *Server) getBlockMetas(start uint64, count uint64) (*iotexapi.GetBlockMetasResponse, error) {
	if count > api.cfg.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	tipHeight := api.bc.TipHeight()
	if start > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	var res []*iotextypes.BlockMeta
	for height := start; height <= tipHeight && count > 0; height++ {
		blk, err := api.bc.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		hash := blk.HashBlock()
		ts, _ := ptypes.TimestampProto(blk.Header.Timestamp())
		txRoot := blk.TxRoot()
		receiptRoot := blk.ReceiptRoot()
		deltaStateDigest := blk.DeltaStateDigest()
		transferAmount := getTranferAmountInBlock(blk)

		blockMeta := &iotextypes.BlockMeta{
			Hash:             hex.EncodeToString(hash[:]),
			Height:           blk.Height(),
			Timestamp:        ts,
			NumActions:       int64(len(blk.Actions)),
			ProducerAddress:  blk.ProducerAddress(),
			TransferAmount:   transferAmount.String(),
			TxRoot:           hex.EncodeToString(txRoot[:]),
			ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
			DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
		}
		res = append(res, blockMeta)
		count--
	}
	return &iotexapi.GetBlockMetasResponse{
		Total:    tipHeight,
		BlkMetas: res,
	}, nil
}

// getBlockMeta returns block by block hash
func (api *Server) getBlockMeta(blkHash string) (*iotexapi.GetBlockMetasResponse, error) {
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	ts, _ := ptypes.TimestampProto(blk.Header.Timestamp())
	txRoot := blk.TxRoot()
	receiptRoot := blk.ReceiptRoot()
	deltaStateDigest := blk.DeltaStateDigest()
	transferAmount := getTranferAmountInBlock(blk)

	blockMeta := &iotextypes.BlockMeta{
		Hash:             blkHash,
		Height:           blk.Height(),
		Timestamp:        ts,
		NumActions:       int64(len(blk.Actions)),
		ProducerAddress:  blk.ProducerAddress(),
		TransferAmount:   transferAmount.String(),
		TxRoot:           hex.EncodeToString(txRoot[:]),
		ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
		DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
	}
	return &iotexapi.GetBlockMetasResponse{
		Total:    1,
		BlkMetas: []*iotextypes.BlockMeta{blockMeta},
	}, nil
}

func (api *Server) getGravityChainStartHeight(epochHeight uint64) (uint64, error) {
	gravityChainStartHeight := epochHeight
	if _, ok := api.registry.Find(poll.ProtocolID); ok {
		readStateRequest := &iotexapi.ReadStateRequest{
			ProtocolID: []byte(poll.ProtocolID),
			MethodName: []byte("GetGravityChainStartHeight"),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(epochHeight)},
		}
		res, err := api.readState(context.Background(), readStateRequest)
		if err != nil {
			return 0, err
		}
		gravityChainStartHeight = byteutil.BytesToUint64(res.GetData())
	}
	return gravityChainStartHeight, nil
}

func (api *Server) committedAction(selp action.SealedEnvelope) (*iotexapi.ActionInfo, error) {
	actHash := selp.Hash()
	blkHash, err := api.bc.GetBlockHashByActionHash(actHash)
	if err != nil {
		return nil, err
	}
	header, err := api.bc.BlockHeaderByHash(blkHash)
	if err != nil {
		return nil, err
	}
	sender, _ := address.FromBytes(selp.SrcPubkey().Hash())
	receipt, err := api.bc.GetReceiptByActionHash(actHash)
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
	selp, err := api.bc.GetActionByActionHash(actHash)
	if err == nil {
		return api.committedAction(selp)
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

func (api *Server) getTotalActionsByAddress(address string) ([]hash.Hash256, error) {
	actions, err := api.bc.GetActionsFromAddress(address)
	if err != nil {
		return nil, err
	}
	actionsToAddress, err := api.bc.GetActionsToAddress(address)
	if err != nil {
		return nil, err
	}
	return append(actions, actionsToAddress...), nil
}

func (api *Server) actionsInBlock(blk *block.Block, start, count uint64) []*iotexapi.ActionInfo {
	h := blk.HashBlock()
	blkHash := hex.EncodeToString(h[:])
	blkHeight := blk.Height()
	ts := blk.Header.BlockHeaderCoreProto().Timestamp

	var res []*iotexapi.ActionInfo
	for i := start; i < uint64(len(blk.Actions)) && i < start+count; i++ {
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

func getTranferAmountInBlock(blk *block.Block) *big.Int {
	totalAmount := big.NewInt(0)
	for _, selp := range blk.Actions {
		transfer, ok := selp.Action().(*action.Transfer)
		if !ok {
			continue
		}
		totalAmount.Add(totalAmount, transfer.Amount())
	}
	return totalAmount
}
