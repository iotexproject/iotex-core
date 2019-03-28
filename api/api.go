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
	"strconv"

	"github.com/golang/protobuf/proto"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
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
	idx              *indexservice.Server
	registry         *protocol.Registry
	grpcserver       *grpc.Server
}

// NewServer creates a new server
func NewServer(
	cfg config.API,
	chain blockchain.Blockchain,
	dispatcher dispatcher.Dispatcher,
	actPool actpool.ActPool,
	idx *indexservice.Server,
	registry *protocol.Registry,
	opts ...Option,
) (*Server, error) {
	apiCfg := Config{}
	for _, opt := range opts {
		if err := opt(&apiCfg); err != nil {
			return nil, err
		}
	}

	if cfg == (config.API{}) {
		log.L().Warn("API server is not configured.")
		cfg = config.Default.API
	}

	svr := &Server{
		bc:               chain,
		dp:               dispatcher,
		ap:               actPool,
		broadcastHandler: apiCfg.broadcastHandler,
		cfg:              cfg,
		idx:              idx,
		registry:         registry,
		gs:               gasstation.NewGasStation(chain, cfg),
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
		return nil, nil
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
		return nil, nil
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

	timeDuration := blks[len(blks)-1].Timestamp - blks[0].Timestamp
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

	return &iotexapi.SendActionResponse{}, nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (api *Server) GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error) {
	actHash, err := toHash256(in.ActionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	receipt, err := api.bc.GetReceiptByActionHash(actHash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &iotexapi.GetReceiptByActionResponse{Receipt: receipt.ConvertToReceiptPb()}, nil
}

// ReadContract reads the state in a contract address specified by the slot
func (api *Server) ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest) (*iotexapi.ReadContractResponse, error) {
	log.L().Debug("receive read smart contract request")

	selp := &action.SealedEnvelope{}
	if err := selp.LoadProto(in.Action); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	sc, ok := selp.Action().(*action.Execution)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "not an execution")
	}

	callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := api.bc.ExecuteContractRead(callerAddr, sc)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.ReadContractResponse{Data: hex.EncodeToString(res.ReturnValue)}, nil
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
		MethodName: []byte("BlockProducersByHeight"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochHeight)},
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
	return nil
}

// Stop stops the API server
func (api *Server) Stop() error {
	api.grpcserver.Stop()
	log.L().Info("API server stops.")
	return nil
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
func (api *Server) getActions(start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotexapi.ActionInfo
	var actionCount uint64

	tipHeight := api.bc.TipHeight()
	for height := 1; height <= int(tipHeight); height++ {
		blk, err := api.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		selps := blk.Actions
		for i := 0; i < len(selps); i++ {
			actionCount++

			if actionCount <= start {
				continue
			}

			if uint64(len(res)) >= count {
				return &iotexapi.GetActionsResponse{ActionInfo: res}, nil
			}
			act, err := api.convertToAction(selps[i], true)
			if err != nil {
				return nil, err
			}
			res = append(res, act)
		}
	}

	return &iotexapi.GetActionsResponse{ActionInfo: res}, nil
}

// getSingleAction returns action by action hash
func (api *Server) getSingleAction(actionHash string, checkPending bool) (*iotexapi.GetActionsResponse, error) {
	actHash, err := toHash256(actionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	act, err := api.getAction(actHash, checkPending)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &iotexapi.GetActionsResponse{ActionInfo: []*iotexapi.ActionInfo{act}}, nil
}

// getActionsByAddress returns all actions associated with an address
func (api *Server) getActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotexapi.ActionInfo
	actions, err := api.getTotalActionsByAddress(address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	var actionCount uint64
	for i := 0; i < len(actions); i++ {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}

		act, err := api.getAction(actions[i], false)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		res = append(res, act)
	}

	return &iotexapi.GetActionsResponse{ActionInfo: res}, nil
}

// getUnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (api *Server) getUnconfirmedActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotexapi.ActionInfo
	var actionCount uint64

	selps := api.ap.GetUnconfirmedActs(address)
	for i := 0; i < len(selps); i++ {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}
		act, err := api.convertToAction(selps[i], false)
		if err != nil {
			return nil, err
		}
		res = append(res, act)
	}

	return &iotexapi.GetActionsResponse{ActionInfo: res}, nil
}

// getActionsByBlock returns all actions in a block
func (api *Server) getActionsByBlock(blkHash string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotexapi.ActionInfo
	hash, err := toHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	selps := blk.Actions
	var actionCount uint64
	for i := 0; i < len(selps); i++ {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}

		act, err := api.convertToAction(selps[i], true)
		if err != nil {
			return nil, err
		}
		res = append(res, act)
	}
	return &iotexapi.GetActionsResponse{ActionInfo: res}, nil
}

// getBlockMetas gets block within the height range
func (api *Server) getBlockMetas(start uint64, number uint64) (*iotexapi.GetBlockMetasResponse, error) {
	tipHeight := api.bc.TipHeight()
	if start > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	var res []*iotextypes.BlockMeta
	for height := int(start); height <= int(tipHeight); height++ {
		if uint64(len(res)) >= number {
			break
		}
		blk, err := api.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		blockHeaderPb := blk.ConvertToBlockHeaderPb()

		hash := blk.HashBlock()
		txRoot := blk.TxRoot()
		receiptRoot := blk.ReceiptRoot()
		deltaStateDigest := blk.DeltaStateDigest()
		transferAmount := getTranferAmountInBlock(blk)

		blockMeta := &iotextypes.BlockMeta{
			Hash:             hex.EncodeToString(hash[:]),
			Height:           blk.Height(),
			Timestamp:        blockHeaderPb.GetCore().GetTimestamp().GetSeconds(),
			NumActions:       int64(len(blk.Actions)),
			ProducerAddress:  blk.ProducerAddress(),
			TransferAmount:   transferAmount.String(),
			TxRoot:           hex.EncodeToString(txRoot[:]),
			ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
			DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
		}

		res = append(res, blockMeta)
	}

	return &iotexapi.GetBlockMetasResponse{BlkMetas: res}, nil
}

// getBlockMeta returns block by block hash
func (api *Server) getBlockMeta(blkHash string) (*iotexapi.GetBlockMetasResponse, error) {
	hash, err := toHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	blkHeaderPb := blk.ConvertToBlockHeaderPb()
	txRoot := blk.TxRoot()
	receiptRoot := blk.ReceiptRoot()
	deltaStateDigest := blk.DeltaStateDigest()
	transferAmount := getTranferAmountInBlock(blk)

	blockMeta := &iotextypes.BlockMeta{
		Hash:             blkHash,
		Height:           blk.Height(),
		Timestamp:        blkHeaderPb.GetCore().GetTimestamp().GetSeconds(),
		NumActions:       int64(len(blk.Actions)),
		ProducerAddress:  blk.ProducerAddress(),
		TransferAmount:   transferAmount.String(),
		TxRoot:           hex.EncodeToString(txRoot[:]),
		ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
		DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
	}

	return &iotexapi.GetBlockMetasResponse{BlkMetas: []*iotextypes.BlockMeta{blockMeta}}, nil
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

func (api *Server) convertToAction(selp action.SealedEnvelope, pullBlkHash bool) (*iotexapi.ActionInfo, error) {
	actHash := selp.Hash()
	blkHash := hash.ZeroHash256
	var err error
	if pullBlkHash {
		if blkHash, err = api.bc.GetBlockHashByActionHash(actHash); err != nil {
			return nil, err
		}
	}
	return &iotexapi.ActionInfo{
		Action:  selp.Proto(),
		ActHash: hex.EncodeToString(actHash[:]),
		BlkHash: hex.EncodeToString(blkHash[:]),
	}, nil
}

func (api *Server) getAction(actHash hash.Hash256, checkPending bool) (*iotexapi.ActionInfo, error) {
	var selp action.SealedEnvelope
	var err error
	if selp, err = api.bc.GetActionByActionHash(actHash); err != nil {
		if checkPending {
			// Try to fetch pending action from actpool
			selp, err = api.ap.GetActionByHash(actHash)
		}
	}
	if err != nil {
		return nil, err
	}
	return api.convertToAction(selp, !checkPending)
}

func (api *Server) getTotalActionsByAddress(address string) ([]hash.Hash256, error) {
	var actions []hash.Hash256
	if api.cfg.UseRDS {
		actionHistory, err := api.idx.Indexer().GetIndexHistory(config.IndexAction, address)
		if err != nil {
			return nil, err
		}
		actions = append(actions, actionHistory...)
	} else {
		actionsFromAddress, err := api.bc.GetActionsFromAddress(address)
		if err != nil {
			return nil, err
		}

		actionsToAddress, err := api.bc.GetActionsToAddress(address)
		if err != nil {
			return nil, err
		}

		actionsFromAddress = append(actionsFromAddress, actionsToAddress...)
		actions = append(actions, actionsFromAddress...)
	}
	return actions, nil
}

func toHash256(hashString string) (hash.Hash256, error) {
	bytes, err := hex.DecodeString(hashString)
	if err != nil {
		return hash.ZeroHash256, err
	}
	var hash hash.Hash256
	copy(hash[:], bytes)
	return hash, nil
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
