package explorer

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/web3api"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

const (
	PendingBlockNumber  = int64(-2)
	LatestBlockNumber   = int64(-1)
	EarliestBlockNumber = int64(0)
)

// Web3 provide web api for user to interact with blockchain
type PublicWeb3API struct {
	bc  blockchain.Blockchain
	p2p network.Overlay
}

// Web3ClientVersion returns the current client version
func (w3 *PublicWeb3API) Web3ClientVersion() (string, error) {
	panic("did not implement yet")
}

// Web3Sha3 returns Keccak-256 (not the standardized SHA3-256) of the given data
func (w3 *PublicWeb3API) Web3Sha3(input string) (string, error) {
	panic("did not implement yet")
}

// NetVersion returns the current net id
func (w3 *PublicWeb3API) NetVersion() (string, error) {
	panic("did not implement yet")
}

// NetListening returns whether client is actively listening for network connections
func (w3 *PublicWeb3API) NetListening() (bool, error) {
	panic("did not implement yet")
}

// NetPeerCount returns number of peers currently connected to the client
func (w3 *PublicWeb3API) NetPeerCount() (int64, error) {
	return int64(len(w3.p2p.GetPeers())), nil
}

// IotxProtocolVersion returns the current iotex protocol version
func (w3 *PublicWeb3API) IotxProtocolVersion() (string, error) {
	panic("did not implement yet")
}

// IotxSyncing returns an object with data about the sync status or false
func (w3 *PublicWeb3API) IotxSyncing() (string, error) {
	panic("did not implement yet")
}

// IotxCoinbase returns the client coinbase address
func (w3 *PublicWeb3API) IotxCoinbase() (string, error) {
	panic("did not implement yet")
}

// IotxMining returns true if client is actively mining new blocks
func (w3 *PublicWeb3API) IotxMining() (string, error) {
	panic("did not implement yet")
}

// IotxHashRate returns the number of hashes per second that the node is mining with
func (w3 *PublicWeb3API) IotxHashRate() (string, error) {
	panic("we don't have this since we use DPOS")
}

// IotxGasPrice returns the current price per gas
func (w3 *PublicWeb3API) IotxGasPrice() (int64, error) {
	panic("did not implement yet")
}

// IotxAccounts returns a list of addresses owned by client
func (w3 *PublicWeb3API) IotxAccounts() ([]string, error) {
	panic("we don't have this functionality")
}

// IotxBlockNumber returns the number of most recent block
func (w3 *PublicWeb3API) IotxBlockNumber() (int64, error) {
	tip := w3.bc.TipHeight()
	return int64(tip), nil
}

// IotxGetBalance returns the balance of the account of given address
// position can be a block number or 'latest', 'earliest' and 'pending'
func (w3 *PublicWeb3API) IotxGetBalance(address string, blockNumber int64) (string, error) {
	if blockNumber == LatestBlockNumber {
		state, err := w3.bc.StateByAddr(address)
		if err != nil {
			return "", err
		}
		return "0x" + state.Balance.Text(16), nil
	}
	return "", errors.Errorf("check balance for block number %d is not supported", blockNumber)
}

// IotxGetStorageAt returns the value from a storage position at a given address
func (w3 *PublicWeb3API) IotxGetStorageAt(address string, key int64, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// IotxGetTransferCount returns the number of transfers sent from an address
func (w3 *PublicWeb3API) IotxGetTransferCount(address string, blockNumber int64) (int64, error) {
	transferCount := int64(0)
	if blockNumber == LatestBlockNumber {
		transfersFromAddress, err := w3.bc.GetTransfersFromAddress(address)
		if err != nil {
			return 0, err
		}
		transferCount += int64(len(transfersFromAddress))

		transfersToAddress, err := w3.bc.GetTransfersToAddress(address)
		if err != nil {
			return 0, err
		}
		transferCount += int64(len(transfersToAddress))
	}
	return transferCount, errors.Errorf("check balance for block number %d is not supported", blockNumber)
}

// IotxGetBlockTransferCountByHash returns the number of transfers in a block from a
// block matching the given block hash
func (w3 *PublicWeb3API) IotxGetBlockTransferCountByHash(blockHash string) (int64, error) {
	bytes, err := hex.DecodeString(blockHash)
	if err != nil {
		return 0, err
	}
	var hash hash.Hash32B
	copy(hash[:], bytes)

	blk, err := w3.bc.GetBlockByHash(hash)
	if err != nil {
		return 0, err
	}
	return int64(len(blk.Transfers)), nil
}

// IotxGetBlockTransferCountByNumber returns the number of transfers in a block matching
// the given block number
func (w3 *PublicWeb3API) IotxGetBlockTransferCountByNumber(blockNumber int64) (int64, error) {
	if blockNumber == PendingBlockNumber {
		return 0, errors.New("get block transfer count for pending block is not supported")
	}

	height := uint64(blockNumber)
	if blockNumber == LatestBlockNumber {
		height = w3.bc.TipHeight()
	}

	blk, err := w3.bc.GetBlockByHeight(height)
	if err != nil {
		return 0, err
	}
	return int64(len(blk.Transfers)), nil
}

// IotxGetUncleCountByBlockHash returns the number of uncles in a block from a block matching
// the given block hash
func (w3 *PublicWeb3API) IotxGetUncleCountByBlockHash(blockHash string) (int64, error) {
	panic("did not implement yet")
}

// IotxGetUncleCountByBlockNumber returns the number of uncles in a block from a block matching
// the given block number
func (w3 *PublicWeb3API) IotxGetUncleCountByBlockNumber(blockNumber int64) (int64, error) {
	panic("did not implement yet")
}

// IotxGetCode returns code at a given address
func (w3 *PublicWeb3API) IotxGetCode(address string, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// IotxSign returns an Ethereum specific signature with: sign(keccak256("\x19Ethereum Signed
// Message:\n" + len(message) + message)))
func (w3 *PublicWeb3API) IotxSign(address string, data string) (string, error) {
	panic("did not implement yet")
}

// IotxSendTransaction creates new message call transfer or a contract creation, if the
// data field contains code
func (w3 *PublicWeb3API) IotxSendTransfer(args web3api.SendTxArgs) (string, error) {
	panic("did not implement yet")
}

// IotxSendRawTransaction creates new message call transfer or a contract creation for
// signed transactions
func (w3 *PublicWeb3API) IotxSendRawTransfer(encodedTx string) (string, error) {
	panic("did not implement yet")
}

// IotxCall executes a new message call immediately without creating a transfer on the block chain
func (w3 *PublicWeb3API) IotxCall(args web3api.CallArgs, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// IotxEstimateGas generates and returns an estimate of how much gas is necessary to allow the
// transaction to complete. The transaction will not be added to the blockchain. Note that the
// estimate may be significantly more than the amount of gas actually used by the transaction,
// for a variety of reasons including EVM mechanics and node performance
func (w3 *PublicWeb3API) IotxEstimateGas(args web3api.CallArgs) (int64, error) {
	panic("did not implement yet")
}

// IotxGetBlockByHash returns information about a block by hash
func (w3 *PublicWeb3API) IotxGetBlockByHash(blockHash string) (web3api.Block, error) {
	panic("did not implement yet")
}

// IotxGetBlockHashByHash returns information about a block hash by hash
func (w3 *PublicWeb3API) IotxGetBlockHashByHash(blockHash string) (string, error) {
	panic("did not implement yet")
}

// IotxGetBlockByNumber returns information about a block by block number
func (w3 *PublicWeb3API) IotxGetBlockByNumber(blockNumber int64) (web3api.Block, error) {
	panic("did not implement yet")
}

// IotxGetBlockHashByNumber returns information about a block hash by block number
func (w3 *PublicWeb3API) IotxGetBlockHashByNumber(blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// IotxGetTransferByHash returns the information about a transfer requested by transaction hash
func (w3 *PublicWeb3API) IotxGetTransferByHash(hash string) (web3api.Transfer, error) {
	panic("did not implement yet")
}

// IotxGetTransferByBlockHashAndIndex returns information about a transfer by block hash and
// transaction index position
func (w3 *PublicWeb3API) IotxGetTransferByBlockHashAndIndex(blockHash string, index int64) (web3api.Transfer, error) {
	panic("did not implement yet")
}

// IotxGetTransferByBlockNumberAndIndex returns information about a transfer by block number
// and transaction index position
func (w3 *PublicWeb3API) IotxGetTransferByBlockNumberAndIndex(blockNumber int64, index int64) (web3api.Transfer, error) {
	panic("did not implement yet")
}

// IotxGetTransferReceipt returns the receipt of a transfer by transfer hash
func (w3 *PublicWeb3API) IotxGetTransferReceipt(hash string) (web3api.TransactionReceipt, error) {
	panic("did not implement yet")
}

// IotxGetUncleByBlockHashAndIndex returns information about a uncle of a block by hash and uncle index position
func (w3 *PublicWeb3API) IotxGetUncleByBlockHashAndIndex(blockHash string, index int64) (web3api.Block, error) {
	panic("did not implement yet")
}

// IotxGetUncleByBlockNumberAndIndex returns information about a uncle of a block by number and uncle index position
func (w3 *PublicWeb3API) IotxGetUncleByBlockNumberAndIndex(blockNumber int64, uncleIndex int64) (web3api.Block, error) {
	panic("did not implement yet")
}
