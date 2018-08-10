// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	pb "github.com/iotexproject/iotex-core/proto"
)

// ErrInternalServer indicates the internal server error
var ErrInternalServer = errors.New("internal server error")

// Service provide api for user to query blockchain data
type Service struct {
	bc        blockchain.Blockchain
	c         consensus.Consensus
	dp        dispatcher.Dispatcher
	ap        actpool.ActPool
	p2p       network.Overlay
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
	pendingNonce, err := exp.ap.GetPendingNonce(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	details := explorer.AddressDetails{
		Address:      address,
		TotalBalance: (*state).Balance.Int64(),
		Nonce:        int64((*state).Nonce),
		PendingNonce: int64(pendingNonce),
		IsCandidate:  (*state).IsCandidate,
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
					Nonce:     int64(blk.Transfers[i].Nonce),
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
	var transferHash hash.Hash32B
	copy(transferHash[:], bytes)

	transfer, err := getTransfer(exp.bc, transferHash)
	if err != nil {
		return explorer.Transfer{}, err
	}

	return transfer, nil
}

// GetTransfersByAddress returns all transfers associated with an address
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

// GetUnconfirmedTransfersByAddress returns all unconfirmed transfers in actpool associated with an address
func (exp *Service) GetUnconfirmedTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	res := make([]explorer.Transfer, 0)
	if _, err := exp.bc.StateByAddr(address); err != nil {
		return res, err
	}

	acts := exp.ap.GetUnconfirmedActs(address)
	tsfIndex := int64(0)
	for _, act := range acts {
		if act.GetVote() != nil {
			continue
		}

		if tsfIndex < offset {
			tsfIndex++
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		transferPb := act.GetTransfer()
		senderPubKey, err := keypair.BytesToPubKeyString(transferPb.SenderPubKey)
		if err != nil {
			return []explorer.Transfer{}, err
		}
		explorerTransfer := explorer.Transfer{
			Version:      int64(act.Version),
			Nonce:        int64(act.Nonce),
			Sender:       transferPb.Sender,
			Recipient:    transferPb.Recipient,
			Amount:       big.NewInt(0).SetBytes(transferPb.Amount).Int64(),
			Payload:      hex.EncodeToString(transferPb.Payload),
			SenderPubKey: senderPubKey,
			Signature:    hex.EncodeToString(act.Signature),
			IsCoinbase:   transferPb.IsCoinbase,
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
	var hash hash.Hash32B
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
			Nonce:     int64(transfer.Nonce),
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

			selfPublicKey, err := blk.Votes[i].SelfPublicKey()
			if err != nil {
				return res, err
			}
			voter, err := getAddrFromPubKey(selfPublicKey)
			if err != nil {
				return res, err
			}

			votee := blk.Votes[i].GetVote().VoteeAddress
			if err != nil {
				return res, err
			}

			hash := blk.Votes[i].Hash()
			explorerVote := explorer.Vote{
				ID:        hex.EncodeToString(hash[:]),
				Nonce:     int64(blk.Votes[i].Nonce),
				Timestamp: int64(blk.Votes[i].GetVote().Timestamp),
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
	var voteHash hash.Hash32B
	copy(voteHash[:], bytes)

	vote, err := getVote(exp.bc, voteHash)
	if err != nil {
		return explorer.Vote{}, err
	}

	return vote, nil
}

// GetVotesByAddress returns all votes associated with an address
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

// GetUnconfirmedVotesByAddress returns all unconfirmed votes in actpool associated with an address
func (exp *Service) GetUnconfirmedVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	res := make([]explorer.Vote, 0)
	if _, err := exp.bc.StateByAddr(address); err != nil {
		return res, err
	}

	acts := exp.ap.GetUnconfirmedActs(address)
	voteIndex := int64(0)
	for _, act := range acts {
		if act.GetTransfer() != nil {
			continue
		}

		if voteIndex < offset {
			voteIndex++
			continue
		}

		if int64(len(res)) >= limit {
			break
		}

		votePb := act.GetVote()
		voterPubKey, err := keypair.BytesToPubKeyString(votePb.SelfPubkey)
		if err != nil {
			return []explorer.Vote{}, err
		}
		explorerVote := explorer.Vote{
			Version:     int64(act.Version),
			Nonce:       int64(act.Nonce),
			VoterPubKey: voterPubKey,
			Signature:   hex.EncodeToString(act.Signature),
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
	var hash hash.Hash32B
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

		selfPublicKey, err := vote.SelfPublicKey()
		if err != nil {
			return res, err
		}
		voter, err := getAddrFromPubKey(selfPublicKey)
		if err != nil {
			return res, err
		}

		votee := vote.GetVote().VoteeAddress
		if err != nil {
			return res, err
		}

		hash := vote.Hash()
		explorerVote := explorer.Vote{
			ID:        hex.EncodeToString(hash[:]),
			Nonce:     int64(vote.Nonce),
			Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
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
			Address: keypair.EncodePublicKey(blk.Header.Pubkey),
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
		dStrs[i] = d
	}
	var bpStr string
	if cm.LatestBlockProducer != "" {
		bpStr = cm.LatestBlockProducer
	}
	cStrs := make([]string, len(cm.Candidates))
	for i, c := range cm.Candidates {
		cStrs[i] = c
	}
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
	allCandidates, ok := exp.bc.CandidatesByHeight(cm.LatestHeight)
	if !ok {
		return explorer.CandidateMetrics{}, errors.Wrapf(blockchain.ErrCandidates,
			"Failed to get the candidate metrics")
	}
	candidates := make([]explorer.Candidate, len(cm.Candidates))
	for i, c := range allCandidates {
		candidates[i] = explorer.Candidate{
			Address:          c.Address,
			TotalVote:        c.Votes.Int64(),
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
		Candidates: candidates,
	}, nil
}

// SendTransfer sends a transfer
func (exp *Service) SendTransfer(request explorer.SendTransferRequest) (explorer.SendTransferResponse, error) {
	logger.Debug().Msg("receive send transfer request")

	if len(request.SerlializedTransfer) == 0 {
		return explorer.SendTransferResponse{}, errors.New("invalid SendTransferRequest")
	}

	tsfJSON := &explorer.Transfer{}
	if err := json.Unmarshal([]byte(request.SerlializedTransfer), tsfJSON); err != nil {
		return explorer.SendTransferResponse{}, err
	}
	amount := big.NewInt(tsfJSON.Amount).Bytes()

	payload, err := hex.DecodeString(tsfJSON.Payload)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	senderPubKey, err := keypair.StringToPubKeyBytes(tsfJSON.SenderPubKey)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	signature, err := hex.DecodeString(tsfJSON.Signature)
	if err != nil {
		return explorer.SendTransferResponse{}, err
	}
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Transfer{
			Transfer: &pb.TransferPb{
				Amount:       amount,
				Sender:       tsfJSON.Sender,
				Recipient:    tsfJSON.Recipient,
				Payload:      payload,
				SenderPubKey: senderPubKey,
				IsCoinbase:   tsfJSON.IsCoinbase,
			},
		},
		Version:   uint32(tsfJSON.Version),
		Nonce:     uint64(tsfJSON.Nonce),
		Signature: signature,
	}

	// broadcast to the network
	if err := exp.p2p.Broadcast(actPb); err != nil {
		return explorer.SendTransferResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(actPb, nil)
	return explorer.SendTransferResponse{true}, nil
}

// SendVote sends a vote
func (exp *Service) SendVote(request explorer.SendVoteRequest) (explorer.SendVoteResponse, error) {
	if len(request.SerializedVote) == 0 {
		return explorer.SendVoteResponse{}, errors.New("invalid SendVoteRequest")
	}

	voteJSON := &explorer.Vote{}
	if err := json.Unmarshal([]byte(request.SerializedVote), voteJSON); err != nil {
		return explorer.SendVoteResponse{}, err
	}
	selfPubKey, err := keypair.StringToPubKeyBytes(voteJSON.VoterPubKey)
	if err != nil {
		return explorer.SendVoteResponse{}, err
	}
	signature, err := hex.DecodeString(voteJSON.Signature)
	if err != nil {
		return explorer.SendVoteResponse{}, err
	}
	actPb := &pb.ActionPb{
		Action: &pb.ActionPb_Vote{
			Vote: &pb.VotePb{
				SelfPubkey:   selfPubKey,
				VoterAddress: voteJSON.Voter,
				VoteeAddress: voteJSON.Votee,
			},
		},
		Version:   uint32(voteJSON.Version),
		Nonce:     uint64(voteJSON.Nonce),
		Signature: signature,
	}

	// broadcast to the network
	if err := exp.p2p.Broadcast(actPb); err != nil {
		return explorer.SendVoteResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(actPb, nil)
	return explorer.SendVoteResponse{true}, nil
}

// getTransfer takes in a blockchain and transferHash and returns a Explorer Transfer
func getTransfer(bc blockchain.Blockchain, transferHash hash.Hash32B) (explorer.Transfer, error) {
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
		Nonce:     int64(transfer.Nonce),
		Amount:    transfer.Amount.Int64(),
		Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
		ID:        hex.EncodeToString(hash[:]),
		BlockID:   hex.EncodeToString(blkHash[:]),
		Sender:    transfer.Sender,
		Recipient: transfer.Recipient,
		Fee:       0, // TODO: we need to get the actual fee.
		Payload:   hex.EncodeToString(transfer.Payload),
	}

	return explorerTransfer, nil
}

// getVote takes in a blockchain and voteHash and returns a Explorer Vote
func getVote(bc blockchain.Blockchain, voteHash hash.Hash32B) (explorer.Vote, error) {
	var explorerVote explorer.Vote

	vote, err := bc.GetVoteByVoteHash(voteHash)
	if err != nil {
		return explorerVote, err
	}

	blkHash, err := bc.GetBlockHashByVoteHash(voteHash)
	if err != nil {
		return explorerVote, err
	}

	blk, err := bc.GetBlockByHash(blkHash)
	if err != nil {
		return explorerVote, err
	}

	selfPublicKey, err := vote.SelfPublicKey()
	if err != nil {
		return explorerVote, err
	}
	voter, err := getAddrFromPubKey(selfPublicKey)
	if err != nil {
		return explorerVote, err
	}

	votee := vote.GetVote().VoteeAddress
	if err != nil {
		return explorerVote, err
	}

	hash := vote.Hash()
	explorerVote = explorer.Vote{
		ID:        hex.EncodeToString(hash[:]),
		Nonce:     int64(vote.Nonce),
		Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
		Voter:     voter,
		Votee:     votee,
		BlockID:   hex.EncodeToString(blkHash[:]),
	}

	return explorerVote, nil
}

func getAddrFromPubKey(pubKey keypair.PublicKey) (string, error) {
	Address, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, pubKey)
	if err != nil {
		return "", errors.Wrapf(err, " to get address for pubkey %x", pubKey)
	}
	return Address.RawAddress, nil
}
