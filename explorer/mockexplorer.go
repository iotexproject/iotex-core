// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
)

// MockExplorer return an explorer for test purpose
type MockExplorer struct {
}

// GetBlockchainHeight returns the blockchain height
func (exp *MockExplorer) GetBlockchainHeight() (int64, error) {
	return randInt64(), nil
}

// GetAddressBalance returns the balance of an address
func (exp *MockExplorer) GetAddressBalance(address string) (int64, error) {
	return randInt64(), nil
}

// GetAddressDetails returns the properties of an address
func (exp *MockExplorer) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	return explorer.AddressDetails{
		Address:      address,
		TotalBalance: randInt64(),
	}, nil
}

// GetLastTransfersByRange return transfers in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *MockExplorer) GetLastTransfersByRange(startBlockHeight int64, offset int64, limit int64, showCoinBase bool) ([]explorer.Transfer, error) {
	var txs []explorer.Transfer
	for i := int64(0); i < limit; i++ {
		txs = append(txs, randTransaction())
	}
	return txs, nil
}

// GetTransferByID returns transfer by transfer id
func (exp *MockExplorer) GetTransferByID(transferID string) (explorer.Transfer, error) {
	return randTransaction(), nil
}

// GetTransfersByAddress returns all transfers associate with an address
func (exp *MockExplorer) GetTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	return exp.GetLastTransfersByRange(0, offset, limit, true)
}

// GetUnconfirmedTransfersByAddress returns all unconfirmed transfers in actpool associated with an address
func (exp *MockExplorer) GetUnconfirmedTransfersByAddress(address string, offset int64, limit int64) ([]explorer.Transfer, error) {
	return exp.GetLastTransfersByRange(0, offset, limit, true)
}

// GetTransfersByBlockID returns transfers in a block
func (exp *MockExplorer) GetTransfersByBlockID(blockID string, offset int64, limit int64) ([]explorer.Transfer, error) {
	return exp.GetLastTransfersByRange(0, offset, limit, true)
}

// GetLastVotesByRange return votes in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *MockExplorer) GetLastVotesByRange(startBlockHeight int64, offset int64, limit int64) ([]explorer.Vote, error) {
	var votes []explorer.Vote
	for i := int64(0); i < limit; i++ {
		votes = append(votes, randVote())
	}
	return votes, nil
}

// GetVoteByID returns vote by vote id
func (exp *MockExplorer) GetVoteByID(voteID string) (explorer.Vote, error) {
	return randVote(), nil
}

// GetVotesByAddress returns all votes associate with an address
func (exp *MockExplorer) GetVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	return exp.GetLastVotesByRange(0, offset, limit)
}

// GetUnconfirmedVotesByAddress returns all unconfirmed votes in actpool associated with an address
func (exp *MockExplorer) GetUnconfirmedVotesByAddress(address string, offset int64, limit int64) ([]explorer.Vote, error) {
	return exp.GetLastVotesByRange(0, offset, limit)
}

// GetVotesByBlockID returns votes in a block
func (exp *MockExplorer) GetVotesByBlockID(blkID string, offset int64, limit int64) ([]explorer.Vote, error) {
	return exp.GetLastVotesByRange(0, offset, limit)
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *MockExplorer) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var blks []explorer.Block
	for i := int64(0); i < limit; i++ {
		blks = append(blks, randBlock())
	}
	return blks, nil
}

// GetBlockByID returns block by block id
func (exp *MockExplorer) GetBlockByID(blkID string) (explorer.Block, error) {
	return randBlock(), nil
}

// GetCoinStatistic returns stats in blockchain
func (exp *MockExplorer) GetCoinStatistic() (explorer.CoinStatistic, error) {
	return explorer.CoinStatistic{
		Height: randInt64(),
		Supply: randInt64(),
	}, nil
}

// GetConsensusMetrics returns the fake consensus metrics
func (exp *MockExplorer) GetConsensusMetrics() (explorer.ConsensusMetrics, error) {
	delegates := []string{
		randString(),
		randString(),
		randString(),
		randString(),
	}
	return explorer.ConsensusMetrics{
		LatestEpoch:         randInt64(),
		LatestDelegates:     delegates,
		LatestBlockProducer: delegates[0],
	}, nil
}

// GetCandidateMetrics returns the fake delegates metrics
func (exp *MockExplorer) GetCandidateMetrics() (explorer.CandidateMetrics, error) {
	candidate := explorer.Candidate{
		Address:          randString(),
		TotalVote:        randInt64(),
		CreationHeight:   randInt64(),
		LastUpdateHeight: randInt64(),
		IsDelegate:       false,
		IsProducer:       false,
	}
	return explorer.CandidateMetrics{
		Candidates: []explorer.Candidate{candidate},
	}, nil
}

// CreateRawTransfer creates a fake raw transfer
func (exp *MockExplorer) CreateRawTransfer(request explorer.CreateRawTransferRequest) (explorer.CreateRawTransferResponse, error) {
	return explorer.CreateRawTransferResponse{}, nil
}

// SendTransfer sends a fake transfer
func (exp *MockExplorer) SendTransfer(request explorer.SendTransferRequest) (explorer.SendTransferResponse, error) {
	return explorer.SendTransferResponse{}, nil
}

// CreateRawVote creates a fake raw vote
func (exp *MockExplorer) CreateRawVote(request explorer.CreateRawVoteRequest) (explorer.CreateRawVoteResponse, error) {
	return explorer.CreateRawVoteResponse{}, nil
}

// SendVote sends a fake vote
func (exp *MockExplorer) SendVote(request explorer.SendVoteRequest) (explorer.SendVoteResponse, error) {
	return explorer.SendVoteResponse{}, nil
}

func randInt64() int64 {
	rand.Seed(time.Now().UnixNano())
	amount := int64(0)
	for amount == int64(0) {
		amount = int64(rand.Intn(100000000))
	}
	return amount
}

func randString() string {
	return strconv.FormatInt(randInt64(), 10)
}

func randTransaction() explorer.Transfer {
	return explorer.Transfer{
		ID:        randString(),
		Sender:    randString(),
		Recipient: randString(),
		Amount:    randInt64(),
		Fee:       12,
		Timestamp: randInt64(),
		BlockID:   randString(),
	}
}

func randVote() explorer.Vote {
	return explorer.Vote{
		ID:        randString(),
		Timestamp: randInt64(),
		BlockID:   randString(),
		Nonce:     randInt64(),
		Voter:     randString(),
		Votee:     randString(),
	}
}

func randBlock() explorer.Block {
	return explorer.Block{
		ID:        randString(),
		Height:    randInt64(),
		Timestamp: randInt64(),
		Transfers: randInt64(),
		GenerateBy: explorer.BlockGenerator{
			Name:    randString(),
			Address: randString(),
		},
		Amount: randInt64(),
		Forged: randInt64(),
	}
}
