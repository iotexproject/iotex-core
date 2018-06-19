// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
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

func randAmount() int64 {
	rand.Seed(time.Now().UnixNano())
	amount := int64(0)
	for amount == int64(0) {
		amount = int64(rand.Intn(100000000))
	}
	return amount
}

func randAmountString() string {
	return strconv.FormatInt(randAmount(), 10)
}

func randTransaction() explorer.Transfer {
	return explorer.Transfer{
		ID:        randAmountString(),
		Sender:    randAmountString(),
		Recipient: randAmountString(),
		Amount:    randAmount(),
		Fee:       12,
		Timestamp: randAmount(),
		BlockID:   randAmountString(),
	}
}

func randBlock() explorer.Block {
	return explorer.Block{
		ID:        randAmountString(),
		Height:    randAmount(),
		Timestamp: randAmount(),
		Transfers: randAmount(),
		GenerateBy: explorer.BlockGenerator{
			Name:    randAmountString(),
			Address: randAmountString(),
		},
		Amount: randAmount(),
		Forged: randAmount(),
	}
}

// TestExplorer return an explorer for test purpose
type TestExplorer struct {
}

// GetAddressBalance returns the balance of an address
func (exp *TestExplorer) GetAddressBalance(address string) (int64, error) {
	return randAmount(), nil
}

// GetAddressDetails returns the properties of an address
func (exp *TestExplorer) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	return explorer.AddressDetails{
		Address:      address,
		TotalBalance: randAmount(),
	}, nil
}

// GetLastTransfersByRange return transfers in [-(offset+limit-1), -offset] from block
// with height startBlockHeight
func (exp *TestExplorer) GetLastTransfersByRange(startBlockHeight int64, offset int64, limit int64, showCoinBase bool) ([]explorer.Transfer, error) {
	var txs []explorer.Transfer
	for i := int64(0); i < limit; i++ {
		txs = append(txs, randTransaction())
	}
	return txs, nil
}

// GetTransferByID returns transfer by transfer id
func (exp *TestExplorer) GetTransferByID(tid string) (explorer.Transfer, error) {
	return randTransaction(), nil
}

// GetTransfersByAddress returns all transfers associate with an address
func (exp *TestExplorer) GetTransfersByAddress(address string) ([]explorer.Transfer, error) {
	return exp.GetLastTransfersByRange(0, 0, 50, true)
}

// GetTransfersByBlockID returns transfers in a block
func (exp *TestExplorer) GetTransfersByBlockID(blockID string) ([]explorer.Transfer, error) {
	return exp.GetLastTransfersByRange(0, 0, 50, true)
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *TestExplorer) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var blks []explorer.Block
	for i := int64(0); i < limit; i++ {
		blks = append(blks, randBlock())
	}
	return blks, nil
}

// GetBlockByID returns block by block id
func (exp *TestExplorer) GetBlockByID(blockID string) (explorer.Block, error) {
	return randBlock(), nil
}

// GetCoinStatistic returns stats in blockchain
func (exp *TestExplorer) GetCoinStatistic() (explorer.CoinStatistic, error) {
	return explorer.CoinStatistic{
		Height: randAmount(),
		Supply: randAmount(),
	}, nil
}
