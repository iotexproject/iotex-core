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

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	iproto "github.com/iotexproject/iotex-core/proto"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
	// ErrReceipt indicates the error of receipt
	ErrReceipt = errors.New("invalid receipt")
	// ErrAction indicates the error of action
	ErrAction = errors.New("invalid action")
)

var (
	requestMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_explorer_request",
			Help: "IoTeX Explorer request counter.",
		},
		[]string{"method", "succeed"},
	)
)

func init() {
	prometheus.MustRegister(requestMtc)
}

type (
	// BroadcastOutbound sends a broadcast message to the whole network
	BroadcastOutbound func(ctx context.Context, chainID uint32, msg proto.Message) error
	// Neighbors returns the neighbors' addresses
	Neighbors func(context.Context) ([]peerstore.PeerInfo, error)
	// NetworkInfo returns the self network information
	NetworkInfo func() peerstore.PeerInfo
)

// Service provide api for user to query blockchain data
type Service struct {
	bc                 blockchain.Blockchain
	dp                 dispatcher.Dispatcher
	ap                 actpool.ActPool
	gs                 GasStation
	broadcastHandler   BroadcastOutbound
	neighborsHandler   Neighbors
	networkInfoHandler NetworkInfo
	cfg                config.API
	idx                *indexservice.Server
}

// GetAccount returns the metadata of an account
func (api *Service) GetAccount(address string) (string, error) {
	state, err := api.bc.StateByAddr(address)
	if err != nil {
		return "", err
	}
	pendingNonce, err := api.ap.GetPendingNonce(address)
	if err != nil {
		return "", err
	}
	accountMeta := &iproto.AccountMeta{
		Address:      address,
		Balance:      state.Balance.String(),
		Nonce:        state.Nonce,
		PendingNonce: pendingNonce,
	}

	var marshaler jsonpb.Marshaler
	return marshaler.MarshalToString(accountMeta)
}

// GetActions returns actions within the range
func (api *Service) GetActions(start int64, count int64) ([]string, error) {
	var marshaler jsonpb.Marshaler
	var res []string
	var actionCount int64

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

			if int64(len(res)) >= count {
				return res, nil
			}
			actString, err := marshaler.MarshalToString(selps[i].Proto())
			if err != nil {
				return nil, err
			}
			res = append(res, actString)
		}
	}

	return res, nil
}

// GetAction returns action by action hash
func (api *Service) GetAction(actionHash string, checkPending bool) (string, error) {
	actHash, err := toHash256(actionHash)
	if err != nil {
		return "", err
	}
	return getAction(api.bc, api.ap, actHash, checkPending)
}

// GetActionsByAddress returns all actions associated with an address
func (api *Service) GetActionsByAddress(address string, start int64, count int64) ([]string, error) {
	var res []string
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

	var actionCount int64
	for i := len(actions) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if int64(len(res)) >= count {
			break
		}

		actString, err := getAction(api.bc, api.ap, actions[i], false)
		if err != nil {
			return nil, err
		}

		res = append(res, actString)
	}

	return res, nil
}

// GetUnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (api *Service) GetUnconfirmedActionsByAddress(address string, start int64, count int64) ([]string, error) {
	var marshaler jsonpb.Marshaler
	var res []string
	var actionCount int64

	selps := api.ap.GetUnconfirmedActs(address)
	for i := len(selps) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if int64(len(res)) >= count {
			break
		}

		actString, err := marshaler.MarshalToString(selps[i].Proto())
		if err != nil {
			return nil, err
		}
		res = append(res, actString)
	}

	return res, nil
}

// GetActionsByBlock returns all actions in a block
func (api *Service) GetActionsByBlock(blkHash string, start int64, count int64) ([]string, error) {
	var marshaler jsonpb.Marshaler
	var res []string
	hash, err := toHash256(blkHash)
	if err != nil {
		return nil, err
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	selps := blk.Actions
	var actionCount int64
	for i := len(selps) - 1; i >= 0; i-- {
		actionCount++

		if actionCount <= start {
			continue
		}

		if int64(len(res)) >= count {
			break
		}

		actString, err := marshaler.MarshalToString(selps[i].Proto())
		if err != nil {
			return nil, err
		}
		res = append(res, actString)
	}
	return res, nil
}

// GetBlockMetas gets block within the height range
func (api *Service) GetBlockMetas(start int64, number int64) ([]string, error) {
	var marshaler jsonpb.Marshaler
	var res []string

	startHeight := api.bc.TipHeight()
	var blkCount int64
	for height := int(startHeight); height >= 0; height-- {
		blkCount++

		if blkCount <= start {
			continue
		}

		if int64(len(res)) >= number {
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

		blockMeta := &iproto.BlockMeta{
			Hash:             hex.EncodeToString(hash[:]),
			Height:           blk.Height(),
			Timestamp:        blockHeaderPb.GetTimestamp().GetSeconds(),
			NumActions:       int64(len(blk.Actions)),
			ProducerAddress:  blk.ProducerAddress(),
			TransferAmount:   transferAmount.String(),
			TxRoot:           hex.EncodeToString(txRoot[:]),
			ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
			DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
		}

		blkMetaString, err := marshaler.MarshalToString(blockMeta)
		if err != nil {
			return nil, err
		}

		res = append(res, blkMetaString)
	}

	return res, nil
}

// GetBlockMeta returns block by block hash
func (api *Service) GetBlockMeta(blkHash string) (string, error) {
	hash, err := toHash256(blkHash)
	if err != nil {
		return "", err
	}

	blk, err := api.bc.GetBlockByHash(hash)
	if err != nil {
		return "", err
	}

	blkHeaderPb := blk.ConvertToBlockHeaderPb()
	txRoot := blk.TxRoot()
	receiptRoot := blk.ReceiptRoot()
	deltaStateDigest := blk.DeltaStateDigest()
	transferAmount := getTranferAmountInBlock(blk)

	blockMeta := &iproto.BlockMeta{
		Hash:             blkHash,
		Height:           blk.Height(),
		Timestamp:        blkHeaderPb.GetTimestamp().GetSeconds(),
		NumActions:       int64(len(blk.Actions)),
		ProducerAddress:  blk.ProducerAddress(),
		TransferAmount:   transferAmount.String(),
		TxRoot:           hex.EncodeToString(txRoot[:]),
		ReceiptRoot:      hex.EncodeToString(receiptRoot[:]),
		DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
	}

	var marshaler jsonpb.Marshaler
	return marshaler.MarshalToString(blockMeta)
}

// GetChainMeta returns blockchain metadata
func (api *Service) GetChainMeta() (string, error) {
	tipHeight := api.bc.TipHeight()
	totalActions, err := api.bc.GetTotalActions()
	if err != nil {
		return "", err
	}

	blockLimit := int64(api.cfg.TpsWindow)
	if blockLimit <= 0 {
		return "", errors.Wrapf(ErrInternalServer, "block limit is %d", blockLimit)
	}

	// avoid genesis block
	if int64(tipHeight) < blockLimit {
		blockLimit = int64(tipHeight)
	}
	blkStrs, err := api.GetBlockMetas(int64(tipHeight), blockLimit)
	if err != nil {
		return "", err
	}

	if len(blkStrs) == 0 {
		return "", errors.New("get 0 blocks! not able to calculate aps")
	}

	var lastBlk iproto.BlockMeta
	if err := jsonpb.UnmarshalString(blkStrs[0], &lastBlk); err != nil {
		return "", err
	}
	var firstBlk iproto.BlockMeta
	if err := jsonpb.UnmarshalString(blkStrs[len(blkStrs)-1], &firstBlk); err != nil {
		return "", err
	}

	timeDuration := lastBlk.Timestamp - firstBlk.Timestamp
	// if time duration is less than 1 second, we set it to be 1 second
	if timeDuration == 0 {
		timeDuration = 1
	}

	tps := int64(totalActions) / timeDuration

	chainMeta := &iproto.ChainMeta{
		Height:     tipHeight,
		Supply:     blockchain.Gen.TotalSupply.String(),
		NumActions: int64(totalActions),
		Tps:        tps,
	}

	var marshaler jsonpb.Marshaler
	return marshaler.MarshalToString(chainMeta)
}

// SendAction is the API to send an action to blockchain.
func (api *Service) SendAction(req string) (res string, err error) {
	log.L().Debug("receive send action request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
		}
		requestMtc.WithLabelValues("SendAction", succeed).Inc()
	}()
	var act iproto.ActionPb

	if err := jsonpb.UnmarshalString(req, &act); err != nil {
		return "", err
	}

	// broadcast to the network
	if err = api.broadcastHandler(context.Background(), api.bc.ChainID(), &act); err != nil {
		log.L().Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	// send to actpool via dispatcher
	api.dp.HandleBroadcast(context.Background(), api.bc.ChainID(), &act)

	var selp action.SealedEnvelope
	if err := selp.LoadProto(&act); err != nil {
		return "", err
	}

	hash := selp.Hash()
	return hex.EncodeToString(hash[:]), nil
}

// GetReceiptByAction gets receipt with corresponding action hash
func (api *Service) GetReceiptByAction(hash string) (string, error) {
	actHash, err := toHash256(hash)
	if err != nil {
		return "", err
	}
	receipt, err := api.bc.GetReceiptByActionHash(actHash)
	if err != nil {
		return "", err
	}

	var marshaler jsonpb.Marshaler
	return marshaler.MarshalToString(receipt.ConvertToReceiptPb())
}

// ReadContract reads the state in a contract address specified by the slot
func (api *Service) ReadContract(request string) (string, error) {
	log.L().Debug("receive read smart contract request")

	var actPb iproto.ActionPb
	if err := jsonpb.UnmarshalString(request, &actPb); err != nil {
		return "", err
	}
	selp := &action.SealedEnvelope{}
	if err := selp.LoadProto(&actPb); err != nil {
		return "", err
	}
	sc, ok := selp.Action().(*action.Execution)
	if !ok {
		return "", errors.New("not execution")
	}

	callerPKHash := keypair.HashPubKey(selp.SrcPubkey())
	callerAddr, err := address.FromBytes(callerPKHash[:])
	if err != nil {
		return "", err
	}

	res, err := api.bc.ExecuteContractRead(callerAddr, sc)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(res.ReturnValue), nil
}

// SuggestGasPrice suggests gas price
func (api *Service) SuggestGasPrice() (int64, error) {
	return api.gs.suggestGasPrice()
}

// EstimateGasForAction estimates gas for action
func (api *Service) EstimateGasForAction(request string) (int64, error) {
	return api.gs.estimateGasForAction(request)
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

func getAction(bc blockchain.Blockchain, ap actpool.ActPool, actHash hash.Hash256, checkPending bool) (string, error) {
	var marshaler jsonpb.Marshaler
	var selp action.SealedEnvelope
	var err error
	if selp, err = bc.GetActionByActionHash(actHash); err != nil {
		if checkPending {
			// Try to fetch pending action from actpool
			selp, err = ap.GetActionByHash(actHash)
		}
	}
	if err != nil {
		return "", err
	}
	return marshaler.MarshalToString(selp.Proto())
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
