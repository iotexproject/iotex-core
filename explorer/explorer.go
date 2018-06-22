// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// ErrInternalServer indicates the internal server error
var ErrInternalServer = errors.New("internal server error")

// Service provide api for user to query blockchain data
type Service struct {
	bc        blockchain.Blockchain
	c         consensus.Consensus
	tpsWindow int
}

// GetBlockchainHeight returns the current blockchain tip height
func (exp *Service) GetBlockchainHeight() (int64, error) {
	tip, err := exp.bc.TipHeight()
	return int64(tip), err
}

// GetAddressBalance returns the balance of an address
func (exp *Service) GetAddressBalance(address string) (int64, error) {
	state, err := exp.bc.StateByAddr(address)
	if err != nil {
		return int64(0), err
	}
	return state.Balance.Int64(), nil
}

// GetAddressDetails returns the properties of an address
func (exp *Service) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	state, err := exp.bc.StateByAddr(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	details := explorer.AddressDetails{
		Address:      address,
		TotalBalance: (*state).Balance.Int64(),
		Nonce:        int64((*state).Nonce),
	}

	return details, nil
}

// GetLastTransfersByRange return transfers in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *Service) GetLastTransfersByRange(startBlockHeight int64, offset int64, limit int64, showCoinBase bool) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	transferCount := int64(0)

ChainLoop:
	for height := startBlockHeight; height >= 0; height-- {
		var blkID = ""
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return res, err
		}
		blkID = hex.EncodeToString(hash[:])

		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return res, err
		}

		for i := len(blk.Transfers) - 1; i >= 0; i-- {
			if showCoinBase || !blk.Transfers[i].IsCoinbase {
				transferCount++
			}

			if transferCount <= offset {
				continue
			}

			// if showCoinBase is true, add coinbase transfers, else only put non-coinbase transfers
			if showCoinBase || !blk.Transfers[i].IsCoinbase {
				if int64(len(res)) >= limit {
					break ChainLoop
				}

				hash := blk.Transfers[i].Hash()
				explorerTransfer := explorer.Transfer{
					Amount:    blk.Transfers[i].Amount.Int64(),
					Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
					ID:        hex.EncodeToString(hash[:]),
					Nounce:    int64(blk.Transfers[i].Nonce),
					BlockID:   blkID,
					Sender:    blk.Transfers[i].Sender,
					Recipient: blk.Transfers[i].Recipient,
					Fee:       0, // TODO: we need to get the actual fee.
				}
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
	var transferHash common.Hash32B
	copy(transferHash[:], bytes)

	transfer, err := getTransfer(exp.bc, transferHash)
	if err != nil {
		return explorer.Transfer{}, err
	}

	return transfer, nil
}

// GetTransfersByAddress returns all transfers associate with an address
func (exp *Service) GetTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	transfersFromAddress, err := exp.bc.GetTransfersFromAddress(address)
	if err != nil {
		return nil, err
	}

	transfersToAddress, err := exp.bc.GetTransfersToAddress(address)
	if err != nil {
		return nil, err
	}

	transfersFromAddress = append(transfersFromAddress, transfersToAddress...)
	for i, transferHash := range transfersFromAddress {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerTransfer, err := getTransfer(exp.bc, transferHash)
		if err != nil {
			return res, err
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
		return res, err
	}
	var hash common.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	for i, transfer := range blk.Transfers {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		hash := transfer.Hash()
		explorerTransfer := explorer.Transfer{
			Amount:    transfer.Amount.Int64(),
			Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
			ID:        hex.EncodeToString(hash[:]),
			Nounce:    int64(transfer.Nonce),
			BlockID:   blkID,
			Sender:    transfer.Sender,
			Recipient: transfer.Recipient,
			Fee:       0, // TODO: we need to get the actual fee.
		}
		res = append(res, explorerTransfer)
	}
	return res, nil
}

// GetLastVotesByRange return votes in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *Service) GetLastVotesByRange(startBlockHeight int64, offset int64, limit int64) ([]explorer.Vote, error) {
	var res []explorer.Vote
	voteCount := uint64(0)

ChainLoop:
	for height := startBlockHeight; height >= 0; height-- {
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return res, err
		}
		blkID := hex.EncodeToString(hash[:])

		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return res, err
		}

		for i := int64(len(blk.Votes) - 1); i >= 0; i-- {
			voteCount++

			if voteCount <= uint64(offset) {
				continue
			}

			if int64(len(res)) >= limit {
				break ChainLoop
			}

			voter, err := getAddrFromPubKey(blk.Votes[i].SelfPubkey)
			if err != nil {
				return res, err
			}

			votee, err := getAddrFromPubKey(blk.Votes[i].VotePubkey)
			if err != nil {
				return res, err
			}

			hash := blk.Votes[i].Hash()
			explorerVote := explorer.Vote{
				ID:        hex.EncodeToString(hash[:]),
				Nounce:    int64(blk.Votes[i].Nonce),
				Timestamp: int64(blk.Votes[i].Timestamp),
				Voter:     voter,
				Votee:     votee,
				BlockID:   blkID,
			}
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
	var voteHash common.Hash32B
	copy(voteHash[:], bytes)

	vote, err := getVote(exp.bc, voteHash)
	if err != nil {
		return explorer.Vote{}, err
	}

	return vote, nil
}

// GetVotesByAddress returns all votes associate with an address
func (exp *Service) GetVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	var res []explorer.Vote
	votesFromAddress, err := exp.bc.GetVotesFromAddress(address)
	if err != nil {
		return nil, err
	}

	votesToAddress, err := exp.bc.GetVotesToAddress(address)
	if err != nil {
		return nil, err
	}

	votesFromAddress = append(votesFromAddress, votesToAddress...)
	for i, voteHash := range votesFromAddress {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		explorerVote, err := getVote(exp.bc, voteHash)
		if err != nil {
			return res, err
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
		return res, err
	}
	var hash common.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	for i, vote := range blk.Votes {
		if int64(i) < offset {
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		voter, err := getAddrFromPubKey(vote.SelfPubkey)
		if err != nil {
			return res, err
		}

		votee, err := getAddrFromPubKey(vote.VotePubkey)
		if err != nil {
			return res, err
		}

		hash := vote.Hash()
		explorerVote := explorer.Vote{
			ID:        hex.EncodeToString(hash[:]),
			Nounce:    int64(vote.Nonce),
			Timestamp: int64(vote.Timestamp),
			Voter:     voter,
			Votee:     votee,
			BlockID:   blkID,
		}
		res = append(res, explorerVote)
	}
	return res, nil
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *Service) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var res []explorer.Block

	for height := offset; height >= 0 && int64(len(res)) < limit; height-- {
		blk, err := exp.bc.GetBlockByHeight(uint64(height))
		if err != nil {
			return res, err
		}

		blockHeaderPb := blk.ConvertToBlockHeaderPb()
		hash, err := exp.bc.GetHashByHeight(uint64(height))
		if err != nil {
			return res, err
		}

		totalAmount := int64(0)
		totalSize := uint32(0)
		for _, transfer := range blk.Transfers {
			totalAmount += transfer.Amount.Int64()
			totalSize += transfer.TotalSize()
		}

		explorerBlock := explorer.Block{
			ID:        hex.EncodeToString(hash[:]),
			Height:    int64(blockHeaderPb.Height),
			Timestamp: int64(blockHeaderPb.Timestamp),
			Transfers: int64(len(blk.Transfers)),
			Votes:     int64(len(blk.Votes)),
			Amount:    totalAmount,
			Size:      int64(totalSize),
			GenerateBy: explorer.BlockGenerator{
				Name:    "",
				Address: hex.EncodeToString(blk.Header.Pubkey),
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
	var hash common.Hash32B
	copy(hash[:], bytes)

	blk, err := exp.bc.GetBlockByHash(hash)
	if err != nil {
		return explorer.Block{}, err
	}

	blkHeaderPb := blk.ConvertToBlockHeaderPb()

	totalAmount := int64(0)
	totalSize := uint32(0)
	for _, transfer := range blk.Transfers {
		totalAmount += transfer.Amount.Int64()
		totalSize += transfer.TotalSize()
	}

	explorerBlock := explorer.Block{
		ID:        blkID,
		Height:    int64(blkHeaderPb.Height),
		Timestamp: int64(blkHeaderPb.Timestamp),
		Transfers: int64(len(blk.Transfers)),
		Votes:     int64(len(blk.Votes)),
		Amount:    totalAmount,
		Size:      int64(totalSize),
		GenerateBy: explorer.BlockGenerator{
			Name:    "",
			Address: hex.EncodeToString(blk.Header.Pubkey),
		},
	}

	return explorerBlock, nil
}

// GetCoinStatistic returns stats in blockchain
func (exp *Service) GetCoinStatistic() (explorer.CoinStatistic, error) {
	stat := explorer.CoinStatistic{}

	tipHeight, err := exp.bc.TipHeight()
	if err != nil {
		return stat, err
	}

	totalTransfers, err := exp.bc.GetTotalTransfers()
	if err != nil {
		return stat, err
	}

	totalVotes, err := exp.bc.GetTotalVotes()
	if err != nil {
		return stat, err
	}

	blockLimit := int64(exp.tpsWindow)
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
		actionNumber += blk.Transfers + blk.Votes
	}
	aps := actionNumber / timeDuration

	explorerCoinStats := explorer.CoinStatistic{
		Height:    int64(tipHeight),
		Supply:    int64(blockchain.Gen.TotalSupply),
		Transfers: int64(totalTransfers),
		Votes:     int64(totalVotes),
		Aps:       aps,
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
	for i, d := range cm.LatestDelegates {
		dStrs[i] = d.String()
	}
	var bpStr string
	if cm.LatestBlockProducer != nil {
		bpStr = cm.LatestBlockProducer.String()
	}
	return explorer.ConsensusMetrics{
		LatestEpoch:         int64(cm.LatestEpoch),
		LatestDelegates:     dStrs,
		LatestBlockProducer: bpStr,
	}, nil
}

// getTransfer takes in a blockchain and transferHash and returns a Explorer Transfer
func getTransfer(bc blockchain.Blockchain, transferHash common.Hash32B) (explorer.Transfer, error) {
	explorerTransfer := explorer.Transfer{}

	transfer, err := bc.GetTransferByTransferHash(transferHash)
	if err != nil {
		return explorerTransfer, err
	}

	blkHash, err := bc.GetBlockHashByTransferHash(transferHash)
	if err != nil {
		return explorerTransfer, err
	}

	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorerTransfer, err
	}

	hash := transfer.Hash()
	explorerTransfer = explorer.Transfer{
		Nounce:    int64(transfer.Nonce),
		Amount:    transfer.Amount.Int64(),
		Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
		ID:        hex.EncodeToString(hash[:]),
		BlockID:   hex.EncodeToString(blkHash[:]),
		Sender:    transfer.Sender,
		Recipient: transfer.Recipient,
		Fee:       0, // TODO: we need to get the actual fee.
	}

	return explorerTransfer, nil
}

// getVote takes in a blockchain and voteHash and returns a Explorer Vote
func getVote(bc blockchain.Blockchain, voteHash common.Hash32B) (explorer.Vote, error) {
	var explorerVote explorer.Vote

	vote, err := bc.GetVoteByVoteHash(voteHash)
	if err != nil {
		return explorerVote, err
	}

	blkHash, err := bc.GetBlockHashByVoteHash(voteHash)
	if err != nil {
		return explorerVote, err
	}

	voter, err := getAddrFromPubKey(vote.SelfPubkey)
	if err != nil {
		return explorerVote, err
	}

	votee, err := getAddrFromPubKey(vote.VotePubkey)
	if err != nil {
		return explorerVote, err
	}

	hash := vote.Hash()
	explorerVote = explorer.Vote{
		ID:        hex.EncodeToString(hash[:]),
		Nounce:    int64(vote.Nonce),
		Timestamp: int64(vote.Timestamp),
		Voter:     voter,
		Votee:     votee,
		BlockID:   hex.EncodeToString(blkHash[:]),
	}

	return explorerVote, nil
}

func getAddrFromPubKey(pubKey []byte) (string, error) {
	Address, err := iotxaddress.GetAddress(pubKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		return "", errors.Wrapf(err, " to get address for pubkey %x", pubKey)
	}
	return Address.RawAddress, nil
}
