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
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
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
	grpcserver       *grpc.Server
}

// NewServer creates a new server
func NewServer(
	cfg config.API,
	chain blockchain.Blockchain,
	dispatcher dispatcher.Dispatcher,
	actPool actpool.ActPool,
	idx *indexservice.Server,
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
		return nil, err
	}
	pendingNonce, err := api.ap.GetPendingNonce(in.Address)
	if err != nil {
		return nil, err
	}
	accountMeta := &iotextypes.AccountMeta{
		Address:      in.Address,
		Balance:      state.Balance.String(),
		Nonce:        state.Nonce,
		PendingNonce: pendingNonce,
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
		return api.getAction(request.ActionHash, request.CheckPending)
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
	totalActions, err := api.bc.GetTotalActions()
	if err != nil {
		return nil, err
	}

	blockLimit := int64(api.cfg.TpsWindow)
	if blockLimit <= 0 {
		return nil, errors.Wrapf(ErrInternalServer, "block limit is %d", blockLimit)
	}

	// avoid genesis block
	if int64(tipHeight) < blockLimit {
		blockLimit = int64(tipHeight)
	}
	r, err := api.getBlockMetas(tipHeight, uint64(blockLimit))
	if err != nil {
		return nil, err
	}
	blks := r.BlkMetas

	if len(blks) == 0 {
		return nil, errors.New("get 0 blocks! not able to calculate aps")
	}
	epoch, err := api.getEpochData(tipHeight)
	if err != nil {
		return nil, err
	}

	timeDuration := blks[0].Timestamp - blks[len(blks)-1].Timestamp
	// if time duration is less than 1 second, we set it to be 1 second
	if timeDuration == 0 {
		timeDuration = 1
	}

	tps := int64(totalActions) / timeDuration

	chainMeta := &iotextypes.ChainMeta{
		Height:     tipHeight,
		Epoch:      epoch,
		Supply:     blockchain.Gen.TotalSupply.String(),
		NumActions: int64(totalActions),
		Tps:        tps,
	}

	return &iotexapi.GetChainMetaResponse{ChainMeta: chainMeta}, nil
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
		return nil, err
	}
	receipt, err := api.bc.GetReceiptByActionHash(actHash)
	if err != nil {
		return nil, err
	}

	return &iotexapi.GetReceiptByActionResponse{Receipt: receipt.ConvertToReceiptPb()}, nil
}

// ReadContract reads the state in a contract address specified by the slot
func (api *Server) ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest) (*iotexapi.ReadContractResponse, error) {
	log.L().Debug("receive read smart contract request")

	selp := &action.SealedEnvelope{}
	if err := selp.LoadProto(in.Action); err != nil {
		return nil, err
	}
	sc, ok := selp.Action().(*action.Execution)
	if !ok {
		return nil, errors.New("not execution")
	}

	callerPKHash := keypair.HashPubKey(selp.SrcPubkey())
	callerAddr, err := address.FromBytes(callerPKHash[:])
	if err != nil {
		return nil, err
	}

	res, err := api.bc.ExecuteContractRead(callerAddr, sc)
	if err != nil {
		return nil, err
	}
	return &iotexapi.ReadContractResponse{Data: hex.EncodeToString(res.ReturnValue)}, nil
}

// SuggestGasPrice suggests gas price
func (api *Server) SuggestGasPrice(ctx context.Context, in *iotexapi.SuggestGasPriceRequest) (*iotexapi.SuggestGasPriceResponse, error) {
	suggestPrice, err := api.gs.SuggestGasPrice()
	if err != nil {
		return nil, err
	}
	return &iotexapi.SuggestGasPriceResponse{GasPrice: suggestPrice}, nil
}

// EstimateGasForAction estimates gas for action
func (api *Server) EstimateGasForAction(ctx context.Context, in *iotexapi.EstimateGasForActionRequest) (*iotexapi.EstimateGasForActionResponse, error) {
	estimateGas, err := api.gs.EstimateGasForAction(in.Action)
	if err != nil {
		return nil, err
	}
	return &iotexapi.EstimateGasForActionResponse{Gas: estimateGas}, nil
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

// GetActions returns actions within the range
func (api *Server) getActions(start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotextypes.Action
	var actionCount uint64

	tipHeight := api.bc.TipHeight()
	for height := int64(tipHeight); height >= 0; height-- {
		blk, err := api.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, err
		}
		selps := blk.Actions
		for i := len(selps) - 1; i >= 0; i-- {
			actionCount++

			if actionCount <= start {
				continue
			}

			if uint64(len(res)) >= count {
				return &iotexapi.GetActionsResponse{Actions: res}, nil
			}
			res = append(res, selps[i].Proto())
		}
	}

	return &iotexapi.GetActionsResponse{Actions: res}, nil
}

// getAction returns action by action hash
func (api *Server) getAction(actionHash string, checkPending bool) (*iotexapi.GetActionsResponse, error) {
	actHash, err := toHash256(actionHash)
	if err != nil {
		return nil, err
	}
	actPb, err := getAction(api.bc, api.ap, actHash, checkPending)
	if err != nil {
		return nil, err
	}
	return &iotexapi.GetActionsResponse{Actions: []*iotextypes.Action{actPb}}, nil
}

// getActionsByAddress returns all actions associated with an address
func (api *Server) getActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotextypes.Action
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

	var actionCount uint64
	for i := len(actions) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}

		actPb, err := getAction(api.bc, api.ap, actions[i], false)
		if err != nil {
			return nil, err
		}

		res = append(res, actPb)
	}

	return &iotexapi.GetActionsResponse{Actions: res}, nil
}

// getUnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (api *Server) getUnconfirmedActionsByAddress(address string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotextypes.Action
	var actionCount uint64

	selps := api.ap.GetUnconfirmedActs(address)
	for i := len(selps) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}

		res = append(res, selps[i].Proto())
	}

	return &iotexapi.GetActionsResponse{Actions: res}, nil
}

// getActionsByBlock returns all actions in a block
func (api *Server) getActionsByBlock(blkHash string, start uint64, count uint64) (*iotexapi.GetActionsResponse, error) {
	var res []*iotextypes.Action
	hash, err := toHash256(blkHash)
	if err != nil {
		return nil, err
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	selps := blk.Actions
	var actionCount uint64
	for i := len(selps) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if uint64(len(res)) >= count {
			break
		}

		res = append(res, selps[i].Proto())
	}
	return &iotexapi.GetActionsResponse{Actions: res}, nil
}

// getBlockMetas gets block within the height range
func (api *Server) getBlockMetas(start uint64, number uint64) (*iotexapi.GetBlockMetasResponse, error) {
	var res []*iotextypes.BlockMeta

	startHeight := api.bc.TipHeight()
	var blkCount uint64
	for height := int(startHeight); height >= 0; height-- {
		blkCount++

		if blkCount <= start {
			continue
		}

		if uint64(len(res)) >= number {
			break
		}

		blk, err := api.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, err
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
		return nil, err
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, err
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

// getEpochData is the API to get epoch data
func (api *Server) getEpochData(height uint64) (*iotextypes.EpochData, error) {
	if height == 0 {
		return nil, errors.New("epoch data is not available to block 0")
	}
	// TODO: fill with real epoch data
	return &iotextypes.EpochData{
		Num:               0,
		Height:            0,
		BeaconChainHeight: 0,
	}, nil
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

func getAction(bc blockchain.Blockchain, ap actpool.ActPool, actHash hash.Hash256, checkPending bool) (*iotextypes.Action, error) {
	var selp action.SealedEnvelope
	var err error
	if selp, err = bc.GetActionByActionHash(actHash); err != nil {
		if checkPending {
			// Try to fetch pending action from actpool
			selp, err = ap.GetActionByHash(actHash)
		}
	}
	if err != nil {
		return nil, err
	}
	return selp.Proto(), nil
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
