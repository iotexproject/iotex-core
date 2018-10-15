package explorer

import (
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/web3api"
)

const (
	PendingBlockNumber  = int64(-2)
	LatestBlockNumber   = int64(-1)
	EarliestBlockNumber = int64(0)
)

// Web3 provide web api for user to interact with blockchain
type PublicWeb3API struct {
	bc blockchain.Blockchain
}

// ClientVersion returns the current client version
func (w3 *PublicWeb3API) ClientVersion() (string, error) {
	panic("did not implement yet")
}

// Sha3 returns Keccak-256 (not the standardized SHA3-256) of the given data
func (w3 *PublicWeb3API) Sha3(input string) (string, error) {
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
	panic("did not implement yet")
}

// ProtocolVersion returns the current iotex protocol version
func (w3 *PublicWeb3API) ProtocolVersion() (string, error) {
	panic("did not implement yet")
}

// Syncing returns an object with data about the sync status or false
func (w3 *PublicWeb3API) Syncing() (string, error) {
	panic("did not implement yet")
}

// Coinbase returns the client coinbase address
func (w3 *PublicWeb3API) Coinbase() (string, error) {
	panic("did not implement yet")
}

// Mining returns true if client is actively mining new blocks
func (w3 *PublicWeb3API) Mining() (string, error) {
	panic("did not implement yet")
}

// HashRate returns the number of hashes per second that the node is mining with
func (w3 *PublicWeb3API) HashRate() (string, error) {
	panic("we don't have this since we use DPOS")
}

// GasPrice returns the current price per gas
func (w3 *PublicWeb3API) GasPrice() (int64, error) {
	panic("did not implement yet")
}

// Accounts returns a list of addresses owned by client
func (w3 *PublicWeb3API) Accounts() ([]string, error) {
	panic("we don't have this functionality")
}

// BlockNumber returns the number of most recent block
func (w3 *PublicWeb3API) BlockNumber() (int64, error) {
	panic("did not implement yet")
}

// GetBalance returns the balance of the account of given address
// position can be a block number or 'latest', 'earliest' and 'pending'
func (w3 *PublicWeb3API) GetBalance(address string, blockNumber int64) (uint64, error) {
	panic("did not implement yet")
}

// GetStorageAt returns the value from a storage position at a given address
func (w3 *PublicWeb3API) GetStorageAt(address string, key int64, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// GetTransactionCount returns the number of transactions sent from an address
func (w3 *PublicWeb3API) GetTransactionCount(address string, blockNumber int64) (int64, error) {
	panic("did not implement yet")
}

// GetBlockTransactionCountByHash returns the number of transactions in a block from a
// block matching the given block hash
func (w3 *PublicWeb3API) GetBlockTransactionCountByHash(blockHash string) (int64, error) {
	panic("did not implement yet")
}

// GetBlockTransactionCountByNumber returns the number of transactions in a block matching
// the given block number
func (w3 *PublicWeb3API) GetBlockTransactionCountByNumber(blockNumber int64) (int64, error) {
	panic("did not implement yet")
}

// GetUncleCountByBlockHash returns the number of uncles in a block from a block matching
// the given block hash
func (w3 *PublicWeb3API) GetUncleCountByBlockHash(blockHash string) (int64, error) {
	panic("did not implement yet")
}

// GetUncleCountByBlockNumber returns the number of uncles in a block from a block matching
// the given block number
func (w3 *PublicWeb3API) GetUncleCountByBlockNumber(blockNumber int64) (int64, error) {
	panic("did not implement yet")
}

// GetCode returns code at a given address
func (w3 *PublicWeb3API) GetCode(address string, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// Sign returns an Ethereum specific signature with: sign(keccak256("\x19Ethereum Signed
// Message:\n" + len(message) + message)))
func (w3 *PublicWeb3API) Sign(address string, data string) (string, error) {
	panic("did not implement yet")
}

// SendTransaction creates new message call transaction or a contract creation, if the
// data field contains code
func (w3 *PublicWeb3API) SendTransaction(args web3api.SendTxArgs) (string, error) {
	panic("did not implement yet")
}

// SendRawTransaction creates new message call transaction or a contract creation for
// signed transactions
func (w3 *PublicWeb3API) SendRawTransaction(encodedTx string) (string, error) {
	panic("did not implement yet")
}

// Call executes a new message call immediately without creating a transaction on the block chain
func (w3 *PublicWeb3API) Call(args web3api.CallArgs, blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// EstimateGas generates and returns an estimate of how much gas is necessary to allow the
// transaction to complete. The transaction will not be added to the blockchain. Note that the
// estimate may be significantly more than the amount of gas actually used by the transaction,
// for a variety of reasons including EVM mechanics and node performance
func (w3 *PublicWeb3API) EstimateGas(args web3api.CallArgs) (int64, error) {
	panic("did not implement yet")
}

// GetBlockByHash returns information about a block by hash
func (w3 *PublicWeb3API) GetBlockByHash(blockHash string) (web3api.Block, error) {
	panic("did not implement yet")
}

// GetBlockHashByHash returns information about a block hash by hash
func (w3 *PublicWeb3API) GetBlockHashByHash(blockHash string) (string, error) {
	panic("did not implement yet")
}

// GetBlockByNumber returns information about a block by block number
func (w3 *PublicWeb3API) GetBlockByNumber(blockNumber int64) (web3api.Block, error) {
	panic("did not implement yet")
}

// GetBlockHashByNumber returns information about a block hash by block number
func (w3 *PublicWeb3API) GetBlockHashByNumber(blockNumber int64) (string, error) {
	panic("did not implement yet")
}

// GetTransactionByHash returns the information about a transaction requested by transaction hash
func (w3 *PublicWeb3API) GetTransactionByHash(hash string) (web3api.Transaction, error) {
	panic("did not implement yet")
}

// GetTransactionByBlockHashAndIndex returns information about a transaction by block hash and
// transaction index position
func (w3 *PublicWeb3API) GetTransactionByBlockHashAndIndex(blockHash string, index int64) (web3api.Transaction, error) {
	panic("did not implement yet")
}

// GetTransactionByBlockNumberAndIndex returns information about a transaction by block number
// and transaction index position
func (w3 *PublicWeb3API) GetTransactionByBlockNumberAndIndex(blockNumber int64, index int64) (web3api.Transaction, error) {
	panic("did not implement yet")
}

// GetTransactionReceipt returns the receipt of a transaction by transaction hash
func (w3 *PublicWeb3API) GetTransactionReceipt(hash string) (web3api.TransactionReceipt, error) {
	panic("did not implement yet")
}

// GetUncleByBlockHashAndIndex returns information about a uncle of a block by hash and uncle index position
func (w3 *PublicWeb3API) GetUncleByBlockHashAndIndex(blockHash string, index int64) (web3api.Block, error) {
	panic("did not implement yet")
}

// GetUncleByBlockNumberAndIndex returns information about a uncle of a block by number and uncle index position
func (w3 *PublicWeb3API) GetUncleByBlockNumberAndIndex(blockNumber int64, uncleIndex int64) (web3api.Block, error) {
	panic("did not implement yet")
}
