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
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
)

// ErrInternalServer indicates the internal server error
var ErrInternalServer = errors.New("internal server error")

// Service provide api for user to query blockchain data
type Service struct {
	bc        blockchain.Blockchain
	tpsWindow int
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
		hash, getHashErr := exp.bc.GetHashByHeight(uint64(height))
		if getHashErr != nil {
			return res, getHashErr
		}
		blkID = hex.EncodeToString(hash[:])

		blk, getErr := exp.bc.GetBlockByHeight(uint64(height))
		if getErr != nil {
			return res, getErr
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
				hash := blk.Transfers[i].Hash()
				explorerTransfer := explorer.Transfer{
					Amount:    blk.Transfers[i].Amount.Int64(),
					Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
					ID:        hex.EncodeToString(hash[:]),
					BlockID:   blkID,
					Sender:    blk.Transfers[i].Sender,
					Recipient: blk.Transfers[i].Recipient,
					Fee:       0, // TODO: we need to get the actual fee.
				}
				res = append(res, explorerTransfer)
				if int64(len(res)) >= limit {
					break ChainLoop
				}
			}
		}
	}

	return res, nil
}

// GetTransferByID returns transfer by transfer id
func (exp *Service) GetTransferByID(tid string) (explorer.Transfer, error) {
	bytes, decodeErr := hex.DecodeString(tid)
	if decodeErr != nil {
		return explorer.Transfer{}, decodeErr
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
	transfersFromAddress, getFromAddressErr := exp.bc.GetTransfersFromAddress(address)
	if getFromAddressErr != nil {
		return nil, getFromAddressErr
	}

	transfersToAddress, getToAddressErr := exp.bc.GetTransfersToAddress(address)
	if getToAddressErr != nil {
		return nil, getToAddressErr
	}

	transfersFromAddress = append(transfersFromAddress, transfersToAddress...)
	transferCount := int64(0)
	for _, transferHash := range transfersFromAddress {
		transferCount++
		if transferCount <= offset {
			continue
		}
		explorerTransfer, err := getTransfer(exp.bc, transferHash)
		if err != nil {
			return res, err
		}

		res = append(res, explorerTransfer)
		if int64(len(res)) >= limit {
			break
		}
	}

	return res, nil
}

// GetTransfersByBlockID returns transfers in a block
func (exp *Service) GetTransfersByBlockID(blockID string, offset int64, limit int64) ([]explorer.Transfer, error) {
	var res []explorer.Transfer
	bytes, decodeErr := hex.DecodeString(blockID)

	if decodeErr != nil {
		return res, decodeErr
	}
	var hash common.Hash32B
	copy(hash[:], bytes)

	blk, getErr := exp.bc.GetBlockByHash(hash)
	if getErr != nil {
		return res, getErr
	}

	transferCount := int64(0)
	for _, transfer := range blk.Transfers {
		transferCount++
		if transferCount <= offset {
			continue
		}

		hash := transfer.Hash()
		explorerTransfer := explorer.Transfer{
			Amount:    transfer.Amount.Int64(),
			Timestamp: int64(blk.ConvertToBlockHeaderPb().Timestamp),
			ID:        hex.EncodeToString(hash[:]),
			BlockID:   blockID,
			Sender:    transfer.Sender,
			Recipient: transfer.Recipient,
			Fee:       0, // TODO: we need to get the actual fee.
		}
		res = append(res, explorerTransfer)
		if int64(len(res)) >= limit {
			break
		}
	}
	return res, nil
}

// GetLastBlocksByRange get block with height [offset-limit+1, offset]
func (exp *Service) GetLastBlocksByRange(offset int64, limit int64) ([]explorer.Block, error) {
	var res []explorer.Block

	for height := offset; height >= 0 && int64(len(res)) < limit; height-- {
		blk, getErr := exp.bc.GetBlockByHeight(uint64(height))
		if getErr != nil {
			return res, getErr
		}

		blockHeaderPb := blk.ConvertToBlockHeaderPb()
		hash, encodeErr := exp.bc.GetHashByHeight(uint64(height))
		if encodeErr != nil {
			return res, encodeErr
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
func (exp *Service) GetBlockByID(blockID string) (explorer.Block, error) {
	bytes, decodeErr := hex.DecodeString(blockID)
	if decodeErr != nil {
		return explorer.Block{}, decodeErr
	}
	var hash common.Hash32B
	copy(hash[:], bytes)

	blk, getErr := exp.bc.GetBlockByHash(hash)
	if getErr != nil {
		return explorer.Block{}, getErr
	}

	blkHeaderPb := blk.ConvertToBlockHeaderPb()

	totalAmount := int64(0)
	totalSize := uint32(0)
	for _, transfer := range blk.Transfers {
		totalAmount += transfer.Amount.Int64()
		totalSize += transfer.TotalSize()
	}

	explorerBlock := explorer.Block{
		ID:        blockID,
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
		actionNumber += (blk.Transfers + blk.Votes)
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

// getTransfer takes in a blockchain and transferHash and returns a Explorer Transfer
func getTransfer(bc blockchain.Blockchain, transferHash common.Hash32B) (explorer.Transfer, error) {
	explorerTransfer := explorer.Transfer{}

	transfer, getTransferErr := bc.GetTransferByTransferHash(transferHash)
	if getTransferErr != nil {
		return explorerTransfer, getTransferErr
	}

	blkHash, getBlkHashErr := bc.GetBlockHashByTransferHash(transferHash)
	if getBlkHashErr != nil {
		return explorerTransfer, getBlkHashErr
	}

	blk, getBlkErr := bc.GetBlockByHash(blkHash)
	if getBlkErr != nil {
		return explorerTransfer, getBlkErr
	}

	hash := transfer.Hash()
	explorerTransfer = explorer.Transfer{
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
