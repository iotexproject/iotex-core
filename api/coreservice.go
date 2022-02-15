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
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

// coreService provides api for user to interact with blockchain data
type coreService struct {
	bc                blockchain.Blockchain
	bs                blocksync.BlockSync
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
	hasActionIndex    bool
	electionCommittee committee.Committee
	readCache         *ReadCache
}

// newcoreService creates a api server that contains major blockchain components
func newCoreService(
	cfg config.Config,
	chain blockchain.Blockchain,
	bs blocksync.BlockSync,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	indexer blockindex.Indexer,
	bfIndexer blockindex.BloomFilterIndexer,
	actPool actpool.ActPool,
	registry *protocol.Registry,
	opts ...Option,
) (CoreService, error) {
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
	svr := &coreService{
		bc:                chain,
		bs:                bs,
		sf:                sf,
		dao:               dao,
		indexer:           indexer,
		bfIndexer:         bfIndexer,
		ap:                actPool,
		broadcastHandler:  apiCfg.broadcastHandler,
		cfg:               cfg,
		registry:          registry,
		chainListener:     NewChainListener(500),
		gs:                gasstation.NewGasStation(chain, sf.SimulateExecution, dao, cfg.API),
		electionCommittee: apiCfg.electionCommittee,
		readCache:         NewReadCache(),
	}
	if _, ok := cfg.Plugins[config.GatewayPlugin]; ok {
		svr.hasActionIndex = true
	}
	return svr, nil
}

// Account returns the metadata of an account
func (core *coreService) Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	ctx, span := tracer.NewSpan(context.Background(), "coreService.Account")
	defer span.End()
	addrStr := addr.String()
	if addrStr == address.RewardingPoolAddr || addrStr == address.StakingBucketPoolAddr {
		return core.getProtocolAccount(ctx, addrStr)
	}
	span.AddEvent("accountutil.AccountStateWithHeight")
	state, tipHeight, err := accountutil.AccountStateWithHeight(core.sf, addr)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	span.AddEvent("ap.GetPendingNonce")
	pendingNonce, err := core.ap.GetPendingNonce(addrStr)
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	if core.indexer == nil {
		return nil, nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	span.AddEvent("indexer.GetActionCount")
	numActions, err := core.indexer.GetActionCountByAddress(hash.BytesToHash160(addr.Bytes()))
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
		_, err = core.sf.State(&code, protocol.NamespaceOption(evm.CodeKVNameSpace), protocol.KeyOption(state.CodeHash))
		if err != nil {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		accountMeta.ContractByteCode = code
	}
	span.AddEvent("bc.BlockHeaderByHeight")
	header, err := core.bc.BlockHeaderByHeight(tipHeight)
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
func (core *coreService) ChainMeta() (*iotextypes.ChainMeta, string, error) {
	tipHeight := core.bc.TipHeight()
	if tipHeight == 0 {
		return &iotextypes.ChainMeta{
			Epoch:   &iotextypes.EpochData{},
			ChainID: core.bc.ChainID(),
		}, "", nil
	}
	syncStatus := ""
	if core.bs != nil {
		syncStatus = core.bs.SyncStatus()
	}
	chainMeta := &iotextypes.ChainMeta{
		Height:  tipHeight,
		ChainID: core.bc.ChainID(),
	}
	if core.indexer == nil {
		return chainMeta, syncStatus, nil
	}
	totalActions, err := core.indexer.GetTotalActions()
	if err != nil {
		return nil, "", status.Error(codes.Internal, err.Error())
	}
	blockLimit := int64(core.cfg.API.TpsWindow)
	if blockLimit <= 0 {
		return nil, "", status.Errorf(codes.Internal, "block limit is %d", blockLimit)
	}

	// avoid genesis block
	if int64(tipHeight) < blockLimit {
		blockLimit = int64(tipHeight)
	}
	blks, err := core.BlockMetas(tipHeight-uint64(blockLimit)+1, uint64(blockLimit))
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

	rp := rolldpos.FindProtocol(core.registry)
	if rp != nil {
		epochNum := rp.GetEpochNum(tipHeight)
		epochHeight := rp.GetEpochHeight(epochNum)
		gravityChainStartHeight, err := core.getGravityChainStartHeight(epochHeight)
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

// ServerMeta gets the server metadata
func (core *coreService) ServerMeta() (packageVersion string, packageCommitID string, gitStatus string, goVersion string, buildTime string) {
	packageVersion = version.PackageVersion
	packageCommitID = version.PackageCommitID
	gitStatus = version.GitStatus
	goVersion = version.GoVersion
	buildTime = version.BuildTime
	return
}

// SendAction is the API to send an action to blockchain.
func (core *coreService) SendAction(ctx context.Context, in *iotextypes.Action) (string, error) {
	log.Logger("api").Debug("receive send action request")
	var selp action.SealedEnvelope
	if err := selp.LoadProto(in); err != nil {
		return "", status.Error(codes.InvalidArgument, err.Error())
	}

	// reject action if chainID is not matched at KamchatkaHeight
	if err := core.validateChainID(in.GetCore().GetChainID()); err != nil {
		return "", err
	}

	// Add to local actpool
	ctx = protocol.WithRegistry(ctx, core.registry)
	hash, err := selp.Hash()
	if err != nil {
		return "", err
	}
	l := log.Logger("api").With(zap.String("actionHash", hex.EncodeToString(hash[:])))
	if err = core.ap.Add(ctx, selp); err != nil {
		txBytes, serErr := proto.Marshal(in)
		if serErr != nil {
			l.Error("Data corruption", zap.Error(serErr))
		} else {
			l.With(zap.String("txBytes", hex.EncodeToString(txBytes))).Error("Failed to accept action", zap.Error(err))
		}
		errMsg := core.cfg.ProducerAddress().String() + ": " + err.Error()
		st := status.New(codes.Internal, errMsg)
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
	if err = core.broadcastHandler(ctx, core.bc.ChainID(), in); err != nil {
		l.Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	return hex.EncodeToString(hash[:]), nil
}

func (core *coreService) validateChainID(chainID uint32) error {
	if core.cfg.Genesis.Blockchain.IsMidway(core.bc.TipHeight()) &&
		chainID != core.bc.ChainID() && chainID != 0 {
		return status.Errorf(codes.InvalidArgument, "ChainID does not match, expecting %d, got %d", core.bc.ChainID(), chainID)
	}
	return nil
}

// ReceiptByAction gets receipt with corresponding action hash
func (core *coreService) ReceiptByAction(actHash hash.Hash256) (*action.Receipt, string, error) {
	if !core.hasActionIndex || core.indexer == nil {
		return nil, "", status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}
	receipt, err := core.ReceiptByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := core.getBlockHashByActionHash(actHash)
	if err != nil {
		return nil, "", status.Error(codes.NotFound, err.Error())
	}
	return receipt, hex.EncodeToString(blkHash[:]), nil
}

// ReadContract reads the state in a contract address specified by the slot
func (core *coreService) ReadContract(ctx context.Context, callerAddr address.Address, sc *action.Execution) (string, *iotextypes.Receipt, error) {
	log.Logger("api").Debug("receive read smart contract request")
	key := hash.Hash160b(append([]byte(sc.Contract()), sc.Data()...))
	// TODO: either moving readcache into the upper layer or change the storage format
	if d, ok := core.readCache.Get(key); ok {
		res := iotexapi.ReadContractResponse{}
		if err := proto.Unmarshal(d, &res); err == nil {
			return res.Data, res.Receipt, nil
		}
	}
	state, err := accountutil.AccountState(core.sf, callerAddr)
	if err != nil {
		return "", nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if ctx, err = core.bc.Context(ctx); err != nil {
		return "", nil, err
	}
	sc.SetNonce(state.Nonce + 1)
	if sc.GasLimit() == 0 || core.cfg.Genesis.BlockGasLimit < sc.GasLimit() {
		sc.SetGasLimit(core.cfg.Genesis.BlockGasLimit)
	}
	sc.SetGasPrice(big.NewInt(0)) // ReadContract() is read-only, use 0 to prevent insufficient gas

	retval, receipt, err := core.sf.SimulateExecution(ctx, callerAddr, sc, core.dao.GetBlockHash)
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
		core.readCache.Put(key, d)
	}
	return res.Data, res.Receipt, nil
}

// ReadState reads state on blockchain
func (core *coreService) ReadState(protocolID string, height string, methodName []byte, arguments [][]byte) (*iotexapi.ReadStateResponse, error) {
	p, ok := core.registry.Find(protocolID)
	if !ok {
		return nil, status.Errorf(codes.Internal, "protocol %s isn't registered", protocolID)
	}
	data, readStateHeight, err := core.readState(context.Background(), p, height, methodName, arguments...)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	blkHash, err := core.dao.GetBlockHash(readStateHeight)
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
func (core *coreService) SuggestGasPrice() (uint64, error) {
	return core.gs.SuggestGasPrice()
}

// EstimateGasForAction estimates gas for action
func (core *coreService) EstimateGasForAction(in *iotextypes.Action) (uint64, error) {
	estimateGas, err := core.gs.EstimateGasForAction(in)
	if err != nil {
		return 0, status.Error(codes.Internal, err.Error())
	}
	return estimateGas, nil
}

// EpochMeta gets epoch metadata
func (core *coreService) EpochMeta(epochNum uint64) (*iotextypes.EpochData, uint64, []*iotexapi.BlockProducerInfo, error) {
	rp := rolldpos.FindProtocol(core.registry)
	if rp == nil {
		return nil, 0, nil, nil
	}
	if epochNum < 1 {
		return nil, 0, nil, status.Error(codes.InvalidArgument, "epoch number cannot be less than one")
	}
	epochHeight := rp.GetEpochHeight(epochNum)
	gravityChainStartHeight, err := core.getGravityChainStartHeight(epochHeight)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}
	epochData := &iotextypes.EpochData{
		Num:                     epochNum,
		Height:                  epochHeight,
		GravityChainStartHeight: gravityChainStartHeight,
	}

	pp := poll.FindProtocol(core.registry)
	if pp == nil {
		return nil, 0, nil, status.Error(codes.Internal, "poll protocol is not registered")
	}

	methodName := []byte("ActiveBlockProducersByEpoch")
	arguments := [][]byte{[]byte(strconv.FormatUint(epochNum, 10))}
	height := strconv.FormatUint(epochHeight, 10)
	data, _, err := core.readState(context.Background(), pp, height, methodName, arguments...)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}

	var activeConsensusBlockProducers state.CandidateList
	if err := activeConsensusBlockProducers.Deserialize(data); err != nil {
		return nil, 0, nil, status.Error(codes.Internal, err.Error())
	}

	numBlks, produce, err := core.getProductivityByEpoch(rp, epochNum, core.bc.TipHeight(), activeConsensusBlockProducers)
	if err != nil {
		return nil, 0, nil, status.Error(codes.NotFound, err.Error())
	}

	methodName = []byte("BlockProducersByEpoch")
	data, _, err = core.readState(context.Background(), pp, height, methodName, arguments...)
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
func (core *coreService) RawBlocks(startHeight uint64, count uint64, withReceipts bool, withTransactionLogs bool) ([]*iotexapi.BlockInfo, error) {
	if count == 0 || count > core.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	tipHeight := core.bc.TipHeight()
	if startHeight > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	endHeight := startHeight + count - 1
	if endHeight > tipHeight {
		endHeight = tipHeight
	}
	var res []*iotexapi.BlockInfo
	for height := startHeight; height <= endHeight; height++ {
		blk, err := core.dao.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		var receiptsPb []*iotextypes.Receipt
		if withReceipts && height > 0 {
			receipts, err := core.dao.GetReceipts(height)
			if err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			for _, receipt := range receipts {
				receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
			}
		}
		var transactionLogs *iotextypes.TransactionLogs
		if withTransactionLogs {
			if transactionLogs, err = core.dao.TransactionLogs(height); err != nil {
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

// StreamBlocks streams blocks
func (core *coreService) StreamBlocks(stream iotexapi.APIService_StreamBlocksServer) error {
	errChan := make(chan error)
	if err := core.chainListener.AddResponder(NewBlockListener(stream, errChan)); err != nil {
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
func (core *coreService) StreamLogs(in *iotexapi.LogsFilter, stream iotexapi.APIService_StreamLogsServer) error {
	if in == nil {
		return status.Error(codes.InvalidArgument, "empty filter")
	}
	errChan := make(chan error)
	// register the log filter so it will match logs in new blocks
	if err := core.chainListener.AddResponder(logfilter.NewLogFilter(in, stream, errChan)); err != nil {
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

// ElectionBuckets returns the native election buckets.
func (core *coreService) ElectionBuckets(epochNum uint64) ([]*iotextypes.ElectionBucket, error) {
	if core.electionCommittee == nil {
		return nil, status.Error(codes.Unavailable, "Native election no supported")
	}
	buckets, err := core.electionCommittee.NativeBucketsByEpoch(epochNum)
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
	return re, nil
}

// ReceiptByActionHash returns receipt by action hash
func (core *coreService) ReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	if !core.hasActionIndex || core.indexer == nil {
		return nil, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	actIndex, err := core.indexer.GetActionIndex(h[:])
	if err != nil {
		return nil, err
	}
	return core.dao.GetReceiptByActionHash(h, actIndex.BlockHeight())
}

// TransactionLogByActionHash returns transaction log by action hash
func (core *coreService) TransactionLogByActionHash(actHash string) (*iotextypes.TransactionLog, error) {
	if !core.hasActionIndex || core.indexer == nil {
		return nil, status.Error(codes.Unimplemented, blockindex.ErrActionIndexNA.Error())
	}
	if !core.dao.ContainsTransactionLog() {
		return nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	h, err := hex.DecodeString(actHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	actIndex, err := core.indexer.GetActionIndex(h)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	sysLog, err := core.dao.TransactionLogs(actIndex.BlockHeight())
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
func (core *coreService) TransactionLogByBlockHeight(blockHeight uint64) (*iotextypes.BlockIdentifier, *iotextypes.TransactionLogs, error) {
	if !core.dao.ContainsTransactionLog() {
		return nil, nil, status.Error(codes.Unimplemented, filedao.ErrNotSupported.Error())
	}

	tip, err := core.dao.Height()
	if err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	if blockHeight < 1 || blockHeight > tip {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid block height = %d", blockHeight)
	}

	h, err := core.dao.GetBlockHash(blockHeight)
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
	sysLog, err := core.dao.TransactionLogs(blockHeight)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			// should return empty, no transaction happened in block
			return blockIdentifier, nil, nil
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	return blockIdentifier, sysLog, nil
}

// Start starts the API server
func (core *coreService) Start(ctx context.Context) error {
	if err := core.bc.AddSubscriber(core.readCache); err != nil {
		return errors.Wrap(err, "failed to add readCache")
	}
	if err := core.bc.AddSubscriber(core.chainListener); err != nil {
		return errors.Wrap(err, "failed to add chainListener")
	}
	if err := core.chainListener.Start(); err != nil {
		return errors.Wrap(err, "failed to start blockchain listener")
	}
	return nil
}

// Stop stops the API server
func (core *coreService) Stop(ctx context.Context) error {
	return core.chainListener.Stop()
}

func (core *coreService) readState(ctx context.Context, p protocol.Protocol, height string, methodName []byte, arguments ...[]byte) ([]byte, uint64, error) {
	key := ReadKey{
		Name:   p.Name(),
		Height: height,
		Method: methodName,
		Args:   arguments,
	}
	if d, ok := core.readCache.Get(key.Hash()); ok {
		var h uint64
		if height != "" {
			h, _ = strconv.ParseUint(height, 0, 64)
		}
		return d, h, nil
	}

	// TODO: need to complete the context
	tipHeight := core.bc.TipHeight()
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: tipHeight,
	})
	ctx = genesis.WithGenesisContext(
		protocol.WithRegistry(ctx, core.registry),
		core.cfg.Genesis,
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	rp := rolldpos.FindProtocol(core.registry)
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
			d, h, err := p.ReadState(ctx, factory.NewHistoryStateReader(core.sf, rp.GetEpochHeight(inputEpochNum)), methodName, arguments...)
			if err == nil {
				core.readCache.Put(key.Hash(), d)
			}
			return d, h, err
		}
	}

	// TODO: need to distinguish user error and system error
	d, h, err := p.ReadState(ctx, core.sf, methodName, arguments...)
	if err == nil {
		core.readCache.Put(key.Hash(), d)
	}
	return d, h, err
}

func (core *coreService) getActionsFromIndex(totalActions, start, count uint64) ([]*iotexapi.ActionInfo, error) {
	hashes, err := core.indexer.GetActionHashFromIndex(start, count)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	var actionInfo []*iotexapi.ActionInfo
	for i := range hashes {
		act, err := core.getAction(hash.BytesToHash256(hashes[i]), false)
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		actionInfo = append(actionInfo, act)
	}
	return actionInfo, nil
}

// Actions returns actions within the range
func (core *coreService) Actions(start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > core.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	totalActions, err := core.indexer.GetTotalActions()
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
	if core.hasActionIndex {
		return core.getActionsFromIndex(totalActions, start, count)
	}
	// Finding actions in reverse order saves time for querying most recent actions
	reverseStart := totalActions - (start + count)
	if totalActions < start+count {
		reverseStart = uint64(0)
		count = totalActions - start
	}

	var res []*iotexapi.ActionInfo
	var hit bool
	for height := core.bc.TipHeight(); height >= 1 && count > 0; height-- {
		blk, err := core.dao.GetBlockByHeight(height)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if !hit && reverseStart >= uint64(len(blk.Actions)) {
			reverseStart -= uint64(len(blk.Actions))
			continue
		}
		// now reverseStart < len(blk.Actions), we are going to fetch actions from this block
		hit = true
		act := core.reverseActionsInBlock(blk, reverseStart, count)
		res = append(act, res...)
		count -= uint64(len(act))
		reverseStart = 0
	}
	return res, nil
}

// Action returns action by action hash
func (core *coreService) Action(actionHash string, checkPending bool) (*iotexapi.ActionInfo, error) {
	actHash, err := hash.HexStringToHash256(actionHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	act, err := core.getAction(actHash, checkPending)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return act, nil
}

// ActionsByAddress returns all actions associated with an address
func (core *coreService) ActionsByAddress(addr address.Address, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > core.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	actions, err := core.indexer.GetActionsByAddress(hash.BytesToHash160(addr.Bytes()), start, count)
	if err != nil {
		if errors.Cause(err) == db.ErrBucketNotExist || errors.Cause(err) == db.ErrNotExist {
			// no actions associated with address, return nil
			return nil, nil
		}
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var res []*iotexapi.ActionInfo
	for i := range actions {
		act, err := core.getAction(hash.BytesToHash256(actions[i]), false)
		if err != nil {
			continue
		}
		res = append(res, act)
	}
	return res, nil
}

// getBlockHashByActionHash returns block hash by action hash
func (core *coreService) getBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	actIndex, err := core.indexer.GetActionIndex(h[:])
	if err != nil {
		return hash.ZeroHash256, err
	}
	return core.dao.GetBlockHash(actIndex.BlockHeight())
}

// ActionByActionHash returns action by action hash
func (core *coreService) ActionByActionHash(h hash.Hash256) (action.SealedEnvelope, hash.Hash256, uint64, uint32, error) {
	if !core.hasActionIndex || core.indexer == nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, status.Error(codes.NotFound, blockindex.ErrActionIndexNA.Error())
	}

	actIndex, err := core.indexer.GetActionIndex(h[:])
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, err
	}

	blk, err := core.dao.GetBlockByHeight(actIndex.BlockHeight())
	if err != nil {
		return action.SealedEnvelope{}, hash.ZeroHash256, 0, 0, err
	}

	selp, index, err := core.dao.GetActionByActionHash(h, actIndex.BlockHeight())
	return selp, blk.HashBlock(), actIndex.BlockHeight(), index, err
}

// UnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
func (core *coreService) UnconfirmedActionsByAddress(address string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > core.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	selps := core.ap.GetUnconfirmedActs(address)
	if len(selps) == 0 {
		return []*iotexapi.ActionInfo{}, nil
	}
	if start >= uint64(len(selps)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	var res []*iotexapi.ActionInfo
	for i := start; i < uint64(len(selps)) && i < start+count; i++ {
		if act, err := core.pendingAction(selps[i]); err == nil {
			res = append(res, act)
		}
	}
	return res, nil
}

// ActionsByBlock returns all actions in a block
func (core *coreService) ActionsByBlock(blkHash string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > core.cfg.API.RangeQueryLimit && count != math.MaxUint64 {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	blk, err := core.dao.GetBlock(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if start >= uint64(len(blk.Actions)) {
		return nil, status.Error(codes.InvalidArgument, "start exceeds the limit")
	}

	return core.actionsInBlock(blk, start, count), nil
}

// BlockMetas returns blockmetas response within the height range
func (core *coreService) BlockMetas(start uint64, count uint64) ([]*iotextypes.BlockMeta, error) {
	if count == 0 {
		return nil, status.Error(codes.InvalidArgument, "count must be greater than zero")
	}
	if count > core.cfg.API.RangeQueryLimit {
		return nil, status.Error(codes.InvalidArgument, "range exceeds the limit")
	}

	var (
		tipHeight = core.bc.TipHeight()
		res       = make([]*iotextypes.BlockMeta, 0)
	)
	if start > tipHeight {
		return nil, status.Error(codes.InvalidArgument, "start height should not exceed tip height")
	}
	for height := start; height <= tipHeight && count > 0; height++ {
		blockMeta, err := core.getBlockMetaByHeight(height)
		if err != nil {
			return nil, err
		}
		res = append(res, blockMeta)
		count--
	}
	return res, nil
}

// BlockMetaByHash returns blockmeta response by block hash
func (core *coreService) BlockMetaByHash(blkHash string) (*iotextypes.BlockMeta, error) {
	hash, err := hash.HexStringToHash256(blkHash)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	height, err := core.dao.GetBlockHeight(hash)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return core.getBlockMetaByHeight(height)
}

// getBlockMetaByHeight gets BlockMeta by height
func (core *coreService) getBlockMetaByHeight(height uint64) (*iotextypes.BlockMeta, error) {
	blk, err := core.dao.GetBlockByHeight(height)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	// get block's receipt
	if blk.Height() > 0 {
		blk.Receipts, err = core.dao.GetReceipts(height)
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
	ts, _ := ptypes.TimestampProto(header.Timestamp())
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

func (core *coreService) getGravityChainStartHeight(epochHeight uint64) (uint64, error) {
	gravityChainStartHeight := epochHeight
	if pp := poll.FindProtocol(core.registry); pp != nil {
		methodName := []byte("GetGravityChainStartHeight")
		arguments := [][]byte{[]byte(strconv.FormatUint(epochHeight, 10))}
		data, _, err := core.readState(context.Background(), pp, "", methodName, arguments...)
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

func (core *coreService) committedAction(selp action.SealedEnvelope, blkHash hash.Hash256, blkHeight uint64) (*iotexapi.ActionInfo, error) {
	actHash, err := selp.Hash()
	if err != nil {
		return nil, err
	}
	header, err := core.dao.Header(blkHash)
	if err != nil {
		return nil, err
	}
	sender := selp.SrcPubkey().Address()
	receipt, err := core.dao.GetReceiptByActionHash(actHash, blkHeight)
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

func (core *coreService) pendingAction(selp action.SealedEnvelope) (*iotexapi.ActionInfo, error) {
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

func (core *coreService) getAction(actHash hash.Hash256, checkPending bool) (*iotexapi.ActionInfo, error) {
	selp, blkHash, blkHeight, actIndex, err := core.ActionByActionHash(actHash)
	if err == nil {
		act, err := core.committedAction(selp, blkHash, blkHeight)
		if err != nil {
			return nil, err
		}
		act.Index = actIndex
		return act, nil
	}
	// Try to fetch pending action from actpool
	if checkPending {
		selp, err = core.ap.GetActionByHash(actHash)
	}
	if err != nil {
		return nil, err
	}
	return core.pendingAction(selp)
}

func (core *coreService) actionsInBlock(blk *block.Block, start, count uint64) []*iotexapi.ActionInfo {
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
		receipt, err := core.dao.GetReceiptByActionHash(actHash, blkHeight)
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

func (core *coreService) reverseActionsInBlock(blk *block.Block, reverseStart, count uint64) []*iotexapi.ActionInfo {
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
		receipt, err := core.dao.GetReceiptByActionHash(actHash, blkHeight)
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

// LogsInBlock filter logs in the block x
func (core *coreService) LogsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error) {
	logBloomFilter, err := core.bfIndexer.BlockFilterByHeight(blockNumber)
	if err != nil {
		return nil, err
	}

	if !filter.ExistInBloomFilterv2(logBloomFilter) {
		return []*iotextypes.Log{}, nil
	}

	receipts, err := core.dao.GetReceipts(blockNumber)
	if err != nil {
		return nil, err
	}

	h, err := core.dao.GetBlockHash(blockNumber)
	if err != nil {
		return nil, err
	}

	return filter.MatchLogs(receipts, h), nil
}

// LogsInRange filter logs among [start, end] blocks
func (core *coreService) LogsInRange(filter *logfilter.LogFilter, start, end, paginationSize uint64) ([]*iotextypes.Log, error) {
	start, end, err := core.correctLogsRange(start, end)
	if err != nil {
		return nil, err
	}
	// getLogs via range Blooom filter [start, end]
	blockNumbers, err := core.bfIndexer.FilterBlocksInRange(filter, start, end)
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
		logsInBlock, err := core.LogsInBlock(filter, i)
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

func (core *coreService) correctLogsRange(start, end uint64) (uint64, uint64, error) {
	if start > end {
		return 0, 0, errors.New("invalid start and end height")
	}
	if start == 0 {
		start = 1
	}
	if start > core.bc.TipHeight() {
		return 0, 0, errors.New("start block > tip height")
	}
	if end > core.bc.TipHeight() || end == 0 {
		end = core.bc.TipHeight()
	}
	return start, end, nil
}

// CalculateGasConsumption estimate gas consumption for actions except execution
func (core *coreService) CalculateGasConsumption(intrinsicGas, payloadGas, payloadSize uint64) (uint64, error) {
	return action.CalculateIntrinsicGas(intrinsicGas, payloadGas, payloadSize)
}

// EstimateExecutionGasConsumption estimate gas consumption for execution action
func (core *coreService) EstimateExecutionGasConsumption(ctx context.Context, sc *action.Execution, callerAddr address.Address) (uint64, error) {
	state, err := accountutil.AccountState(core.sf, callerAddr)
	if err != nil {
		return 0, status.Error(codes.InvalidArgument, err.Error())
	}
	sc.SetNonce(state.Nonce + 1)
	sc.SetGasPrice(big.NewInt(0))
	sc.SetGasLimit(core.cfg.Genesis.BlockGasLimit)
	enough, receipt, err := core.isGasLimitEnough(ctx, callerAddr, sc)
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
	enough, _, err = core.isGasLimitEnough(ctx, callerAddr, sc)
	if err != nil && err != action.ErrInsufficientFunds {
		return 0, status.Error(codes.Internal, err.Error())
	}
	if !enough {
		low, high := estimatedGas, core.cfg.Genesis.BlockGasLimit
		estimatedGas = high
		for low <= high {
			mid := (low + high) / 2
			sc.SetGasLimit(mid)
			enough, _, err = core.isGasLimitEnough(ctx, callerAddr, sc)
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

func (core *coreService) isGasLimitEnough(
	ctx context.Context,
	caller address.Address,
	sc *action.Execution,
) (bool, *action.Receipt, error) {
	ctx, span := tracer.NewSpan(ctx, "Server.isGasLimitEnough")
	defer span.End()
	ctx, err := core.bc.Context(ctx)
	if err != nil {
		return false, nil, err
	}
	_, receipt, err := core.sf.SimulateExecution(ctx, caller, sc, core.dao.GetBlockHash)
	if err != nil {
		return false, nil, err
	}
	return receipt.Status == uint64(iotextypes.ReceiptStatus_Success), receipt, nil
}

func (core *coreService) getProductivityByEpoch(
	rp *rolldpos.Protocol,
	epochNum uint64,
	tipHeight uint64,
	abps state.CandidateList,
) (uint64, map[string]uint64, error) {
	num, produce, err := rp.ProductivityByEpoch(epochNum, tipHeight, func(start uint64, end uint64) (map[string]uint64, error) {
		return blockchain.Productivity(core.bc, start, end)
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

func (core *coreService) getProtocolAccount(ctx context.Context, addr string) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	span := tracer.SpanFromContext(ctx)
	defer span.End()
	var (
		balance string
		out     *iotexapi.ReadStateResponse
		err     error
	)
	switch addr {
	case address.RewardingPoolAddr:
		if out, err = core.ReadState("rewarding", "", []byte("TotalBalance"), nil); err != nil {
			return nil, nil, err
		}
		val, ok := big.NewInt(0).SetString(string(out.GetData()), 10)
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
		if out, err = core.ReadState("staking", "", methodName, [][]byte{arg}); err != nil {
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
func (core *coreService) ActPoolActions(actHashes []string) ([]*iotextypes.Action, error) {
	var ret []*iotextypes.Action
	if len(actHashes) == 0 {
		for _, sealeds := range core.ap.PendingActionMap() {
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
		sealed, err := core.ap.GetActionByHash(hs)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		ret = append(ret, sealed.Proto())
	}
	return ret, nil
}

// EVMNetworkID returns the network id of evm
func (core *coreService) EVMNetworkID() uint32 {
	return config.EVMNetworkID()
}

// ChainID returns the chain id of evm
func (core *coreService) ChainID() uint32 {
	return core.bc.ChainID()
}

// ReadContractStorage reads contract's storage
func (core *coreService) ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error) {
	ctx, err := core.bc.Context(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return core.sf.ReadContractStorage(ctx, addr, key)
}

// BlockChain returns the member bc
func (core *coreService) BlockChain() blockchain.Blockchain {
	return core.bc
}

// StateFactory return the member sf
func (core *coreService) StateFactory() factory.Factory {
	return core.sf
}

// BlockDao return the member dao
func (core *coreService) BlockDao() blockdao.BlockDAO {
	return core.dao
}

// Indexer return the member indexer
func (core *coreService) Indexer() blockindex.Indexer {
	return core.indexer
}

// ActPool return the member ap
func (core *coreService) ActPool() actpool.ActPool {
	return core.ap
}

// Config return the member cfg
func (core *coreService) Config() config.Config {
	return core.cfg
}

// Registry return the member registry
func (core *coreService) Registry() *protocol.Registry {
	return core.registry
}

// HasActionIndex return the member hasActionIndex
func (core *coreService) HasActionIndex() bool {
	return core.hasActionIndex
}
