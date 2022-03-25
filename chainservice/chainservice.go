// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// ChainService is a blockchain service with all blockchain components.
	ChainService struct {
		lifecycle         lifecycle.Lifecycle
		actpool           actpool.ActPool
		blocksync         blocksync.BlockSync
		consensus         consensus.Consensus
		chain             blockchain.Blockchain
		factory           factory.Factory
		blockdao          blockdao.BlockDAO
		p2pAgent          p2p.Agent
		electionCommittee committee.Committee
		// TODO: explorer dependency deleted at #1085, need to api related params
		api                *api.ServerV2
		indexer            blockindex.Indexer
		bfIndexer          blockindex.BloomFilterIndexer
		candidateIndexer   *poll.CandidateIndexer
		candBucketsIndexer *staking.CandidatesBucketsIndexer
		registry           *protocol.Registry

		readCache *ReadCache
		gs        *gasstation.GasStation
	}

	intrinsicGasCalculator interface {
		IntrinsicGas() (uint64, error)
	}
)

// Start starts the server
func (cs *ChainService) Start(ctx context.Context) error {
	return cs.lifecycle.OnStartSequentially(ctx)
}

// Stop stops the server
func (cs *ChainService) Stop(ctx context.Context) error {
	return cs.lifecycle.OnStopSequentially(ctx)
}

// ReportFullness switch on or off block sync
func (cs *ChainService) ReportFullness(_ context.Context, _ iotexrpc.MessageType, fullness float32) {
}

// HandleAction handles incoming action request.
func (cs *ChainService) HandleAction(ctx context.Context, actPb *iotextypes.Action) error {
	act, err := (&action.Deserializer{}).ActionToSealedEnvelope(actPb)
	if err != nil {
		return err
	}
	ctx = protocol.WithRegistry(ctx, cs.registry)
	err = cs.actpool.Add(ctx, act)
	if err != nil {
		log.L().Debug(err.Error())
	}
	return err
}

// HandleBlock handles incoming block request.
func (cs *ChainService) HandleBlock(ctx context.Context, peer string, pbBlock *iotextypes.Block) error {
	blk, err := (&block.Deserializer{}).FromBlockProto(pbBlock)
	if err != nil {
		return err
	}
	ctx, err = cs.chain.Context(ctx)
	if err != nil {
		return err
	}
	return cs.blocksync.ProcessBlock(ctx, peer, blk)
}

// HandleSyncRequest handles incoming sync request.
func (cs *ChainService) HandleSyncRequest(ctx context.Context, peer peer.AddrInfo, sync *iotexrpc.BlockSync) error {
	return cs.blocksync.ProcessSyncRequest(ctx, sync.Start, sync.End, func(ctx context.Context, blk *block.Block) error {
		return cs.p2pAgent.UnicastOutbound(
			ctx,
			peer,
			blk.ConvertToBlockPb(),
		)
	})
}

// HandleConsensusMsg handles incoming consensus message.
func (cs *ChainService) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	return cs.consensus.HandleConsensusMsg(msg)
}

// ChainID returns ChainID.
func (cs *ChainService) ChainID() uint32 { return cs.chain.ChainID() }

// Blockchain returns the Blockchain
func (cs *ChainService) Blockchain() blockchain.Blockchain {
	return cs.chain
}

// StateFactory returns the state factory
func (cs *ChainService) StateFactory() factory.Factory {
	return cs.factory
}

// BlockDAO returns the blockdao
func (cs *ChainService) BlockDAO() blockdao.BlockDAO {
	return cs.blockdao
}

// ActionPool returns the Action pool
func (cs *ChainService) ActionPool() actpool.ActPool {
	return cs.actpool
}

// APIServer defines the interface of core service of the server
type APIServer interface {
	GetActions(ctx context.Context, in *iotexapi.GetActionsRequest) (*iotexapi.GetActionsResponse, error)
	GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest) (*iotexapi.GetReceiptByActionResponse, error)
}

// APIServer returns the API server
func (cs *ChainService) APIServer() APIServer {
	return cs.api.GrpcServer
}

// Consensus returns the consensus instance
func (cs *ChainService) Consensus() consensus.Consensus {
	return cs.consensus
}

// BlockSync returns the block syncer
func (cs *ChainService) BlockSync() blocksync.BlockSync {
	return cs.blocksync
}

// Registry returns a pointer to the registry
func (cs *ChainService) Registry() *protocol.Registry { return cs.registry }

// Account returns the metadata of an account
func (cs *ChainService) Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	ctx, span := tracer.NewSpan(context.Background(), "coreService.Account")
	defer span.End()
	addrStr := addr.String()
	if addrStr == address.RewardingPoolAddr || addrStr == address.StakingBucketPoolAddr {
		return cs.getProtocolAccount(ctx, addrStr)
	}
	span.AddEvent("accountutil.AccountStateWithHeight")
	state, tipHeight, err := accountutil.AccountStateWithHeight(cs.factory, addr)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	span.AddEvent("ap.GetPendingNonce")
	pendingNonce, err := cs.actpool.GetPendingNonce(addrStr)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	if cs.indexer == nil {
		return nil, nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	span.AddEvent("indexer.GetActionCount")
	numActions, err := cs.indexer.GetActionCountByAddress(hash.BytesToHash160(addr.Bytes()))
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	accountMeta := &iotextypes.AccountMeta{
		Address:      addrStr,
		Balance:      state.Balance.String(),
		Nonce:        state.Nonce,
		PendingNonce: pendingNonce,
		NumActions:   numActions,
		IsContract:   state.IsContract(),
	}
	if state.IsContract() {
		var code protocol.SerializableBytes
		_, err = cs.factory.State(&code, protocol.NamespaceOption(evm.CodeKVNameSpace), protocol.KeyOption(state.CodeHash))
		if err != nil {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		accountMeta.ContractByteCode = code
	}
	span.AddEvent("bc.BlockHeaderByHeight")
	header, err := cs.chain.BlockHeaderByHeight(tipHeight)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	hash := header.HashBlock()
	span.AddEvent("coreService.Account.End")
	return accountMeta, &iotextypes.BlockIdentifier{
		Hash:   hex.EncodeToString(hash[:]),
		Height: tipHeight,
	}, nil
}

// ChainMeta returns blockchain metadata
func (cs *ChainService) ChainMeta(blockLimit uint64) (*iotextypes.ChainMeta, string, error) {
	tipHeight := cs.chain.TipHeight()
	if tipHeight == 0 {
		return &iotextypes.ChainMeta{
			Epoch:   &iotextypes.EpochData{},
			ChainID: cs.chain.ChainID(),
		}, "", nil
	}
	syncStatus := ""
	if cs.blocksync != nil {
		syncStatus = cs.blocksync.SyncStatus()
	}
	chainMeta := &iotextypes.ChainMeta{
		Height:  tipHeight,
		ChainID: cs.chain.ChainID(),
	}
	if cs.indexer == nil {
		return chainMeta, syncStatus, nil
	}
	totalActions, err := cs.indexer.GetTotalActions()
	if err != nil {
		return nil, "", status.Error(codes.Internal, err.Error())
	}
	// avoid genesis block
	if tipHeight < blockLimit {
		blockLimit = tipHeight
	}
	blks, err := cs.BlockMetas(tipHeight-uint64(blockLimit)+1, uint64(blockLimit))
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	if len(blks) == 0 {
		return nil, "", status.Error(codes.NotFound, "get 0 blocks! not able to calculate aps")
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

	chainMeta.NumActions = int64(totalActions)
	chainMeta.Tps = int64(math.Ceil(float64(tps)))
	chainMeta.TpsFloat = tps

	rp := rolldpos.FindProtocol(cs.registry)
	if rp != nil {
		epochNum := rp.GetEpochNum(tipHeight)
		epochHeight := rp.GetEpochHeight(epochNum)
		gravityChainStartHeight, err := cs.getGravityChainStartHeight(epochHeight)
		if err != nil {
			return nil, "", status.Error(codes.NotFound, err.Error())
		}
		chainMeta.Epoch = &iotextypes.EpochData{
			Num:                     epochNum,
			Height:                  epochHeight,
			GravityChainStartHeight: gravityChainStartHeight,
		}
	}
	return chainMeta, syncStatus, nil
}

// SendAction is the API to send an action to blockchain.
func (cs *ChainService) SendAction(ctx context.Context, in *iotextypes.Action) (string, error) {
	log.Logger("api").Debug("receive send action request")
	selp, err := (&action.Deserializer{}).ActionToSealedEnvelope(in)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, err.Error())
	}

	// reject action if chainID is not matched at KamchatkaHeight
	if err := cs.validateChainID(in.GetCore().GetChainID()); err != nil {
		return "", err
	}

	// Add to local actpool
	ctx = protocol.WithRegistry(ctx, cs.registry)
	hash, err := selp.Hash()
	if err != nil {
		return "", err
	}
	l := log.Logger("api").With(zap.String("actionHash", hex.EncodeToString(hash[:])))
	if err = cs.actpool.Add(ctx, selp); err != nil {
		txBytes, serErr := proto.Marshal(in)
		if serErr != nil {
			l.Error("Data corruption", zap.Error(serErr))
		} else {
			l.With(zap.String("txBytes", hex.EncodeToString(txBytes))).Error("Failed to accept action", zap.Error(err))
		}
		st := status.New(codes.Internal, err.Error())
		br := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "Action rejected",
					Description: action.LoadErrorDescription(err),
				},
			},
		}
		st, err := st.WithDetails(br)
		if err != nil {
			log.Logger("api").Panic("Unexpected error attaching metadata", zap.Error(err))
		}
		return "", st.Err()
	}
	// If there is no error putting into local actpool,
	// Broadcast it to the network
	if err = cs.p2pAgent.BroadcastOutbound(ctx, in); err != nil {
		l.Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	return hex.EncodeToString(hash[:]), nil
}

// PendingNonce returns the pending nonce of an address
func (cs *ChainService) PendingNonce(addr address.Address) (uint64, error) {
	return cs.actpool.GetPendingNonce(addr.String())
}

// ReceiptByAction gets receipt with corresponding action hash
func (cs *ChainService) ReceiptByAction(actHash hash.Hash256) (*action.Receipt, string, error) {
	if cs.indexer == nil {
		return nil, "", status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	receipt, err := cs.ReceiptByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := cs.getBlockHashByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	return receipt, hex.EncodeToString(blkHash[:]), nil
}

// ReadContract reads the state in a contract address specified by the slot
func (cs *ChainService) ReadContract(ctx context.Context, callerAddr address.Address, sc *action.Execution) (string, *iotextypes.Receipt, error) {
	log.Logger("api").Debug("receive read smart contract request")
	key := hash.Hash160b(append([]byte(sc.Contract()), sc.Data()...))
	// TODO: either moving readcache into the upper layer or change the storage format
	if d, ok := cs.readCache.Get(key); ok {
		res := iotexapi.ReadContractResponse{}
		if err := proto.Unmarshal(d, &res); err == nil {
			return res.Data, res.Receipt, nil
		}
	}
	state, err := accountutil.AccountState(cs.factory, callerAddr)
	if err != nil {
		return "", nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if ctx, err = cs.chain.Context(ctx); err != nil {
		return "", nil, err
	}
	sc.SetNonce(state.Nonce + 1)
	blockGasLimit := cs.chain.Genesis().BlockGasLimit
	if sc.GasLimit() == 0 || blockGasLimit < sc.GasLimit() {
		sc.SetGasLimit(blockGasLimit)
	}
	sc.SetGasPrice(big.NewInt(0)) // ReadContract() is read-only, use 0 to prevent insufficient gas

	retval, receipt, err := cs.factory.SimulateExecution(ctx, callerAddr, sc, cs.blockdao.GetBlockHash)
	if err != nil {
		return "", nil, status.Error(codes.Internal, err.Error())
	}
	// ReadContract() is read-only, if no error returned, we consider it a success
	receipt.Status = uint64(iotextypes.ReceiptStatus_Success)
	res := iotexapi.ReadContractResponse{
		Data:    hex.EncodeToString(retval),
		Receipt: receipt.ConvertToReceiptPb(),
	}
	if d, err := proto.Marshal(&res); err == nil {
		cs.readCache.Put(key, d)
	}
	return res.Data, res.Receipt, nil
}

// ReadState reads state on blockchain
func (cs *ChainService) ReadState(protocolID string, height string, methodName []byte, arguments [][]byte) (*iotexapi.ReadStateResponse, error) {
	p, ok := cs.registry.Find(protocolID)
	if !ok {
		return nil, status.Errorf(codes.Internal, "protocol %s isn't registered", protocolID)
	}
	data, readStateHeight, err := cs.readState(context.Background(), p, height, methodName, arguments...)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := cs.blockdao.GetBlockHash(readStateHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &iotexapi.ReadStateResponse{
		Data: data,
		BlockIdentifier: &iotextypes.BlockIdentifier{
			Height: readStateHeight,
			Hash:   hex.EncodeToString(blkHash[:]),
		},
	}, nil
}

// SuggestGasPrice suggests gas price
func (cs *ChainService) SuggestGasPrice() (uint64, error) {
	return cs.gs.SuggestGasPrice()
}

// EstimateGasForAction estimates gas for action
func (cs *ChainService) EstimateGasForAction(in *iotextypes.Action) (uint64, error) {
	estimateGas, err := cs.gs.EstimateGasForAction(in)
	if err != nil {
		return 0, status.Error(codes.Internal, err.Error())
	}
	return estimateGas, nil
}

// EpochMeta gets epoch metadata
func (cs *ChainService) EpochMeta(epochNum uint64) (*iotextypes.EpochData, uint64, []*iotexapi.BlockProducerInfo, error) {
	rp := rolldpos.FindProtocol(cs.registry)
	if rp == nil {
		return nil, 0, nil, nil
	}
	if epochNum < 1 {
		return nil, 0, nil, status.Error(codes.InvalidArgument, "epoch number cannot be less than one")
	}
	epochHeight := rp.GetEpochHeight(epochNum)
	gravityChainStartHeight, err := cs.getGravityChainStartHeight(epochHeight)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}
	epochData := &iotextypes.EpochData{
		Num:                     epochNum,
		Height:                  epochHeight,
		GravityChainStartHeight: gravityChainStartHeight,
	}

	pp := poll.FindProtocol(cs.registry)
	if pp == nil {
		return nil, 0, nil, status.Error(codes.Internal, "poll protocol is not registered")
	}

	methodName := []byte("ActiveBlockProducersByEpoch")
	arguments := [][]byte{[]byte(strconv.FormatUint(epochNum, 10))}
	height := strconv.FormatUint(epochHeight, 10)
	data, _, err := cs.readState(context.Background(), pp, height, methodName, arguments...)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}

	var activeConsensusBlockProducers state.CandidateList
	if err := activeConsensusBlockProducers.Deserialize(data); err != nil {
		return nil, 0, nil, status.Error(codes.Internal, err.Error())
	}

	numBlks, produce, err := cs.getProductivityByEpoch(rp, epochNum, cs.chain.TipHeight(), activeConsensusBlockProducers)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}

	methodName = []byte("BlockProducersByEpoch")
	data, _, err = cs.readState(context.Background(), pp, height, methodName, arguments...)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}

	var BlockProducers state.CandidateList
	if err := BlockProducers.Deserialize(data); err != nil {
		return nil, 0, nil, status.Error(codes.Internal, err.Error())
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
	return epochData, numBlks, blockProducersInfo, nil
}

// RawBlocks gets raw block data
func (cs *ChainService) RawBlocks(startHeight uint64, count uint64, withReceipts bool, withTransactionLogs bool) ([]*iotexapi.BlockInfo, error) {
	tipHeight := cs.chain.TipHeight()
	if startHeight > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	endHeight := startHeight + count - 1
	if endHeight > tipHeight {
		endHeight = tipHeight
	}
	var res []*iotexapi.BlockInfo
	for height := startHeight; height <= endHeight; height++ {
		blk, err := cs.blockdao.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		var receiptsPb []*iotextypes.Receipt
		if withReceipts && height > 0 {
			receipts, err := cs.blockdao.GetReceipts(height)
			if err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			for _, receipt := range receipts {
				receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
			}
		}
		var transactionLogs *iotextypes.TransactionLogs
		if withTransactionLogs {
			if transactionLogs, err = cs.blockdao.TransactionLogs(height); err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
		}
		res = append(res, &iotexapi.BlockInfo{
			Block:           blk.ConvertToBlockPb(),
			Receipts:        receiptsPb,
			TransactionLogs: transactionLogs,
		})
	}
	return res, nil
}

// ElectionBuckets returns the native election buckets.
func (cs *ChainService) ElectionBuckets(epochNum uint64) ([]*iotextypes.ElectionBucket, error) {
	if cs.electionCommittee == nil {
		return nil, status.Error(codes.Unavailable, "Native election no supported")
	}
	buckets, err := cs.electionCommittee.NativeBucketsByEpoch(epochNum)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	re := make([]*iotextypes.ElectionBucket, len(buckets))
	for i, b := range buckets {
		startTime := timestamppb.New(b.StartTime())
		re[i] = &iotextypes.ElectionBucket{
			Voter:     b.Voter(),
			Candidate: b.Candidate(),
			Amount:    b.Amount().Bytes(),
			StartTime: startTime,
			Duration:  durationpb.New(b.Duration()),
			Decay:     b.Decay(),
		}
	}
	return re, nil
}

// ReceiptByActionHash returns receipt by action hash
func (cs *ChainService) ReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	if cs.indexer == nil {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	actIndex, err := cs.indexer.GetActionIndex(h[:])
	if err != nil {
		return nil, err
	}
	return cs.blockdao.GetReceiptByActionHash(h, actIndex.BlockHeight())
}

// TransactionLogByActionHash returns transaction log by action hash
func (cs *ChainService) TransactionLogByActionHash(actHash string) (*iotextypes.TransactionLog, error) {
	if cs.indexer == nil {
		return nil, status.Error(codes.Unimplemented, blockindex.ErrActionIndexNA.Error())
	}
	if !cs.blockdao.ContainsTransactionLog() {
		return nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	h, err := hex.DecodeString(actHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	actIndex, err := cs.indexer.GetActionIndex(h)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	sysLog, err := cs.blockdao.TransactionLogs(actIndex.BlockHeight())
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, log := range sysLog.Logs {
		if bytes.Equal(h, log.ActionHash) {
			return log, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "transaction log not found for action %s", actHash)
}

// TransactionLogByBlockHeight returns transaction log by block height
func (cs *ChainService) TransactionLogByBlockHeight(blockHeight uint64) (*iotextypes.BlockIdentifier, *iotextypes.TransactionLogs, error) {
	if !cs.blockdao.ContainsTransactionLog() {
		return nil, nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	tip, err := cs.blockdao.Height()
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	if blockHeight < 1 || blockHeight > tip {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid block height = %d", blockHeight)
	}

	h, err := cs.blockdao.GetBlockHash(blockHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}

	blockIdentifier := &iotextypes.BlockIdentifier{
		Hash:   hex.EncodeToString(h[:]),
		Height: blockHeight,
	}
	sysLog, err := cs.blockdao.TransactionLogs(blockHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			// should return empty, no transaction happened in block
			return blockIdentifier, nil, nil
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	return blockIdentifier, sysLog, nil
}

// TipHeight returns the tip height of block chain
func (cs *ChainService) TipHeight() uint64 {
	return cs.chain.TipHeight()
}

// Indexer returns the tip height of block chain
func (cs *ChainService) Indexer() blockindex.Indexer {
	return cs.indexer
}

func (cs *ChainService) readState(ctx context.Context, p protocol.Protocol, height string, methodName []byte, arguments ...[]byte) ([]byte, uint64, error) {
	key := ReadKey{
		Name:   p.Name(),
		Height: height,
		Method: methodName,
		Args:   arguments,
	}
	if d, ok := cs.readCache.Get(key.Hash()); ok {
		var h uint64
		if height != "" {
			h, _ = strconv.ParseUint(height, 0, 64)
		}
		return d, h, nil
	}

	// TODO: need to complete the context
	tipHeight := cs.chain.TipHeight()
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: tipHeight,
	})
	ctx = genesis.WithGenesisContext(
		protocol.WithRegistry(ctx, cs.registry),
		cs.chain.Genesis(),
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	rp := rolldpos.FindProtocol(cs.registry)
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
			d, h, err := p.ReadState(ctx, factory.NewHistoryStateReader(cs.factory, rp.GetEpochHeight(inputEpochNum)), methodName, arguments...)
			if err == nil {
				cs.readCache.Put(key.Hash(), d)
			}
			return d, h, err
		}
	}

	// TODO: need to distinguish user error and system error
	d, h, err := p.ReadState(ctx, cs.factory, methodName, arguments...)
	if err == nil {
		cs.readCache.Put(key.Hash(), d)
	}
	return d, h, err
}

func (cs *ChainService) getActionsFromIndex(totalActions, start, count uint64) ([]*iotexapi.ActionInfo, error) {
	hashes, err := cs.indexer.GetActionHashFromIndex(start, count)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	var actionInfo []*iotexapi.ActionInfo
	for i := range hashes {
		act, err := cs.getAction(hash.BytesToHash256(hashes[i]), false)
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		actionInfo = append(actionInfo, act)
	}
	return actionInfo, nil
}

// Actions returns actions within the range
func (cs *ChainService) Actions(start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if err := cs.checkActionIndex(); err != nil {
		return nil, err
	}
	totalActions, err := cs.indexer.GetTotalActions()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if start >= totalActions {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the total actions in the block")
	}
	if totalActions == uint64(0) || count == 0 {
		return []*iotexapi.ActionInfo{}, nil
	}
	if start+count > totalActions {
		count = totalActions - start
	}
	if cs.indexer != nil {
		return cs.getActionsFromIndex(totalActions, start, count)
	}
	// Finding actions in reverse order saves time for querying most recent actions
	reverseStart := totalActions - (start + count)
	if totalActions < start+count {
		reverseStart = uint64(0)
		count = totalActions - start
	}

	var res []*iotexapi.ActionInfo
	var hit bool
	for height := cs.chain.TipHeight(); height >= 1 && count > 0; height-- {
		blk, err := cs.blockdao.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if !hit && reverseStart >= uint64(len(blk.Actions)) {
			reverseStart -= uint64(len(blk.Actions))
			continue
		}
		// now reverseStart < len(blk.Actions), we are going to fetch actions from this block
		hit = true
		act := cs.reverseActionsInBlock(blk, reverseStart, count)
		res = append(act, res...)
		count -= uint64(len(act))
		reverseStart = 0
	}
	return res, nil
}

// Action returns action by action hash
func (cs *ChainService) Action(actionHash string, checkPending bool) (*iotexapi.ActionInfo, error) {
	if err := cs.checkActionIndex(); err != nil {
		return nil, err
	}
	actHash, err := hash.HexStringToHash256(actionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	act, err := cs.getAction(actHash, checkPending)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return act, nil
}

// ActionsByAddress returns all actions associated with an address
func (cs *ChainService) ActionsByAddress(addr address.Address, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if err := cs.checkActionIndex(); err != nil {
		return nil, err
	}
	actions, err := cs.indexer.GetActionsByAddress(hash.BytesToHash160(addr.Bytes()), start, count)
	if err != nil {
		if errors.Cause(err) == db.ErrBucketNotExist || errors.Cause(err) == db.ErrNotExist {
			// no actions associated with address, return nil
			return nil, nil
		}
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var res []*iotexapi.ActionInfo
	for i := range actions {
		act, err := cs.getAction(hash.BytesToHash256(actions[i]), false)
		if err != nil {
			continue
		}
		res = append(res, act)
	}
	return res, nil
}

// getBlockHashByActionHash returns block hash by action hash
func (cs *ChainService) getBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	actIndex, err := cs.indexer.GetActionIndex(h[:])
	if err != nil {
		return hash.ZeroHash256, err
	}
	return cs.blockdao.GetBlockHash(actIndex.BlockHeight())
}

// ActionByActionHash returns action by action hash
func (cs *ChainService) ActionByActionHash(h hash.Hash256) (action.SealedEnvelope, hash.Hash256, uint64, uint32, error) {
	if err := cs.checkActionIndex(); err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	actIndex, err := cs.indexer.GetActionIndex(h[:])
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, err
	}

	blk, err := cs.blockdao.GetBlockByHeight(actIndex.BlockHeight())
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, err
	}

	selp, index, err := cs.blockdao.GetActionByActionHash(h, actIndex.BlockHeight())
	return selp, blk.HashBlock(), actIndex.BlockHeight(), index, err
}

// UnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (cs *ChainService) UnconfirmedActionsByAddress(address string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	selps := cs.actpool.GetUnconfirmedActs(address)
	if len(selps) == 0 {
		return []*iotexapi.ActionInfo{}, nil
	}
	if start >= uint64(len(selps)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	var res []*iotexapi.ActionInfo
	for i := start; i < uint64(len(selps)) && i < start+count; i++ {
		if act, err := cs.pendingAction(selps[i]); err == nil {
			res = append(res, act)
		}
	}
	return res, nil
}

// ActionsByBlock returns all actions in a block
func (cs *ChainService) ActionsByBlock(blkHash string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if err := cs.checkActionIndex(); err != nil {
		return nil, err
	}
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blk, err := cs.blockdao.GetBlock(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if start >= uint64(len(blk.Actions)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	return cs.actionsInBlock(blk, start, count), nil
}

// BlockMetas returns blockmetas response within the height range
func (cs *ChainService) BlockMetas(start uint64, count uint64) ([]*iotextypes.BlockMeta, error) {
	var (
		tipHeight = cs.chain.TipHeight()
		res       = make([]*iotextypes.BlockMeta, 0)
	)
	if start > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	for height := start; height <= tipHeight && count > 0; height++ {
		blockMeta, err := cs.getBlockMetaByHeight(height)
		if err != nil {
			return nil, err
		}
		res = append(res, blockMeta)
		count--
	}
	return res, nil
}

// BlockMetaByHash returns blockmeta response by block hash
func (cs *ChainService) BlockMetaByHash(blkHash string) (*iotextypes.BlockMeta, error) {
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	height, err := cs.blockdao.GetBlockHeight(hash)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return cs.getBlockMetaByHeight(height)
}

// getBlockMetaByHeight gets BlockMeta by height
func (cs *ChainService) getBlockMetaByHeight(height uint64) (*iotextypes.BlockMeta, error) {
	blk, err := cs.blockdao.GetBlockByHeight(height)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	// get block's receipt
	if blk.Height() > 0 {
		blk.Receipts, err = cs.blockdao.GetReceipts(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
	}
	return generateBlockMeta(blk), nil
}

// generateBlockMeta generates BlockMeta from block
func generateBlockMeta(blk *block.Block) *iotextypes.BlockMeta {
	header := blk.Header
	height := header.Height()
	ts := timestamppb.New(header.Timestamp())
	var (
		producerAddress string
		h               hash.Hash256
	)
	if blk.Height() > 0 {
		producerAddress = header.ProducerAddress()
		h = header.HashBlock()
	} else {
		h = block.GenesisHash()
	}
	txRoot := header.TxRoot()
	receiptRoot := header.ReceiptRoot()
	deltaStateDigest := header.DeltaStateDigest()
	prevHash := header.PrevHash()

	blockMeta := iotextypes.BlockMeta{
		Hash:              hex.EncodeToString(h[:]),
		Height:            height,
		Timestamp:         ts,
		ProducerAddress:   producerAddress,
		TxRoot:            hex.EncodeToString(txRoot[:]),
		ReceiptRoot:       hex.EncodeToString(receiptRoot[:]),
		DeltaStateDigest:  hex.EncodeToString(deltaStateDigest[:]),
		PreviousBlockHash: hex.EncodeToString(prevHash[:]),
	}
	if logsBloom := header.LogsBloomfilter(); logsBloom != nil {
		blockMeta.LogsBloom = hex.EncodeToString(logsBloom.Bytes())
	}
	blockMeta.NumActions = int64(len(blk.Actions))
	blockMeta.TransferAmount = blk.CalculateTransferAmount().String()
	blockMeta.GasLimit, blockMeta.GasUsed = gasLimitAndUsed(blk)
	return &blockMeta
}

// GasLimitAndUsed returns the gas limit and used in a block
func gasLimitAndUsed(b *block.Block) (uint64, uint64) {
	var gasLimit, gasUsed uint64
	for _, tx := range b.Actions {
		gasLimit += tx.GasLimit()
	}
	for _, r := range b.Receipts {
		gasUsed += r.GasConsumed
	}
	return gasLimit, gasUsed
}

func (cs *ChainService) getGravityChainStartHeight(epochHeight uint64) (uint64, error) {
	gravityChainStartHeight := epochHeight
	if pp := poll.FindProtocol(cs.registry); pp != nil {
		methodName := []byte("GetGravityChainStartHeight")
		arguments := [][]byte{[]byte(strconv.FormatUint(epochHeight, 10))}
		data, _, err := cs.readState(context.Background(), pp, "", methodName, arguments...)
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

func (cs *ChainService) committedAction(selp action.SealedEnvelope, blkHash hash.Hash256, blkHeight uint64) (*iotexapi.ActionInfo, error) {
	actHash, err := selp.Hash()
	if err != nil {
		return nil, err
	}
	header, err := cs.blockdao.Header(blkHash)
	if err != nil {
		return nil, err
	}
	sender := selp.SrcPubkey().Address()
	receipt, err := cs.blockdao.GetReceiptByActionHash(actHash, blkHeight)
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

func (cs *ChainService) pendingAction(selp action.SealedEnvelope) (*iotexapi.ActionInfo, error) {
	actHash, err := selp.Hash()
	if err != nil {
		return nil, err
	}
	sender := selp.SrcPubkey().Address()
	return &iotexapi.ActionInfo{
		Action:    selp.Proto(),
		ActHash:   hex.EncodeToString(actHash[:]),
		BlkHash:   hex.EncodeToString(hash.ZeroHash256[:]),
		BlkHeight: 0,
		Sender:    sender.String(),
		Timestamp: nil,
		Index:     0,
	}, nil
}

func (cs *ChainService) getAction(actHash hash.Hash256, checkPending bool) (*iotexapi.ActionInfo, error) {
	selp, blkHash, blkHeight, actIndex, err := cs.ActionByActionHash(actHash)
	if err == nil {
		act, err := cs.committedAction(selp, blkHash, blkHeight)
		if err != nil {
			return nil, err
		}
		act.Index = actIndex
		return act, nil
	}
	// Try to fetch pending action from actpool
	if checkPending {
		selp, err = cs.actpool.GetActionByHash(actHash)
	}
	if err != nil {
		return nil, err
	}
	return cs.pendingAction(selp)
}

func (cs *ChainService) actionsInBlock(blk *block.Block, start, count uint64) []*iotexapi.ActionInfo {
	var res []*iotexapi.ActionInfo
	if len(blk.Actions) == 0 || start >= uint64(len(blk.Actions)) {
		return res
	}

	h := blk.HashBlock()
	blkHash := hex.EncodeToString(h[:])
	blkHeight := blk.Height()

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
		actHash, err := selp.Hash()
		if err != nil {
			log.Logger("api").Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		receipt, err := cs.blockdao.GetReceiptByActionHash(actHash, blkHeight)
		if err != nil {
			log.Logger("api").Debug("Skipping action due to failing to get receipt", zap.Error(err))
			continue
		}
		gas := new(big.Int).Mul(selp.GasPrice(), big.NewInt(int64(receipt.GasConsumed)))
		sender := selp.SrcPubkey().Address()
		res = append(res, &iotexapi.ActionInfo{
			Action:    selp.Proto(),
			ActHash:   hex.EncodeToString(actHash[:]),
			BlkHash:   blkHash,
			Timestamp: blk.Header.BlockHeaderCoreProto().Timestamp,
			BlkHeight: blkHeight,
			Sender:    sender.String(),
			GasFee:    gas.String(),
			Index:     uint32(i),
		})
	}
	return res
}

func (cs *ChainService) reverseActionsInBlock(blk *block.Block, reverseStart, count uint64) []*iotexapi.ActionInfo {
	h := blk.HashBlock()
	blkHash := hex.EncodeToString(h[:])
	blkHeight := blk.Height()

	var res []*iotexapi.ActionInfo
	for i := reverseStart; i < uint64(len(blk.Actions)) && i < reverseStart+count; i++ {
		ri := uint64(len(blk.Actions)) - 1 - i
		selp := blk.Actions[ri]
		actHash, err := selp.Hash()
		if err != nil {
			log.Logger("api").Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		receipt, err := cs.blockdao.GetReceiptByActionHash(actHash, blkHeight)
		if err != nil {
			log.Logger("api").Debug("Skipping action due to failing to get receipt", zap.Error(err))
			continue
		}
		gas := new(big.Int).Mul(selp.GasPrice(), big.NewInt(int64(receipt.GasConsumed)))
		sender := selp.SrcPubkey().Address()
		res = append([]*iotexapi.ActionInfo{{
			Action:    selp.Proto(),
			ActHash:   hex.EncodeToString(actHash[:]),
			BlkHash:   blkHash,
			Timestamp: blk.Header.BlockHeaderCoreProto().Timestamp,
			BlkHeight: blkHeight,
			Sender:    sender.String(),
			GasFee:    gas.String(),
			Index:     uint32(ri),
		}}, res...)
	}
	return res
}

// LogsInBlockByHash gets the logs in a block by block hash
func (cs *ChainService) LogsInBlockByHash(filter *logfilter.LogFilter, blockHash hash.Hash256) ([]*iotextypes.Log, error) {
	blkHeight, err := cs.blockdao.GetBlockHeight(blockHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid block hash")
	}
	return cs.logsInBlock(filter, blkHeight)
}

// LogsInBlock filter logs in the block x
func (cs *ChainService) LogsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error) {
	return cs.logsInBlock(filter, blockNumber)
}

func (cs *ChainService) logsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error) {
	logBloomFilter, err := cs.bfIndexer.BlockFilterByHeight(blockNumber)
	if err != nil {
		return nil, err
	}

	if !filter.ExistInBloomFilterv2(logBloomFilter) {
		return []*iotextypes.Log{}, nil
	}

	receipts, err := cs.blockdao.GetReceipts(blockNumber)
	if err != nil {
		return nil, err
	}

	h, err := cs.blockdao.GetBlockHash(blockNumber)
	if err != nil {
		return nil, err
	}

	return filter.MatchLogs(receipts, h), nil
}

// LogsInRange filter logs among [start, end] blocks
func (cs *ChainService) LogsInRange(filter *logfilter.LogFilter, start, end, paginationSize uint64) ([]*iotextypes.Log, error) {
	start, end, err := cs.correctLogsRange(start, end)
	if err != nil {
		return nil, err
	}
	// getLogs via range Blooom filter [start, end]
	blockNumbers, err := cs.bfIndexer.FilterBlocksInRange(filter, start, end)
	if err != nil {
		return nil, err
	}

	// TODO: improve using goroutine
	if paginationSize == 0 {
		paginationSize = 1000
	}
	if paginationSize > 5000 {
		paginationSize = 5000
	}
	logs := []*iotextypes.Log{}
	for _, i := range blockNumbers {
		logsInBlock, err := cs.LogsInBlock(filter, i)
		if err != nil {
			return nil, err
		}
		for _, log := range logsInBlock {
			logs = append(logs, log)
			if len(logs) >= int(paginationSize) {
				return logs, nil
			}
		}
	}
	return logs, nil
}

func (cs *ChainService) correctLogsRange(start, end uint64) (uint64, uint64, error) {
	if start > end {
		return 0, 0, errors.New("invalid start and end height")
	}
	if start == 0 {
		start = 1
	}
	if start > cs.chain.TipHeight() {
		return 0, 0, errors.New("start block > tip height")
	}
	if end > cs.chain.TipHeight() || end == 0 {
		end = cs.chain.TipHeight()
	}
	return start, end, nil
}

// EstimateGasForNonExecution estimates action gas except execution
func (cs *ChainService) EstimateGasForNonExecution(actType action.Action) (uint64, error) {
	act, ok := actType.(intrinsicGasCalculator)
	if !ok {
		return 0, errors.Errorf("invalid action type not supported")
	}
	return act.IntrinsicGas()
}

// EstimateExecutionGasConsumption estimate gas consumption for execution action
func (cs *ChainService) EstimateExecutionGasConsumption(ctx context.Context, sc *action.Execution, callerAddr address.Address) (uint64, error) {
	state, err := accountutil.AccountState(cs.factory, callerAddr)
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, err.Error())
	}
	sc.SetNonce(state.Nonce + 1)
	sc.SetGasPrice(big.NewInt(0))
	blockGasLimit := cs.chain.Genesis().BlockGasLimit
	sc.SetGasLimit(blockGasLimit)
	enough, receipt, err := cs.isGasLimitEnough(ctx, callerAddr, sc)
	if err != nil {
		return 0, status.Error(codes.Internal, err.Error())
	}
	if !enough {
		if receipt.ExecutionRevertMsg() != "" {
			return 0, status.Errorf(codes.Internal, fmt.Sprintf("execution simulation is reverted due to the reason: %s", receipt.ExecutionRevertMsg()))
		}
		return 0, status.Error(codes.Internal, fmt.Sprintf("execution simulation failed: status = %d", receipt.Status))
	}
	estimatedGas := receipt.GasConsumed
	sc.SetGasLimit(estimatedGas)
	enough, _, err = cs.isGasLimitEnough(ctx, callerAddr, sc)
	if err != nil && err != action.ErrInsufficientFunds {
		return 0, status.Error(codes.Internal, err.Error())
	}
	if !enough {
		low, high := estimatedGas, blockGasLimit
		estimatedGas = high
		for low <= high {
			mid := (low + high) / 2
			sc.SetGasLimit(mid)
			enough, _, err = cs.isGasLimitEnough(ctx, callerAddr, sc)
			if err != nil && err != action.ErrInsufficientFunds {
				return 0, status.Error(codes.Internal, err.Error())
			}
			if enough {
				estimatedGas = mid
				high = mid - 1
			} else {
				low = mid + 1
			}
		}
	}

	return estimatedGas, nil
}

func (cs *ChainService) isGasLimitEnough(
	ctx context.Context,
	caller address.Address,
	sc *action.Execution,
) (bool, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "Server.isGasLimitEnough")
	defer span.End()
	ctx, err := cs.chain.Context(ctx)
	if err != nil {
		return false, nil, err
	}
	_, receipt, err := cs.factory.SimulateExecution(ctx, caller, sc, cs.blockdao.GetBlockHash)
	if err != nil {
		return false, nil, err
	}
	return receipt.Status == uint64(iotextypes.ReceiptStatus_Success), receipt, nil
}

func (cs *ChainService) getProductivityByEpoch(
	rp *rolldpos.Protocol,
	epochNum uint64,
	tipHeight uint64,
	abps state.CandidateList,
) (uint64, map[string]uint64, error) {
	num, produce, err := rp.ProductivityByEpoch(epochNum, tipHeight, func(start uint64, end uint64) (map[string]uint64, error) {
		return blockchain.Productivity(cs.chain, start, end)
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

func (cs *ChainService) checkActionIndex() error {
	if cs.indexer == nil {
		return errors.New("no action index")
	}
	return nil
}

func (cs *ChainService) getProtocolAccount(ctx context.Context, addr string) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	span := tracer.SpanFromContext(ctx)
	defer span.End()
	var (
		balance string
		out     *iotexapi.ReadStateResponse
		err     error
	)
	switch addr {
	case address.RewardingPoolAddr:
		if out, err = cs.ReadState("rewarding", "", []byte("TotalBalance"), nil); err != nil {
			return nil, nil, err
		}
		val, ok := new(big.Int).SetString(string(out.GetData()), 10)
		if !ok {
			return nil, nil, errors.New("balance convert error")
		}
		balance = val.String()
	case address.StakingBucketPoolAddr:
		methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
			Method: iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT,
		})
		if err != nil {
			return nil, nil, err
		}
		arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
			Request: &iotexapi.ReadStakingDataRequest_TotalStakingAmount_{
				TotalStakingAmount: &iotexapi.ReadStakingDataRequest_TotalStakingAmount{},
			},
		})
		if err != nil {
			return nil, nil, err
		}
		if out, err = cs.ReadState("staking", "", methodName, [][]byte{arg}); err != nil {
			return nil, nil, err
		}
		acc := iotextypes.AccountMeta{}
		if err := proto.Unmarshal(out.GetData(), &acc); err != nil {
			return nil, nil, errors.Wrap(err, "failed to unmarshal account meta")
		}
		balance = acc.GetBalance()
	default:
		return nil, nil, errors.Errorf("invalid address %s", addr)
	}
	return &iotextypes.AccountMeta{
		Address: addr,
		Balance: balance,
	}, out.GetBlockIdentifier(), nil
}

// ActPoolActions returns the all Transaction Identifiers in the mempool
func (cs *ChainService) ActPoolActions(actHashes []string) ([]*iotextypes.Action, error) {
	var ret []*iotextypes.Action
	if len(actHashes) == 0 {
		for _, sealeds := range cs.actpool.PendingActionMap() {
			for _, sealed := range sealeds {
				ret = append(ret, sealed.Proto())
			}
		}
		return ret, nil
	}

	for _, hashStr := range actHashes {
		hs, err := hash.HexStringToHash256(hashStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, errors.Wrap(err, "failed to hex string to hash256").Error())
		}
		sealed, err := cs.actpool.GetActionByHash(hs)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		ret = append(ret, sealed.Proto())
	}
	return ret, nil
}

// EVMNetworkID returns the network id of evm
func (cs *ChainService) EVMNetworkID() uint32 {
	return config.EVMNetworkID()
}

// ReadContractStorage reads contract's storage
func (cs *ChainService) ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error) {
	ctx, err := cs.chain.Context(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return cs.factory.ReadContractStorage(ctx, addr, key)
}

// ReceiveBlock clears chainservice cache on receiving a new block
func (cs *ChainService) ReceiveBlock(blk *block.Block) error {
	// TODO: move readCache to api service and delete this function
	cs.readCache.Clear()
	return nil
}

// SimulateExecution simulates an execution
func (cs *ChainService) SimulateExecution(ctx context.Context, addr address.Address, exec *action.Execution) ([]byte, *action.Receipt, error) {
	state, err := accountutil.AccountState(cs.factory, addr)
	if err != nil {
		return nil, nil, err
	}
	ctx, err = cs.chain.Context(ctx)
	if err != nil {
		return nil, nil, err
	}
	// TODO (liuhaai): Use original nonce and gas limit properly
	exec.SetNonce(state.Nonce + 1)
	if err != nil {
		return nil, nil, err
	}
	exec.SetGasLimit(cs.chain.Genesis().BlockGasLimit)
	return cs.factory.SimulateExecution(ctx, addr, exec, cs.blockdao.GetBlockHash)
}

func (cs *ChainService) validateChainID(chainID uint32) error {
	if ge := cs.chain.Genesis(); ge.IsMidway(cs.chain.TipHeight()) && chainID != cs.chain.ChainID() && chainID != 0 {
		return status.Errorf(codes.InvalidArgument, "ChainID does not match, expecting %d, got %d", cs.chain.ChainID(), chainID)
	}
	return nil
}
