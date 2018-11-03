// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"fmt"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocols/multichain/mainchain"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	pb "github.com/iotexproject/iotex-core/proto"
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

// Service provide api for user to query blockchain data
type Service struct {
	bc  blockchain.Blockchain
	c   consensus.Consensus
	dp  dispatcher.Dispatcher
	ap  actpool.ActPool
	p2p network.Overlay
	cfg config.Explorer
	idx *indexservice.Server
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
		Nonce:        int64((*state).Nonce),
		PendingNonce: int64(pendingNonce),
		IsCandidate:  (*state).IsCandidate,
	}

	return details, nil
}

// GetLastTransfersByRange returns transfers in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *Service) GetLastTransfersByRange(startBlockHeight int64, offset int64, limit int64, showCoinBase bool) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	transferCount := int64(0)

ChainLoop:
	for height := startBlockHeight; height >= 0; height-- {
		var blkID string
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return []explorer.Transfer{}, err
		}
		blkID = hex.EncodeToString(hash[:])

		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return []explorer.Transfer{}, err
		}

		transfers, _, _ := action.ClassifyActions(blk.Actions)

		for i := len(transfers) - 1; i >= 0; i-- {
			if showCoinBase || !transfers[i].IsCoinbase() {
				transferCount++
			}

			if transferCount <= offset {
				continue
			}

			// if showCoinBase is true, add coinbase transfers, else only put non-coinbase transfers
			if showCoinBase || !transfers[i].IsCoinbase() {
				if int64(len(res)) >= limit {
					break ChainLoop
				}

				explorerTransfer, err := convertTsfToExplorerTsf(transfers[i], false)
				if err != nil {
					return []explorer.Transfer{}, errors.Wrapf(err,
						"failed to convert transfer %v to explorer's JSON transfer", transfers[i])
				}
				explorerTransfer.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
				explorerTransfer.BlockID = blkID
				res = append(res, explorerTransfer)
			}
		}
	}

	return res, nil
}

// GetTransferByID returns transfer by transfer id
func (exp *Service) GetTransferByID(transferID string) (explorer.Transfer, error) {
	bytes, err := hex.DecodeString(transferID)
	if err != nil {
		return explorer.Transfer{}, err
	}
	var transferHash hash.Hash32B
	copy(transferHash[:], bytes)

	return getTransfer(exp.bc, exp.ap, transferHash, exp.idx, exp.cfg.UseRDS)
}

// GetTransfersByAddress returns all transfers associated with an address
func (exp *Service) GetTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	var transfers []hash.Hash32B
	if exp.cfg.UseRDS {
		transferHistory, err := exp.idx.Indexer().GetTransferHistory(address)
		if err != nil {
			return []explorer.Transfer{}, err
		}
		transfers = append(transfers, transferHistory...)
	} else {
		transfersFromAddress, err := exp.bc.GetTransfersFromAddress(address)
		if err != nil {
			return []explorer.Transfer{}, err
		}

		transfersToAddress, err := exp.bc.GetTransfersToAddress(address)
		if err != nil {
			return []explorer.Transfer{}, err
		}

		transfersFromAddress = append(transfersFromAddress, transfersToAddress...)
		transfers = append(transfers, transfersFromAddress...)
	}

	for i, transferHash := range transfers {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerTransfer, err := getTransfer(exp.bc, exp.ap, transferHash, exp.idx, exp.cfg.UseRDS)
		if err != nil {
			return []explorer.Transfer{}, err
		}

		res = append(res, explorerTransfer)
	}

	return res, nil
}

// GetUnconfirmedTransfersByAddress returns all unconfirmed transfers in actpool associated with an address
func (exp *Service) GetUnconfirmedTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	res := make([]explorer.Transfer, 0)
	if _, err := exp.bc.StateByAddr(address); err != nil {
		return []explorer.Transfer{}, err
	}

	acts := exp.ap.GetUnconfirmedActs(address)
	tsfIndex := int64(0)
	for _, act := range acts {
		transfer, ok := act.(*action.Transfer)
		if !ok {
			continue
		}

		if tsfIndex < offset {
			tsfIndex++
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerTransfer, err := convertTsfToExplorerTsf(transfer, true)
		if err != nil {
			return []explorer.Transfer{}, errors.Wrapf(err, "failed to convert transfer %v to explorer's JSON transfer", transfer)
		}
		res = append(res, explorerTransfer)
	}

	return res, nil
}

// GetTransfersByBlockID returns transfers in a block
func (exp *Service) GetTransfersByBlockID(blkID string, offset int64, limit int64) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	bytes, err := hex.DecodeString(blkID)

	if err != nil {
		return []explorer.Transfer{}, err
	}
	var hash hash.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return []explorer.Transfer{}, err
	}

	transfers, _, _ := action.ClassifyActions(blk.Actions)

	for i, transfer := range transfers {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerTransfer, err := convertTsfToExplorerTsf(transfer, false)
		if err != nil {
			return []explorer.Transfer{}, errors.Wrapf(err, "failed to convert transfer %v to explorer's JSON transfer", transfer)
		}
		explorerTransfer.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
		explorerTransfer.BlockID = blkID
		res = append(res, explorerTransfer)
	}
	return res, nil
}

// GetLastVotesByRange returns votes in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *Service) GetLastVotesByRange(startBlockHeight int64, offset int64, limit int64) ([]explorer.Vote, error) {
	var res []explorer.Vote
	voteCount := uint64(0)

ChainLoop:
	for height := startBlockHeight; height >= 0; height-- {
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return []explorer.Vote{}, err
		}
		blkID := hex.EncodeToString(hash[:])

		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return []explorer.Vote{}, err
		}

		_, votes, _ := action.ClassifyActions(blk.Actions)

		for i := int64(len(votes) - 1); i >= 0; i-- {
			voteCount++

			if voteCount <= uint64(offset) {
				continue
			}

			if int64(len(res)) >= limit {
				break ChainLoop
			}

			explorerVote, err := convertVoteToExplorerVote(votes[i], false)
			if err != nil {
				return []explorer.Vote{}, errors.Wrapf(err, "failed to convert vote %v to explorer's JSON vote", votes[i])
			}
			explorerVote.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
			explorerVote.BlockID = blkID
			res = append(res, explorerVote)
		}
	}

	return res, nil
}

// GetVoteByID returns vote by vote id
func (exp *Service) GetVoteByID(voteID string) (explorer.Vote, error) {
	bytes, err := hex.DecodeString(voteID)
	if err != nil {
		return explorer.Vote{}, err
	}
	var voteHash hash.Hash32B
	copy(voteHash[:], bytes)

	return getVote(exp.bc, exp.ap, voteHash, exp.idx, exp.cfg.UseRDS)
}

// GetVotesByAddress returns all votes associated with an address
func (exp *Service) GetVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	var res []explorer.Vote
	var votes []hash.Hash32B
	if exp.cfg.UseRDS {
		voteHistory, err := exp.idx.Indexer().GetVoteHistory(address)
		if err != nil {
			return []explorer.Vote{}, err
		}
		votes = append(votes, voteHistory...)
	} else {
		votesFromAddress, err := exp.bc.GetVotesFromAddress(address)
		if err != nil {
			return []explorer.Vote{}, err
		}

		votesToAddress, err := exp.bc.GetVotesToAddress(address)
		if err != nil {
			return []explorer.Vote{}, err
		}

		votesFromAddress = append(votesFromAddress, votesToAddress...)
		votes = append(votes, votesFromAddress...)
	}

	for i, voteHash := range votes {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerVote, err := getVote(exp.bc, exp.ap, voteHash, exp.idx, exp.cfg.UseRDS)
		if err != nil {
			return []explorer.Vote{}, err
		}

		res = append(res, explorerVote)
	}

	return res, nil
}

// GetUnconfirmedVotesByAddress returns all unconfirmed votes in actpool associated with an address
func (exp *Service) GetUnconfirmedVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	res := make([]explorer.Vote, 0)
	if _, err := exp.bc.StateByAddr(address); err != nil {
		return []explorer.Vote{}, err
	}

	acts := exp.ap.GetUnconfirmedActs(address)
	voteIndex := int64(0)
	for _, act := range acts {
		vote, ok := act.(*action.Vote)
		if !ok {
			continue
		}

		if voteIndex < offset {
			voteIndex++
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerVote, err := convertVoteToExplorerVote(vote, true)
		if err != nil {
			return []explorer.Vote{}, errors.Wrapf(err, "failed to convert vote %v to explorer's JSON vote", vote)
		}
		res = append(res, explorerVote)
	}

	return res, nil
}

// GetVotesByBlockID returns votes in a block
func (exp *Service) GetVotesByBlockID(blkID string, offset int64, limit int64) ([]explorer.Vote, error) {
	var res []explorer.Vote
	bytes, err := hex.DecodeString(blkID)
	if err != nil {
		return []explorer.Vote{}, err
	}
	var hash hash.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return []explorer.Vote{}, err
	}

	_, votes, _ := action.ClassifyActions(blk.Actions)

	for i, vote := range votes {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerVote, err := convertVoteToExplorerVote(vote, false)
		if err != nil {
			return []explorer.Vote{}, errors.Wrapf(err, "failed to convert vote %v to explorer's JSON vote", vote)
		}
		explorerVote.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
		explorerVote.BlockID = blkID
		res = append(res, explorerVote)
	}
	return res, nil
}

// GetLastExecutionsByRange returns executions in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *Service) GetLastExecutionsByRange(startBlockHeight int64, offset int64, limit int64) ([]explorer.Execution, error) {
	var res []explorer.Execution
	executionCount := uint64(0)

ChainLoop:
	for height := startBlockHeight; height >= 0; height-- {
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return []explorer.Execution{}, err
		}
		blkID := hex.EncodeToString(hash[:])

		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return []explorer.Execution{}, err
		}

		_, _, executions := action.ClassifyActions(blk.Actions)

		for i := int64(len(executions) - 1); i >= 0; i-- {
			executionCount++

			if executionCount <= uint64(offset) {
				continue
			}

			if int64(len(res)) >= limit {
				break ChainLoop
			}

			explorerExecution, err := convertExecutionToExplorerExecution(executions[i], false)
			if err != nil {
				return []explorer.Execution{}, errors.Wrapf(err,
					"failed to convert execution %v to explorer's JSON execution", executions[i])
			}
			explorerExecution.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
			explorerExecution.BlockID = blkID
			res = append(res, explorerExecution)
		}
	}

	return res, nil
}

// GetExecutionByID returns execution by execution id
func (exp *Service) GetExecutionByID(executionID string) (explorer.Execution, error) {
	bytes, err := hex.DecodeString(executionID)
	if err != nil {
		return explorer.Execution{}, err
	}
	var executionHash hash.Hash32B
	copy(executionHash[:], bytes)

	return getExecution(exp.bc, exp.ap, executionHash, exp.idx, exp.cfg.UseRDS)
}

// GetExecutionsByAddress returns all executions associated with an address
func (exp *Service) GetExecutionsByAddress(address string, offset int64, limit int64) ([]explorer.Execution, error) {
	var res []explorer.Execution
	var executions []hash.Hash32B
	if exp.cfg.UseRDS {
		executionHistory, err := exp.idx.Indexer().GetExecutionHistory(address)
		if err != nil {
			return []explorer.Execution{}, err
		}
		executions = append(executions, executionHistory...)
	} else {
		executionsFromAddress, err := exp.bc.GetExecutionsFromAddress(address)
		if err != nil {
			return []explorer.Execution{}, err
		}

		executionsToAddress, err := exp.bc.GetExecutionsToAddress(address)
		if err != nil {
			return []explorer.Execution{}, err
		}

		executionsFromAddress = append(executionsFromAddress, executionsToAddress...)
		executions = append(executions, executionsFromAddress...)
	}

	for i, executionHash := range executions {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerExecution, err := getExecution(exp.bc, exp.ap, executionHash, exp.idx, exp.cfg.UseRDS)
		if err != nil {
			return []explorer.Execution{}, err
		}

		res = append(res, explorerExecution)
	}

	return res, nil
}

// GetUnconfirmedExecutionsByAddress returns all unconfirmed executions in actpool associated with an address
func (exp *Service) GetUnconfirmedExecutionsByAddress(address string, offset int64, limit int64) ([]explorer.Execution, error) {
	res := make([]explorer.Execution, 0)
	if _, err := exp.bc.StateByAddr(address); err != nil {
		return []explorer.Execution{}, err
	}

	acts := exp.ap.GetUnconfirmedActs(address)
	executionIndex := int64(0)
	for _, act := range acts {
		execution, ok := act.(*action.Execution)
		if !ok {
			continue
		}

		if executionIndex < offset {
			executionIndex++
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerExecution, err := convertExecutionToExplorerExecution(execution, true)
		if err != nil {
			return []explorer.Execution{}, errors.Wrapf(err, "failed to convert execution %v to explorer's JSON execution", execution)
		}
		res = append(res, explorerExecution)
	}

	return res, nil
}

// GetExecutionsByBlockID returns executions in a block
func (exp *Service) GetExecutionsByBlockID(blkID string, offset int64, limit int64) ([]explorer.Execution, error) {
	var res []explorer.Execution
	bytes, err := hex.DecodeString(blkID)

	if err != nil {
		return []explorer.Execution{}, err
	}
	var hash hash.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return []explorer.Execution{}, err
	}

	_, _, executions := action.ClassifyActions(blk.Actions)

	for i, execution := range executions {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerExecution, err := convertExecutionToExplorerExecution(execution, false)
		if err != nil {
			return []explorer.Execution{}, errors.Wrapf(err, "failed to convert execution %v to explorer's JSON execution", execution)
		}
		explorerExecution.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
		explorerExecution.BlockID = blkID
		res = append(res, explorerExecution)
	}
	return res, nil
}

// GetReceiptByExecutionID gets receipt with corresponding execution id
func (exp *Service) GetReceiptByExecutionID(id string) (explorer.Receipt, error) {
	bytes, err := hex.DecodeString(id)
	if err != nil {
		return explorer.Receipt{}, err
	}
	var executionHash hash.Hash32B
	copy(executionHash[:], bytes)
	receipt, err := exp.bc.GetReceiptByExecutionHash(executionHash)
	if err != nil {
		return explorer.Receipt{}, err
	}

	return convertReceiptToExplorerReceipt(receipt)
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *Service) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var res []explorer.Block

	for height := offset; height >= 0 && int64(len(res)) < limit; height-- {
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

		explorerBlock := explorer.Block{
			ID:         hex.EncodeToString(hash[:]),
			Height:     int64(blockHeaderPb.Height),
			Timestamp:  int64(blockHeaderPb.Timestamp),
			Transfers:  int64(len(transfers)),
			Votes:      int64(len(votes)),
			Executions: int64(len(executions)),
			Amount:     totalAmount.String(),
			Size:       int64(totalSize),
			GenerateBy: explorer.BlockGenerator{
				Name:    "",
				Address: keypair.EncodePublicKey(blk.Header.Pubkey),
			},
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
	var hash hash.Hash32B
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

	explorerBlock := explorer.Block{
		ID:         blkID,
		Height:     int64(blkHeaderPb.Height),
		Timestamp:  int64(blkHeaderPb.Timestamp),
		Transfers:  int64(len(transfers)),
		Votes:      int64(len(votes)),
		Executions: int64(len(executions)),
		Amount:     totalAmount.String(),
		Size:       int64(totalSize),
		GenerateBy: explorer.BlockGenerator{
			Name:    "",
			Address: keypair.EncodePublicKey(blk.Header.Pubkey),
		},
	}

	return explorerBlock, nil
}

// GetCoinStatistic returns stats in blockchain
func (exp *Service) GetCoinStatistic() (explorer.CoinStatistic, error) {
	stat := explorer.CoinStatistic{}

	tipHeight := exp.bc.TipHeight()

	totalTransfers, err := exp.bc.GetTotalTransfers()
	if err != nil {
		return stat, err
	}

	totalVotes, err := exp.bc.GetTotalVotes()
	if err != nil {
		return stat, err
	}

	totalExecutions, err := exp.bc.GetTotalExecutions()
	if err != nil {
		return stat, err
	}

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
		Supply:     blockchain.Gen.TotalSupply.String(),
		Transfers:  int64(totalTransfers),
		Votes:      int64(totalVotes),
		Executions: int64(totalExecutions),
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
			Address:          c.Address,
			TotalVote:        c.Votes.String(),
			CreationHeight:   int64(c.CreationHeight),
			LastUpdateHeight: int64(c.LastUpdateHeight),
			IsDelegate:       false,
			IsProducer:       false,
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
		pubKey, err := keypair.BytesToPubKeyString(c.PublicKey[:])
		if err != nil {
			return explorer.CandidateMetrics{}, errors.Wrapf(err,
				"Invalid candidate pub key")
		}
		candidates = append(candidates, explorer.Candidate{
			Address:          c.Address,
			PubKey:           pubKey,
			TotalVote:        c.Votes.String(),
			CreationHeight:   int64(c.CreationHeight),
			LastUpdateHeight: int64(c.LastUpdateHeight),
		})
	}

	return explorer.CandidateMetrics{
		Candidates: candidates,
	}, nil
}

// SendTransfer sends a transfer
func (exp *Service) SendTransfer(tsfJSON explorer.SendTransferRequest) (resp explorer.SendTransferResponse, err error) {
	logger.Debug().Msg("receive send transfer request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
		}
		requestMtc.WithLabelValues("SendTransfer", succeed).Inc()
	}()

	payload, err := hex.DecodeString(tsfJSON.Payload)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	if uint64(len(payload)) > exp.cfg.MaxTransferPayloadBytes {
		return explorer.SendTransferResponse{}, errors.Wrapf(
			ErrTransfer,
			"transfer payload contains %d bytes, and is longer than %d bytes limit",
			len(payload),
			exp.cfg.MaxTransferPayloadBytes,
		)
	}
	senderPubKey, err := keypair.StringToPubKeyBytes(tsfJSON.SenderPubKey)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	signature, err := hex.DecodeString(tsfJSON.Signature)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	amount, ok := big.NewInt(0).SetString(tsfJSON.Amount, 10)
	if !ok {
		return explorer.SendTransferResponse{}, errors.New("failed to set transfer amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(tsfJSON.GasPrice, 10)
	if !ok {
		return explorer.SendTransferResponse{}, errors.New("failed to set transfer gas price")
	}
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Transfer{
			Transfer: &pb.TransferPb{
				Amount:     amount.Bytes(),
				Recipient:  tsfJSON.Recipient,
				Payload:    payload,
				IsCoinbase: tsfJSON.IsCoinbase,
			},
		},
		Version:      uint32(tsfJSON.Version),
		Sender:       tsfJSON.Sender,
		SenderPubKey: senderPubKey,
		Nonce:        uint64(tsfJSON.Nonce),
		GasLimit:     uint64(tsfJSON.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}
	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendTransferResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), actPb, nil)

	tsf := &action.Transfer{}
	tsf.LoadProto(actPb)
	h := tsf.Hash()
	return explorer.SendTransferResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// SendVote sends a vote
func (exp *Service) SendVote(voteJSON explorer.SendVoteRequest) (resp explorer.SendVoteResponse, err error) {
	logger.Debug().Msg("receive send vote request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
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
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Vote{
			Vote: &pb.VotePb{
				VoteeAddress: voteJSON.Votee,
			},
		},
		Version:      uint32(voteJSON.Version),
		Sender:       voteJSON.Voter,
		SenderPubKey: selfPubKey,
		Nonce:        uint64(voteJSON.Nonce),
		GasLimit:     uint64(voteJSON.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}
	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendVoteResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), actPb, nil)

	v := &action.Vote{}
	v.LoadProto(actPb)
	h := v.Hash()
	return explorer.SendVoteResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// PutSubChainBlock put block merkel root on root chain.
func (exp *Service) PutSubChainBlock(putBlockJSON explorer.PutSubChainBlockRequest) (resp explorer.PutSubChainBlockResponse, err error) {
	logger.Debug().Msg("receive put block request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
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

	roots := make(map[string][]byte)
	for _, mr := range putBlockJSON.Roots {
		v, err := hex.DecodeString(mr.Value)
		if err != nil {
			return explorer.PutSubChainBlockResponse{}, err
		}
		roots[mr.Name] = v
	}
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_PutBlock{
			PutBlock: &pb.PutBlockPb{
				SubChainAddress: putBlockJSON.SubChainAddress,
				Height:          uint64(putBlockJSON.Height),
				Roots:           roots,
			},
		},
		Version:      uint32(putBlockJSON.Version),
		Sender:       putBlockJSON.SenderAddress,
		SenderPubKey: senderPubKey,
		Nonce:        uint64(putBlockJSON.Nonce),
		GasLimit:     uint64(putBlockJSON.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}
	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), actPb); err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), actPb, nil)

	v := &action.PutBlock{}
	v.LoadProto(actPb)
	h := v.Hash()
	return explorer.PutSubChainBlockResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// SendAction is the API to send an action to blockchain.
func (exp *Service) SendAction(req explorer.SendActionRequest) (resp explorer.SendActionResponse, err error) {
	logger.Debug().Msg("receive send action request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
		}
		requestMtc.WithLabelValues("SendAction", succeed).Inc()
	}()

	raw, err := hex.DecodeString(req.Payload)
	if err != nil {
		return explorer.SendActionResponse{}, err
	}

	var payload pb.SendActionRequest
	if err := proto.Unmarshal(raw, &payload); err != nil {
		return explorer.SendActionResponse{}, err
	}

	if payload.GetAction() == nil {
		return explorer.SendActionResponse{}, errors.New("empty request payload")
	}

	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), payload.GetAction()); err != nil {
		logger.Warn().Err(err).Msg("failed to broadcast SendAction request.")
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), payload.GetAction(), nil)

	return explorer.SendActionResponse{}, nil
}

// GetPeers return a list of node peers and itself's network addsress info.
func (exp *Service) GetPeers() (explorer.GetPeersResponse, error) {
	var peers []explorer.Node
	for _, p := range exp.p2p.GetPeers() {
		peers = append(peers, explorer.Node{
			Address: p.String(),
		})
	}
	return explorer.GetPeersResponse{
		Self:  explorer.Node{Address: exp.p2p.Self().String()},
		Peers: peers,
	}, nil
}

// SendSmartContract sends a smart contract
func (exp *Service) SendSmartContract(execution explorer.Execution) (resp explorer.SendSmartContractResponse, err error) {
	logger.Debug().Msg("receive send smart contract request")

	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
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
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Execution{
			Execution: &pb.ExecutionPb{
				Amount:   amount.Bytes(),
				Contract: execution.Contract,
				Data:     data,
			},
		},
		Version:      uint32(execution.Version),
		Sender:       execution.Executor,
		SenderPubKey: executorPubKey,
		Nonce:        uint64(execution.Nonce),
		GasLimit:     uint64(execution.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}
	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), actPb); err != nil {
		return explorer.SendSmartContractResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), actPb, nil)

	sc := &action.Execution{}
	sc.LoadProto(actPb)
	h := sc.Hash()
	return explorer.SendSmartContractResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// ReadExecutionState reads the state in a contract address specified by the slot
func (exp *Service) ReadExecutionState(execution explorer.Execution) (string, error) {
	logger.Debug().Msg("receive read smart contract request")

	data, err := hex.DecodeString(execution.Data)
	if err != nil {
		return "", err
	}
	signature, err := hex.DecodeString(execution.Signature)
	if err != nil {
		return "", err
	}
	amount, ok := big.NewInt(0).SetString(execution.Amount, 10)
	if !ok {
		return "", errors.New("failed to set execution amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(execution.GasPrice, 10)
	if !ok {
		return "", errors.New("failed to set execution gas price")
	}
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Execution{
			Execution: &pb.ExecutionPb{
				Amount:   amount.Bytes(),
				Contract: execution.Contract,
				Data:     data,
			},
		},
		Version:      uint32(execution.Version),
		Sender:       execution.Executor,
		SenderPubKey: nil,
		Nonce:        uint64(execution.Nonce),
		GasLimit:     uint64(execution.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}

	sc := &action.Execution{}
	sc.LoadProto(actPb)
	res, err := exp.bc.ExecuteContractRead(sc)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(res), nil
}

// GetBlockOrActionByHash get block or action by a hash
func (exp *Service) GetBlockOrActionByHash(hashStr string) (explorer.GetBlkOrActResponse, error) {
	if blk, err := exp.GetBlockByID(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Block: &blk}, nil
	}

	if tsf, err := exp.GetTransferByID(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Transfer: &tsf}, nil
	}

	if vote, err := exp.GetVoteByID(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Vote: &vote}, nil
	}

	if exe, err := exp.GetExecutionByID(hashStr); err == nil {
		return explorer.GetBlkOrActResponse{Execution: &exe}, nil
	}

	return explorer.GetBlkOrActResponse{}, nil
}

// Deposit deposits balance from main-chain to sub-chain
func (exp *Service) Deposit(req explorer.DepositRequest) (res explorer.DepositResponse, err error) {
	defer func() {
		succeed := "true"
		if err != nil {
			succeed = "false"
		}
		requestMtc.WithLabelValues("deposit", succeed).Inc()
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
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Deposit{
			Deposit: &pb.DepositPb{
				Amount:    amount.Bytes(),
				Recipient: req.Recipient,
			},
		},
		Version:      uint32(req.Version),
		Sender:       req.Sender,
		SenderPubKey: senderPubKey,
		Nonce:        uint64(req.Nonce),
		GasLimit:     uint64(req.GasLimit),
		GasPrice:     gasPrice.Bytes(),
		Signature:    signature,
	}
	// broadcast to the network
	if err = exp.p2p.Broadcast(exp.bc.ChainID(), actPb); err != nil {
		return res, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(exp.bc.ChainID(), actPb, nil)

	deposit := &action.Deposit{}
	deposit.LoadProto(actPb)
	h := deposit.Hash()
	return explorer.DepositResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// GetDeposits returns the deposits of a sub-chain in the given range in descending order by the index
func (exp *Service) GetDeposits(subChainID int64, offset int64, limit int64) ([]explorer.Deposit, error) {
	subChainsInOp, err := exp.mainChain.SubChainsInOperation()
	if err != nil {
		return nil, err
	}
	var targetSubChain mainchain.InOperation
	for _, e := range subChainsInOp {
		subChainInOp, ok := e.(mainchain.InOperation)
		if !ok {
			return nil, errors.New("error when casting the element in sorted slice into sub-chain in operation")
		}
		if subChainInOp.ID == uint32(subChainID) {
			targetSubChain = subChainInOp
		}
	}
	if targetSubChain.ID != uint32(subChainID) {
		return nil, fmt.Errorf("sub-chain %d is not found in operation", subChainID)
	}
	subChainAddr, err := address.BytesToAddress(targetSubChain.Addr)
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
		recepient, err := address.BytesToAddress(deposit.Addr)
		if err != nil {
			return nil, err
		}
		deposits = append(deposits, explorer.Deposit{
			Amount:    deposit.Amount.String(),
			Address:   recepient.IotxAddress(),
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

// getTransfer takes in a blockchain and transferHash and returns an Explorer Transfer
func getTransfer(bc blockchain.Blockchain, ap actpool.ActPool, transferHash hash.Hash32B, idx *indexservice.Server, useRDS bool) (explorer.Transfer, error) {
	explorerTransfer := explorer.Transfer{}

	transfer, err := bc.GetTransferByTransferHash(transferHash)
	if err != nil {
		// Try to fetch pending transfer from actpool
		act, err := ap.GetActionByHash(transferHash)
		if err != nil || act == nil {
			return explorerTransfer, err
		}
		transfer = act.(*action.Transfer)
		return convertTsfToExplorerTsf(transfer, true)
	}

	// Fetch from block
	var blkHash hash.Hash32B
	if useRDS {
		hash, err := idx.Indexer().GetBlockByTransfer(transferHash)
		if err != nil {
			return explorerTransfer, err
		}
		blkHash = hash
	} else {
		hash, err := bc.GetBlockHashByTransferHash(transferHash)
		if err != nil {
			return explorerTransfer, err
		}
		blkHash = hash
	}

	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorerTransfer, err
	}

	if explorerTransfer, err = convertTsfToExplorerTsf(transfer, false); err != nil {
		return explorerTransfer, errors.Wrapf(err, "failed to convert transfer %v to explorer's JSON transfer", transfer)
	}
	explorerTransfer.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
	explorerTransfer.BlockID = hex.EncodeToString(blkHash[:])
	return explorerTransfer, nil
}

// getVote takes in a blockchain and voteHash and returns an Explorer Vote
func getVote(bc blockchain.Blockchain, ap actpool.ActPool, voteHash hash.Hash32B, idx *indexservice.Server, useRDS bool) (explorer.Vote, error) {
	explorerVote := explorer.Vote{}

	vote, err := bc.GetVoteByVoteHash(voteHash)
	if err != nil {
		// Try to fetch pending vote from actpool
		act, err := ap.GetActionByHash(voteHash)
		if err != nil || act == nil {
			return explorerVote, err
		}
		vote = act.(*action.Vote)
		return convertVoteToExplorerVote(vote, true)
	}

	// Fetch from block
	var blkHash hash.Hash32B
	if useRDS {
		hash, err := idx.Indexer().GetBlockByVote(voteHash)
		if err != nil {
			return explorerVote, err
		}
		blkHash = hash
	} else {
		hash, err := bc.GetBlockHashByVoteHash(voteHash)
		if err != nil {
			return explorerVote, err
		}
		blkHash = hash
	}

	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorerVote, err
	}

	if explorerVote, err = convertVoteToExplorerVote(vote, false); err != nil {
		return explorerVote, errors.Wrapf(err, "failed to convert vote %v to explorer's JSON vote", vote)
	}
	explorerVote.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
	explorerVote.BlockID = hex.EncodeToString(blkHash[:])
	return explorerVote, nil
}

// getExecution takes in a blockchain and executionHash and returns an Explorer execution
func getExecution(bc blockchain.Blockchain, ap actpool.ActPool, executionHash hash.Hash32B, idx *indexservice.Server, useRDS bool) (explorer.Execution, error) {
	explorerExecution := explorer.Execution{}

	execution, err := bc.GetExecutionByExecutionHash(executionHash)
	if err != nil {
		// Try to fetch pending execution from actpool
		act, err := ap.GetActionByHash(executionHash)
		if err != nil || act == nil {
			return explorerExecution, err
		}
		execution = act.(*action.Execution)
		return convertExecutionToExplorerExecution(execution, true)
	}

	// Fetch from block
	var blkHash hash.Hash32B
	if useRDS {
		hash, err := idx.Indexer().GetBlockByExecution(executionHash)
		if err != nil {
			return explorerExecution, err
		}
		blkHash = hash
	} else {
		hash, err := bc.GetBlockHashByExecutionHash(executionHash)
		if err != nil {
			return explorerExecution, err
		}
		blkHash = hash
	}

	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorerExecution, err
	}

	if explorerExecution, err = convertExecutionToExplorerExecution(execution, false); err != nil {
		return explorerExecution, errors.Wrapf(err, "failed to convert execution %v to explorer's JSON execution", execution)
	}
	explorerExecution.Timestamp = int64(blk.ConvertToBlockHeaderPb().Timestamp)
	explorerExecution.BlockID = hex.EncodeToString(blkHash[:])
	return explorerExecution, nil
}

func convertTsfToExplorerTsf(transfer *action.Transfer, isPending bool) (explorer.Transfer, error) {
	if transfer == nil {
		return explorer.Transfer{}, errors.Wrap(ErrTransfer, "transfer cannot be nil")
	}
	hash := transfer.Hash()
	explorerTransfer := explorer.Transfer{
		Nonce:     int64(transfer.Nonce()),
		ID:        hex.EncodeToString(hash[:]),
		Sender:    transfer.Sender(),
		Recipient: transfer.Recipient(),
		Fee:       "", // TODO: we need to get the actual fee.
		Payload:   hex.EncodeToString(transfer.Payload()),
		GasLimit:  int64(transfer.GasLimit()),
		IsPending: isPending,
	}
	if transfer.Amount() != nil && len(transfer.Amount().String()) > 0 {
		explorerTransfer.Amount = transfer.Amount().String()
	}
	if transfer.GasPrice() != nil && len(transfer.GasPrice().String()) > 0 {
		explorerTransfer.GasPrice = transfer.GasPrice().String()
	}
	return explorerTransfer, nil
}

func convertVoteToExplorerVote(vote *action.Vote, isPending bool) (explorer.Vote, error) {
	if vote == nil {
		return explorer.Vote{}, errors.Wrap(ErrVote, "vote cannot be nil")
	}
	hash := vote.Hash()
	voterPubkey := vote.VoterPublicKey()
	explorerVote := explorer.Vote{
		ID:          hex.EncodeToString(hash[:]),
		Nonce:       int64(vote.Nonce()),
		Voter:       vote.Voter(),
		VoterPubKey: hex.EncodeToString(voterPubkey[:]),
		Votee:       vote.Votee(),
		GasLimit:    int64(vote.GasLimit()),
		GasPrice:    vote.GasPrice().String(),
		IsPending:   isPending,
	}
	return explorerVote, nil
}

func convertExecutionToExplorerExecution(execution *action.Execution, isPending bool) (explorer.Execution, error) {
	if execution == nil {
		return explorer.Execution{}, errors.Wrap(ErrExecution, "execution cannot be nil")
	}
	hash := execution.Hash()
	explorerExecution := explorer.Execution{
		Nonce:     int64(execution.Nonce()),
		ID:        hex.EncodeToString(hash[:]),
		Executor:  execution.Executor(),
		Contract:  execution.Contract(),
		GasLimit:  int64(execution.GasLimit()),
		Data:      hex.EncodeToString(execution.Data()),
		IsPending: isPending,
	}
	if execution.Amount() != nil && len(execution.Amount().String()) > 0 {
		explorerExecution.Amount = execution.Amount().String()
	}
	if execution.GasPrice() != nil && len(execution.GasPrice().String()) > 0 {
		explorerExecution.GasPrice = execution.GasPrice().String()
	}
	return explorerExecution, nil
}

func convertReceiptToExplorerReceipt(receipt *blockchain.Receipt) (explorer.Receipt, error) {
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
			BlockHash:   hex.EncodeToString(log.BlockHash[:]),
			Index:       int64(log.Index),
		})
	}

	return explorer.Receipt{
		ReturnValue:     hex.EncodeToString(receipt.ReturnValue),
		Status:          int64(receipt.Status),
		Hash:            hex.EncodeToString(receipt.Hash[:]),
		GasConsumed:     int64(receipt.GasConsumed),
		ContractAddress: receipt.ContractAddress,
		Logs:            logs,
	}, nil
}
