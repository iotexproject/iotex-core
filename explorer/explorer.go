// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("invalid transfer")
	// ErrVote indicates the error of vote
	ErrVote = errors.New("invalid vote")
	// ErrExecution indicates the error of execution
	ErrExecution = errors.New("invalid execution")
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

const (
	trueStr  = "ture"
	falseStr = "false"
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
	c                  consensus.Consensus
	dp                 dispatcher.Dispatcher
	ap                 actpool.ActPool
	gs                 GasStation
	broadcastHandler   BroadcastOutbound
	neighborsHandler   Neighbors
	networkInfoHandler NetworkInfo
	cfg                config.Explorer
	idx                *indexservice.Server
	// TODO: the way to make explorer to access the data model managed by main-chain protocol is hack. We need to
	// refactor the code later
	mainChain *mainchain.Protocol
}

// SetMainChainProtocol sets the main-chain side multi-chain protocol
func (exp *Service) SetMainChainProtocol(mainChain *mainchain.Protocol) { exp.mainChain = mainChain }

// GetBlockchainHeight returns the current blockchain tip height
func (exp *Service) GetBlockchainHeight() (int64, error) {
	tip := exp.bc.TipHeight()
	return int64(tip), nil
}

// GetAddressBalance returns the balance of an address
func (exp *Service) GetAddressBalance(address string) (string, error) {
	state, err := exp.bc.StateByAddr(address)
	if err != nil {
		return "", err
	}
	return state.Balance.String(), nil
}

// GetAddressDetails returns the properties of an address
func (exp *Service) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	state, err := exp.bc.StateByAddr(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	pendingNonce, err := exp.ap.GetPendingNonce(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	details := explorer.AddressDetails{
		Address:      address,
		TotalBalance: state.Balance.String(),
		Nonce:        int64(state.Nonce),
		PendingNonce: int64(pendingNonce),
		IsCandidate:  state.IsCandidate,
	}

	return details, nil
}

// GetReceiptByExecutionID gets receipt with corresponding execution id
// Deprecated
func (exp *Service) GetReceiptByExecutionID(id string) (explorer.Receipt, error) {
	return exp.GetReceiptByActionID(id)
}

// GetReceiptByActionID gets receipt with corresponding action id
func (exp *Service) GetReceiptByActionID(id string) (explorer.Receipt, error) {
	bytes, err := hex.DecodeString(id)
	if err != nil {
		return explorer.Receipt{}, err
	}
	var actionHash hash.Hash256
	copy(actionHash[:], bytes)

	// get receipt from boltdb
	if !exp.cfg.UseIndexer {
		receipt, err := exp.bc.GetReceiptByActionHash(actionHash)
		if err != nil {
			return explorer.Receipt{}, err
		}
		return convertReceiptToExplorerReceipt(receipt)
	}

	// get receipt from indexer
	blkHash, err := exp.idx.Indexer().GetBlockByIndex(config.IndexReceipt, actionHash)
	if err != nil {
		return explorer.Receipt{}, err
	}
	blk, err := exp.bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorer.Receipt{}, err
	}

	for _, receipt := range blk.Receipts {
		if receipt.Hash() == actionHash {
			return convertReceiptToExplorerReceipt(receipt)
		}
	}
	return explorer.Receipt{}, err
}

// GetCreateDeposit gets create deposit by ID
func (exp *Service) GetCreateDeposit(createDepositID string) (explorer.CreateDeposit, error) {
	bytes, err := hex.DecodeString(createDepositID)
	if err != nil {
		return explorer.CreateDeposit{}, err
	}
	var createDepositHash hash.Hash256
	copy(createDepositHash[:], bytes)
	return getCreateDeposit(exp.bc, exp.ap, createDepositHash)
}

// GetCreateDepositsByAddress gets the relevant create deposits of an address
func (exp *Service) GetCreateDepositsByAddress(
	address string,
	offset int64,
	limit int64,
) ([]explorer.CreateDeposit, error) {
	res := make([]explorer.CreateDeposit, 0)

	depositsFromAddress, err := exp.bc.GetActionsFromAddress(address)
	if err != nil {
		return []explorer.CreateDeposit{}, err
	}

	for i, depositHash := range depositsFromAddress {
		if int64(i) < offset {
			continue
		}
		if int64(len(res)) >= limit {
			break
		}
		createDeposit, err := getCreateDeposit(exp.bc, exp.ap, depositHash)
		if err != nil {
			continue
		}

		res = append(res, createDeposit)
	}

	return res, nil
}

// GetSettleDeposit gets settle deposit by ID
func (exp *Service) GetSettleDeposit(settleDepositID string) (explorer.SettleDeposit, error) {
	bytes, err := hex.DecodeString(settleDepositID)
	if err != nil {
		return explorer.SettleDeposit{}, err
	}
	var settleDepositHash hash.Hash256
	copy(settleDepositHash[:], bytes)
	return getSettleDeposit(exp.bc, exp.ap, settleDepositHash)
}

// GetSettleDepositsByAddress gets the relevant settle deposits of an address
func (exp *Service) GetSettleDepositsByAddress(
	address string,
	offset int64,
	limit int64,
) ([]explorer.SettleDeposit, error) {
	res := make([]explorer.SettleDeposit, 0)

	depositsToAddress, err := exp.bc.GetActionsToAddress(address)
	if err != nil {
		return []explorer.SettleDeposit{}, err
	}

	for i, depositHash := range depositsToAddress {
		if int64(i) < offset {
			continue
		}
		if int64(len(res)) >= limit {
			break
		}
		settleDeposit, err := getSettleDeposit(exp.bc, exp.ap, depositHash)
		if err != nil {
			continue
		}

		res = append(res, settleDeposit)
	}

	return res, nil
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *Service) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var res []explorer.Block

	for height := offset; height > 0 && int64(len(res)) < limit; height-- {
		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return []explorer.Block{}, err
		}

		blockHeaderPb := blk.ConvertToBlockHeaderPb()
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return []explorer.Block{}, err
		}

		transfers, votes, executions := action.ClassifyActions(blk.Actions)
		totalAmount := big.NewInt(0)
		totalSize := uint32(0)
		for _, transfer := range transfers {
			totalAmount.Add(totalAmount, transfer.Amount())
			totalSize += transfer.TotalSize()
		}

		txRoot := blk.TxRoot()
		deltaStateDigest := blk.DeltaStateDigest()
		explorerBlock := explorer.Block{
			ID:         hex.EncodeToString(hash[:]),
			Height:     int64(blockHeaderPb.GetCore().Height),
			Timestamp:  blockHeaderPb.GetCore().GetTimestamp().GetSeconds(),
			Transfers:  int64(len(transfers)),
			Votes:      int64(len(votes)),
			Executions: int64(len(executions)),
			Amount:     totalAmount.String(),
			Size:       int64(totalSize),
			GenerateBy: explorer.BlockGenerator{
				Name:    "",
				Address: blk.PublicKey().HexString(),
			},
			TxRoot:           hex.EncodeToString(txRoot[:]),
			DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
		}

		res = append(res, explorerBlock)
	}

	return res, nil
}

// GetBlockByID returns block by block id
func (exp *Service) GetBlockByID(blkID string) (explorer.Block, error) {
	bytes, err := hex.DecodeString(blkID)
	if err != nil {
		return explorer.Block{}, err
	}
	var hash hash.Hash256
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return explorer.Block{}, err
	}

	blkHeaderPb := blk.ConvertToBlockHeaderPb()

	transfers, votes, executions := action.ClassifyActions(blk.Actions)
	totalAmount := big.NewInt(0)
	totalSize := uint32(0)
	for _, transfer := range transfers {
		totalAmount.Add(totalAmount, transfer.Amount())
		totalSize += transfer.TotalSize()
	}

	txRoot := blk.TxRoot()
	deltaStateDigest := blk.DeltaStateDigest()
	explorerBlock := explorer.Block{
		ID:         blkID,
		Height:     int64(blkHeaderPb.GetCore().Height),
		Timestamp:  blkHeaderPb.GetCore().GetTimestamp().GetSeconds(),
		Transfers:  int64(len(transfers)),
		Votes:      int64(len(votes)),
		Executions: int64(len(executions)),
		Amount:     totalAmount.String(),
		Size:       int64(totalSize),
		GenerateBy: explorer.BlockGenerator{
			Name:    "",
			Address: blk.PublicKey().HexString(),
		},
		TxRoot:           hex.EncodeToString(txRoot[:]),
		DeltaStateDigest: hex.EncodeToString(deltaStateDigest[:]),
	}

	return explorerBlock, nil
}

// GetCoinStatistic returns stats in blockchain
func (exp *Service) GetCoinStatistic() (explorer.CoinStatistic, error) {
	stat := explorer.CoinStatistic{}

	tipHeight := exp.bc.TipHeight()
	blockLimit := int64(exp.cfg.TpsWindow)
	if blockLimit <= 0 {
		return stat, errors.Wrapf(ErrInternalServer, "block limit is %d", blockLimit)
	}

	// avoid genesis block
	if int64(tipHeight) < blockLimit {
		blockLimit = int64(tipHeight)
	}
	blks, err := exp.GetLastBlocksByRange(int64(tipHeight), blockLimit)
	if err != nil {
		return stat, err
	}

	if len(blks) == 0 {
		return stat, errors.New("get 0 blocks! not able to calculate aps")
	}

	timeDuration := blks[0].Timestamp - blks[len(blks)-1].Timestamp
	// if time duration is less than 1 second, we set it to be 1 second
	if timeDuration == 0 {
		timeDuration = 1
	}
	actionNumber := int64(0)
	for _, blk := range blks {
		actionNumber += blk.Transfers + blk.Votes + blk.Executions
	}
	aps := actionNumber / timeDuration

	explorerCoinStats := explorer.CoinStatistic{
		Height:     int64(tipHeight),
		Transfers:  0,
		Votes:      0,
		Executions: 0,
		Aps:        aps,
	}
	return explorerCoinStats, nil
}

// GetConsensusMetrics returns the latest consensus metrics
func (exp *Service) GetConsensusMetrics() (explorer.ConsensusMetrics, error) {
	cm, err := exp.c.Metrics()
	if err != nil {
		return explorer.ConsensusMetrics{}, err
	}
	dStrs := make([]string, len(cm.LatestDelegates))
	copy(dStrs, cm.LatestDelegates)
	var bpStr string
	if cm.LatestBlockProducer != "" {
		bpStr = cm.LatestBlockProducer
	}
	cStrs := make([]string, len(cm.Candidates))
	copy(cStrs, cm.Candidates)
	return explorer.ConsensusMetrics{
		LatestEpoch:         int64(cm.LatestEpoch),
		LatestDelegates:     dStrs,
		LatestBlockProducer: bpStr,
		Candidates:          cStrs,
	}, nil
}

// GetCandidateMetrics returns the latest delegates metrics
func (exp *Service) GetCandidateMetrics() (explorer.CandidateMetrics, error) {
	cm, err := exp.c.Metrics()
	if err != nil {
		return explorer.CandidateMetrics{}, errors.Wrapf(
			err,
			"Failed to get the candidate metrics")
	}
	delegateSet := make(map[string]bool, len(cm.LatestDelegates))
	for _, d := range cm.LatestDelegates {
		delegateSet[d] = true
	}
	allCandidates, err := exp.bc.CandidatesByHeight(cm.LatestHeight)
	if err != nil {
		return explorer.CandidateMetrics{}, errors.Wrapf(err,
			"Failed to get the candidate metrics")
	}
	candidates := make([]explorer.Candidate, len(cm.Candidates))
	for i, c := range allCandidates {
		candidates[i] = explorer.Candidate{
			Address:    c.Address,
			TotalVote:  c.Votes.String(),
			IsDelegate: false,
			IsProducer: false,
		}
		if _, ok := delegateSet[c.Address]; ok {
			candidates[i].IsDelegate = true
		}
		if cm.LatestBlockProducer == c.Address {
			candidates[i].IsProducer = true
		}
	}

	return explorer.CandidateMetrics{
		Candidates:   candidates,
		LatestEpoch:  int64(cm.LatestEpoch),
		LatestHeight: int64(cm.LatestHeight),
	}, nil
}

// GetCandidateMetricsByHeight returns the candidates metrics for given height.
func (exp *Service) GetCandidateMetricsByHeight(h int64) (explorer.CandidateMetrics, error) {
	if h < 0 {
		return explorer.CandidateMetrics{}, errors.New("Invalid height")
	}
	allCandidates, err := exp.bc.CandidatesByHeight(uint64(h))
	if err != nil {
		return explorer.CandidateMetrics{}, errors.Wrapf(err,
			"Failed to get the candidate metrics")
	}
	candidates := make([]explorer.Candidate, 0, len(allCandidates))
	for _, c := range allCandidates {
		candidates = append(candidates, explorer.Candidate{
			Address:   c.Address,
			TotalVote: c.Votes.String(),
		})
	}

	return explorer.CandidateMetrics{
		Candidates: candidates,
	}, nil
}

// SendTransfer sends a transfer
func (exp *Service) SendTransfer(tsfJSON explorer.SendTransferRequest) (resp explorer.SendTransferResponse, err error) {
	log.L().Debug("receive send transfer request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("SendTransfer", succeed).Inc()
	}()

	actPb, err := convertExplorerTransferToActionPb(&tsfJSON, exp.cfg.MaxTransferPayloadBytes)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	// broadcast to the network
	if err = exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendTransferResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	tsf := &action.SealedEnvelope{}
	if err := tsf.LoadProto(actPb); err != nil {
		return explorer.SendTransferResponse{}, err
	}
	h := tsf.Hash()
	return explorer.SendTransferResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// SendVote sends a vote
func (exp *Service) SendVote(voteJSON explorer.SendVoteRequest) (resp explorer.SendVoteResponse, err error) {
	log.L().Debug("receive send vote request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("SendVote", succeed).Inc()
	}()

	selfPubKey, err := keypair.StringToPubKeyBytes(voteJSON.VoterPubKey)
	if err != nil {
		return explorer.SendVoteResponse{}, err
	}
	signature, err := hex.DecodeString(voteJSON.Signature)
	if err != nil {
		return explorer.SendVoteResponse{}, err
	}
	gasPrice, ok := big.NewInt(0).SetString(voteJSON.GasPrice, 10)
	if !ok {
		return explorer.SendVoteResponse{}, errors.New("failed to set vote gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Vote{
				Vote: &iotextypes.Vote{
					VoteeAddress: voteJSON.Votee,
				},
			},
			Version:  uint32(voteJSON.Version),
			Nonce:    uint64(voteJSON.Nonce),
			GasLimit: uint64(voteJSON.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: selfPubKey,
		Signature:    signature,
	}
	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendVoteResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	v := &action.SealedEnvelope{}
	if err := v.LoadProto(actPb); err != nil {
		return explorer.SendVoteResponse{}, err
	}
	h := v.Hash()
	return explorer.SendVoteResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// PutSubChainBlock put block merkel root on root chain.
func (exp *Service) PutSubChainBlock(putBlockJSON explorer.PutSubChainBlockRequest) (resp explorer.PutSubChainBlockResponse, err error) {
	log.L().Debug("receive put block request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("PutBlock", succeed).Inc()
	}()

	senderPubKey, err := keypair.StringToPubKeyBytes(putBlockJSON.SenderPubKey)
	if err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	signature, err := hex.DecodeString(putBlockJSON.Signature)
	if err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	gasPrice, ok := big.NewInt(0).SetString(putBlockJSON.GasPrice, 10)
	if !ok {
		return explorer.PutSubChainBlockResponse{}, errors.New("failed to set vote gas price")
	}

	roots := make([]*iotextypes.MerkleRoot, 0)
	for _, mr := range putBlockJSON.Roots {
		v, err := hex.DecodeString(mr.Value)
		if err != nil {
			return explorer.PutSubChainBlockResponse{}, err
		}
		roots = append(roots, &iotextypes.MerkleRoot{
			Name:  mr.Name,
			Value: v,
		})
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_PutBlock{
				PutBlock: &iotextypes.PutBlock{
					SubChainAddress: putBlockJSON.SubChainAddress,
					Height:          uint64(putBlockJSON.Height),
					Roots:           roots,
				},
			},
			Version:  uint32(putBlockJSON.Version),
			Nonce:    uint64(putBlockJSON.Nonce),
			GasLimit: uint64(putBlockJSON.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}
	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	v := &action.SealedEnvelope{}
	if err := v.LoadProto(actPb); err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	h := v.Hash()
	return explorer.PutSubChainBlockResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// SendAction is the API to send an action to blockchain.
func (exp *Service) SendAction(req explorer.SendActionRequest) (resp explorer.SendActionResponse, err error) {
	log.L().Debug("receive send action request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("SendAction", succeed).Inc()
	}()
	var action iotextypes.Action

	if err := jsonpb.UnmarshalString(req.Payload, &action); err != nil {
		return explorer.SendActionResponse{}, err
	}

	// broadcast to the network
	if err = exp.broadcastHandler(context.Background(), exp.bc.ChainID(), &action); err != nil {
		log.L().Warn("Failed to broadcast SendAction request.", zap.Error(err))
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), &action)

	// TODO: include action hash
	return explorer.SendActionResponse{}, nil
}

// GetPeers return a list of node peers and itself's network addsress info.
func (exp *Service) GetPeers() (explorer.GetPeersResponse, error) {
	exppeers := make([]explorer.Node, 0)
	ctx := context.Background()
	peers, err := exp.neighborsHandler(ctx)
	if err != nil {
		return explorer.GetPeersResponse{}, err
	}
	for _, p := range peers {
		exppeers = append(exppeers, explorer.Node{
			Address: fmt.Sprintf("%v", p),
		})
	}
	return explorer.GetPeersResponse{
		Self:  explorer.Node{Address: fmt.Sprintf("%v", exp.networkInfoHandler())},
		Peers: exppeers,
	}, nil
}

// SendSmartContract sends a smart contract
func (exp *Service) SendSmartContract(execution explorer.Execution) (resp explorer.SendSmartContractResponse, err error) {
	log.L().Debug("receive send smart contract request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("SendSmartContract", succeed).Inc()
	}()

	executorPubKey, err := keypair.StringToPubKeyBytes(execution.ExecutorPubKey)
	if err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	data, err := hex.DecodeString(execution.Data)
	if err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	signature, err := hex.DecodeString(execution.Signature)
	if err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	amount, ok := big.NewInt(0).SetString(execution.Amount, 10)
	if !ok {
		return explorer.SendSmartContractResponse{}, errors.New("failed to set execution amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(execution.GasPrice, 10)
	if !ok {
		return explorer.SendSmartContractResponse{}, errors.New("failed to set execution gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Execution{
				Execution: &iotextypes.Execution{
					Amount:   amount.String(),
					Contract: execution.Contract,
					Data:     data,
				},
			},
			Version:  uint32(execution.Version),
			Nonce:    uint64(execution.Nonce),
			GasLimit: uint64(execution.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: executorPubKey,
		Signature:    signature,
	}
	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	sc := &action.SealedEnvelope{}
	if err := sc.LoadProto(actPb); err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	h := sc.Hash()
	return explorer.SendSmartContractResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// ReadExecutionState reads the state in a contract address specified by the slot
func (exp *Service) ReadExecutionState(execution explorer.Execution) (string, error) {
	log.L().Debug("receive read smart contract request")

	actPb, err := convertExplorerExecutionToActionPb(&execution)
	if err != nil {
		return "", err
	}
	selp := &action.SealedEnvelope{}
	if err := selp.LoadProto(actPb); err != nil {
		return "", err
	}
	sc, ok := selp.Action().(*action.Execution)
	if !ok {
		return "", errors.New("not execution")
	}

	callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
	if err != nil {
		return "", err
	}
	res, err := exp.bc.ExecuteContractRead(callerAddr, sc)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(res.ReturnValue), nil
}

// GetBlockOrActionByHash get block or action by a hash
func (exp *Service) GetBlockOrActionByHash(hashStr string) (explorer.GetBlkOrActResponse, error) {
	if blk, err := exp.GetBlockByID(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Block: &blk}, nil
	}
	if addr, err := exp.GetAddressDetails(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Address: &addr}, nil
	}

	return explorer.GetBlkOrActResponse{}, nil
}

// CreateDeposit deposits balance from main-chain to sub-chain
func (exp *Service) CreateDeposit(req explorer.CreateDepositRequest) (res explorer.CreateDepositResponse, err error) {
	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("createDeposit", succeed).Inc()
	}()

	senderPubKey, err := keypair.StringToPubKeyBytes(req.SenderPubKey)
	if err != nil {
		return res, err
	}
	signature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return res, err
	}
	amount, ok := big.NewInt(0).SetString(req.Amount, 10)
	if !ok {
		return res, errors.New("error when converting amount string into big int type")
	}
	gasPrice, ok := big.NewInt(0).SetString(req.GasPrice, 10)
	if !ok {
		return res, errors.New("error when converting gas price string into big int type")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_CreateDeposit{
				CreateDeposit: &iotextypes.CreateDeposit{
					ChainID:   uint32(req.ChainID),
					Amount:    amount.String(),
					Recipient: req.Recipient,
				},
			},
			Version:  uint32(req.Version),
			Nonce:    uint64(req.Nonce),
			GasLimit: uint64(req.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}

	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return res, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	selp := &action.SealedEnvelope{}
	if err := selp.LoadProto(actPb); err != nil {
		return res, err
	}
	h := selp.Hash()
	return explorer.CreateDepositResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// GetDeposits returns the deposits of a sub-chain in the given range in descending order by the index
func (exp *Service) GetDeposits(subChainID int64, offset int64, limit int64) ([]explorer.Deposit, error) {
	subChainsInOp, err := exp.mainChain.SubChainsInOperation()
	if err != nil {
		return nil, err
	}
	var targetSubChain mainchain.InOperation
	for _, subChainInOp := range subChainsInOp {
		if subChainInOp.ID == uint32(subChainID) {
			targetSubChain = subChainInOp
		}
	}
	if targetSubChain.ID != uint32(subChainID) {
		return nil, errors.Errorf("sub-chain %d is not found in operation", subChainID)
	}
	subChainAddr, err := address.FromBytes(targetSubChain.Addr)
	if err != nil {
		return nil, err
	}
	subChain, err := exp.mainChain.SubChain(subChainAddr)
	if err != nil {
		return nil, err
	}
	idx := uint64(offset)
	// If the last deposit index is lower than the start index, reset it
	if subChain.DepositCount-1 < idx {
		idx = subChain.DepositCount - 1
	}
	var deposits []explorer.Deposit
	for count := int64(0); count < limit; count++ {
		deposit, err := exp.mainChain.Deposit(subChainAddr, idx)
		if err != nil {
			return nil, err
		}
		recipient, err := address.FromBytes(deposit.Addr)
		if err != nil {
			return nil, err
		}
		deposits = append(deposits, explorer.Deposit{
			Amount:    deposit.Amount.String(),
			Address:   recipient.String(),
			Confirmed: deposit.Confirmed,
		})
		if idx > 0 {
			idx--
		} else {
			break
		}
	}
	return deposits, nil
}

// SettleDeposit settles deposit on sub-chain
func (exp *Service) SettleDeposit(req explorer.SettleDepositRequest) (res explorer.SettleDepositResponse, err error) {
	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("settleDeposit", succeed).Inc()
	}()

	senderPubKey, err := keypair.StringToPubKeyBytes(req.SenderPubKey)
	if err != nil {
		return res, err
	}
	signature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return res, err
	}
	amount, ok := big.NewInt(0).SetString(req.Amount, 10)
	if !ok {
		return res, errors.New("error when converting amount string into big int type")
	}
	gasPrice, ok := big.NewInt(0).SetString(req.GasPrice, 10)
	if !ok {
		return res, errors.New("error when converting gas price string into big int type")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_SettleDeposit{
				SettleDeposit: &iotextypes.SettleDeposit{
					Amount:    amount.String(),
					Index:     uint64(req.Index),
					Recipient: req.Recipient,
				},
			},
			Version:  uint32(req.Version),
			Nonce:    uint64(req.Nonce),
			GasLimit: uint64(req.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}
	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return res, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	deposit := &action.SealedEnvelope{}
	if err := deposit.LoadProto(actPb); err != nil {
		return res, err
	}
	h := deposit.Hash()
	return explorer.SettleDepositResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// SuggestGasPrice suggest gas price
func (exp *Service) SuggestGasPrice() (int64, error) {
	return exp.gs.suggestGasPrice()
}

// EstimateGasForTransfer estimate gas for transfer
func (exp *Service) EstimateGasForTransfer(tsfJSON explorer.SendTransferRequest) (int64, error) {
	return exp.gs.estimateGasForTransfer(tsfJSON)
}

// EstimateGasForVote suggest gas for vote
func (exp *Service) EstimateGasForVote() (int64, error) {
	return exp.gs.estimateGasForVote()
}

// EstimateGasForSmartContract suggest gas for smart contract
func (exp *Service) EstimateGasForSmartContract(execution explorer.Execution) (int64, error) {
	return exp.gs.estimateGasForSmartContract(execution)
}

// GetStateRootHash gets the state root hash of a given block height
func (exp *Service) GetStateRootHash(blockHeight int64) (string, error) {
	rootHash, err := exp.bc.GetFactory().RootHashByHeight(uint64(blockHeight))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(rootHash[:]), nil
}

// getCreateDeposit takes in a blockchain and create deposit hash and returns an Explorer create deposit
func getCreateDeposit(
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	createDepositHash hash.Hash256,
) (explorer.CreateDeposit, error) {
	pending := false
	var selp action.SealedEnvelope
	var err error
	selp, err = bc.GetActionByActionHash(createDepositHash)
	if err != nil {
		// Try to fetch pending create deposit from actpool
		selp, err = ap.GetActionByHash(createDepositHash)
		if err != nil {
			return explorer.CreateDeposit{}, err
		}
		pending = true
	}

	// Fetch from block
	blkHash, err := bc.GetBlockHashByActionHash(createDepositHash)
	if err != nil {
		return explorer.CreateDeposit{}, err
	}
	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorer.CreateDeposit{}, err
	}

	cd, err := castActionToCreateDeposit(selp, pending)
	if err != nil {
		return explorer.CreateDeposit{}, err
	}
	cd.Timestamp = blk.ConvertToBlockHeaderPb().GetCore().GetTimestamp().GetSeconds()
	cd.BlockID = hex.EncodeToString(blkHash[:])
	return cd, nil
}

func castActionToCreateDeposit(selp action.SealedEnvelope, pending bool) (explorer.CreateDeposit, error) {
	cd, ok := selp.Action().(*action.CreateDeposit)
	if !ok {
		return explorer.CreateDeposit{}, errors.Wrap(ErrAction, "action type is not create deposit")
	}
	hash := selp.Hash()
	createDeposit := explorer.CreateDeposit{
		Nonce:     int64(selp.Nonce()),
		ID:        hex.EncodeToString(hash[:]),
		Recipient: cd.Recipient(),
		Fee:       "", // TODO: we need to get the actual fee.
		GasLimit:  int64(selp.GasLimit()),
		IsPending: pending,
	}
	if cd.Amount() != nil && len(cd.Amount().String()) > 0 {
		createDeposit.Amount = cd.Amount().String()
	}
	if selp.GasPrice() != nil && len(selp.GasPrice().String()) > 0 {
		createDeposit.GasPrice = selp.GasPrice().String()
	}
	return createDeposit, nil
}

// getSettleDeposit takes in a blockchain and settle deposit hash and returns an Explorer settle deposit
func getSettleDeposit(
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	settleDepositHash hash.Hash256,
) (explorer.SettleDeposit, error) {
	pending := false
	var selp action.SealedEnvelope
	var err error
	selp, err = bc.GetActionByActionHash(settleDepositHash)
	if err != nil {
		// Try to fetch pending settle deposit from actpool
		selp, err = ap.GetActionByHash(settleDepositHash)
		if err != nil {
			return explorer.SettleDeposit{}, err
		}
		pending = true
	}

	// Fetch from block
	blkHash, err := bc.GetBlockHashByActionHash(settleDepositHash)
	if err != nil {
		return explorer.SettleDeposit{}, err
	}
	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorer.SettleDeposit{}, err
	}

	sd, err := castActionToSettleDeposit(selp, pending)
	if err != nil {
		return explorer.SettleDeposit{}, err
	}
	sd.Timestamp = blk.ConvertToBlockHeaderPb().GetCore().GetTimestamp().GetSeconds()
	sd.BlockID = hex.EncodeToString(blkHash[:])
	return sd, nil
}

func castActionToSettleDeposit(selp action.SealedEnvelope, pending bool) (explorer.SettleDeposit, error) {
	sd, ok := selp.Action().(*action.SettleDeposit)
	if !ok {
		return explorer.SettleDeposit{}, errors.Wrap(ErrAction, "action type is not settle deposit")
	}
	hash := selp.Hash()
	settleDeposit := explorer.SettleDeposit{
		Nonce:     int64(selp.Nonce()),
		ID:        hex.EncodeToString(hash[:]),
		Recipient: sd.Recipient(),
		Index:     int64(sd.Index()),
		Fee:       "", // TODO: we need to get the actual fee.
		GasLimit:  int64(selp.GasLimit()),
		IsPending: pending,
	}
	if sd.Amount() != nil && len(sd.Amount().String()) > 0 {
		settleDeposit.Amount = sd.Amount().String()
	}
	if selp.GasPrice() != nil && len(selp.GasPrice().String()) > 0 {
		settleDeposit.GasPrice = selp.GasPrice().String()
	}
	return settleDeposit, nil
}

func convertTsfToExplorerTsf(selp action.SealedEnvelope, isPending bool) (explorer.Transfer, error) {
	transfer, ok := selp.Action().(*action.Transfer)
	if !ok {
		return explorer.Transfer{}, errors.Wrap(ErrTransfer, "action is not transfer")
	}

	if transfer == nil {
		return explorer.Transfer{}, errors.Wrap(ErrTransfer, "transfer cannot be nil")
	}
	hash := selp.Hash()
	explorerTransfer := explorer.Transfer{
		Nonce:      int64(selp.Nonce()),
		ID:         hex.EncodeToString(hash[:]),
		Recipient:  transfer.Recipient(),
		Fee:        "", // TODO: we need to get the actual fee.
		Payload:    hex.EncodeToString(transfer.Payload()),
		GasLimit:   int64(selp.GasLimit()),
		IsCoinbase: false,
		IsPending:  isPending,
	}
	if transfer.Amount() != nil && len(transfer.Amount().String()) > 0 {
		explorerTransfer.Amount = transfer.Amount().String()
	}
	if selp.GasPrice() != nil && len(selp.GasPrice().String()) > 0 {
		explorerTransfer.GasPrice = selp.GasPrice().String()
	}
	return explorerTransfer, nil
}

func convertVoteToExplorerVote(selp action.SealedEnvelope, isPending bool) (explorer.Vote, error) {
	vote, ok := selp.Action().(*action.Vote)
	if !ok {
		return explorer.Vote{}, errors.Wrap(ErrTransfer, "action is not vote")
	}
	if vote == nil {
		return explorer.Vote{}, errors.Wrap(ErrVote, "vote cannot be nil")
	}
	hash := selp.Hash()
	voterPubkey := vote.VoterPublicKey()
	explorerVote := explorer.Vote{
		ID:          hex.EncodeToString(hash[:]),
		Nonce:       int64(selp.Nonce()),
		VoterPubKey: voterPubkey.HexString(),
		Votee:       vote.Votee(),
		GasLimit:    int64(selp.GasLimit()),
		GasPrice:    selp.GasPrice().String(),
		IsPending:   isPending,
	}
	return explorerVote, nil
}

func convertExecutionToExplorerExecution(selp action.SealedEnvelope, isPending bool) (explorer.Execution, error) {
	execution, ok := selp.Action().(*action.Execution)
	if !ok {
		return explorer.Execution{}, errors.Wrap(ErrTransfer, "action is not execution")
	}
	if execution == nil {
		return explorer.Execution{}, errors.Wrap(ErrExecution, "execution cannot be nil")
	}
	hash := execution.Hash()
	explorerExecution := explorer.Execution{
		Nonce:     int64(selp.Nonce()),
		ID:        hex.EncodeToString(hash[:]),
		Contract:  execution.Contract(),
		GasLimit:  int64(selp.GasLimit()),
		Data:      hex.EncodeToString(execution.Data()),
		IsPending: isPending,
	}
	if execution.Amount() != nil && len(execution.Amount().String()) > 0 {
		explorerExecution.Amount = execution.Amount().String()
	}
	if selp.GasPrice() != nil && len(selp.GasPrice().String()) > 0 {
		explorerExecution.GasPrice = selp.GasPrice().String()
	}
	return explorerExecution, nil
}

func convertReceiptToExplorerReceipt(receipt *action.Receipt) (explorer.Receipt, error) {
	if receipt == nil {
		return explorer.Receipt{}, errors.Wrap(ErrReceipt, "receipt cannot be nil")
	}
	logs := []explorer.Log{}
	for _, log := range receipt.Logs {
		topics := []string{}
		for _, topic := range log.Topics {
			topics = append(topics, hex.EncodeToString(topic[:]))
		}
		logs = append(logs, explorer.Log{
			Address:     log.Address,
			Topics:      topics,
			Data:        hex.EncodeToString(log.Data),
			BlockNumber: int64(log.BlockNumber),
			TxnHash:     hex.EncodeToString(log.TxnHash[:]),
			Index:       int64(log.Index),
		})
	}

	return explorer.Receipt{
		ReturnValue:     hex.EncodeToString(receipt.ReturnValue),
		Status:          int64(receipt.Status),
		Hash:            hex.EncodeToString(receipt.ActHash[:]),
		GasConsumed:     int64(receipt.GasConsumed),
		ContractAddress: receipt.ContractAddress,
		Logs:            logs,
	}, nil
}

func convertExplorerExecutionToActionPb(execution *explorer.Execution) (*iotextypes.Action, error) {
	executorPubKey, err := keypair.StringToPubKeyBytes(execution.ExecutorPubKey)
	if err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(execution.Data)
	if err != nil {
		return nil, err
	}
	signature, err := hex.DecodeString(execution.Signature)
	if err != nil {
		return nil, err
	}
	amount, ok := big.NewInt(0).SetString(execution.Amount, 10)
	if !ok {
		return nil, errors.New("failed to set execution amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(execution.GasPrice, 10)
	if !ok {
		return nil, errors.New("failed to set execution gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Execution{
				Execution: &iotextypes.Execution{
					Amount:   amount.String(),
					Contract: execution.Contract,
					Data:     data,
				},
			},
			Version:  uint32(execution.Version),
			Nonce:    uint64(execution.Nonce),
			GasLimit: uint64(execution.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: executorPubKey,
		Signature:    signature,
	}
	return actPb, nil
}

func convertExplorerTransferToActionPb(tsfJSON *explorer.SendTransferRequest,
	maxTransferPayloadBytes uint64) (*iotextypes.Action, error) {
	payload, err := hex.DecodeString(tsfJSON.Payload)
	if err != nil {
		return nil, err
	}
	if uint64(len(payload)) > maxTransferPayloadBytes {
		return nil, errors.Wrapf(
			ErrTransfer,
			"transfer payload contains %d bytes, and is longer than %d bytes limit",
			len(payload),
			maxTransferPayloadBytes,
		)
	}
	senderPubKey, err := keypair.StringToPubKeyBytes(tsfJSON.SenderPubKey)
	if err != nil {
		return nil, err
	}
	signature, err := hex.DecodeString(tsfJSON.Signature)
	if err != nil {
		return nil, err
	}
	amount, ok := big.NewInt(0).SetString(tsfJSON.Amount, 10)
	if !ok {
		return nil, errors.New("failed to set transfer amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(tsfJSON.GasPrice, 10)
	if !ok {
		return nil, errors.New("failed to set transfer gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{
					Amount:    amount.String(),
					Recipient: tsfJSON.Recipient,
					Payload:   payload,
				},
			},
			Version:  uint32(tsfJSON.Version),
			Nonce:    uint64(tsfJSON.Nonce),
			GasLimit: uint64(tsfJSON.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}
	return actPb, nil
}
