// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state/factory"
)

// CoreService defines the interface of coreservice corresponding to api/coreservice.go
// This interface is used by mockgen for test purposes.
type CoreService interface {
	// Account returns the metadata of an account
	Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
	// ChainMeta returns blockchain metadata
	ChainMeta() (*iotextypes.ChainMeta, string, error)
	// ServerMeta gets the server metadata
	ServerMeta() (packageVersion string, packageCommitID string, gitStatus string, goVersion string, buildTime string)
	// SendAction is the API to send an action to blockchain.
	SendAction(ctx context.Context, in *iotextypes.Action) (string, error)
	// ReceiptByAction gets receipt with corresponding action hash
	ReceiptByAction(actHash hash.Hash256) (*action.Receipt, string, error)
	// ReadContract reads the state in a contract address specified by the slot
	ReadContract(ctx context.Context, callerAddr address.Address, sc *action.Execution) (string, *iotextypes.Receipt, error)
	// ReadState reads state on blockchain
	ReadState(protocolID string, height string, methodName []byte, arguments [][]byte) (*iotexapi.ReadStateResponse, error)
	// SuggestGasPrice suggests gas price
	SuggestGasPrice() (uint64, error)
	// EstimateGasForAction estimates gas for action
	EstimateGasForAction(in *iotextypes.Action) (uint64, error)
	// EpochMeta gets epoch metadata
	EpochMeta(epochNum uint64) (*iotextypes.EpochData, uint64, []*iotexapi.BlockProducerInfo, error)
	// RawBlocks gets raw block data
	RawBlocks(startHeight uint64, count uint64, withReceipts bool, withTransactionLogs bool) ([]*iotexapi.BlockInfo, error)
	// StreamBlocks streams blocks
	StreamBlocks(stream iotexapi.APIService_StreamBlocksServer) error
	// StreamLogs streams logs that match the filter condition
	StreamLogs(in *iotexapi.LogsFilter, stream iotexapi.APIService_StreamLogsServer) error
	// ElectionBuckets returns the native election buckets.
	ElectionBuckets(epochNum uint64) ([]*iotextypes.ElectionBucket, error)
	// ReceiptByActionHash returns receipt by action hash
	ReceiptByActionHash(h hash.Hash256) (*action.Receipt, error)
	// TransactionLogByActionHash returns transaction log by action hash
	TransactionLogByActionHash(actHash string) (*iotextypes.TransactionLog, error)
	// TransactionLogByBlockHeight returns transaction log by block height
	TransactionLogByBlockHeight(blockHeight uint64) (*iotextypes.BlockIdentifier, *iotextypes.TransactionLogs, error)

	// Start starts the API server
	Start() error
	// Stop stops the API server
	Stop() error
	// Actions returns actions within the range
	Actions(start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// Action returns action by action hash
	Action(actionHash string, checkPending bool) (*iotexapi.ActionInfo, error)
	// ActionsByAddress returns all actions associated with an address
	ActionsByAddress(addr address.Address, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// ActionByActionHash returns action by action hash
	ActionByActionHash(h hash.Hash256) (action.SealedEnvelope, hash.Hash256, uint64, uint32, error)
	// ActionsByBlock returns all actions in a block
	ActionsByBlock(blkHash string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// ActPoolActions returns the all Transaction Identifiers in the mempool
	ActPoolActions(actHashes []string) ([]*iotextypes.Action, error)
	// UnconfirmedActionsByAddress returns all unconfirmed actions in actpool associated with an address
	UnconfirmedActionsByAddress(address string, start uint64, count uint64) ([]*iotexapi.ActionInfo, error)
	// CalculateGasConsumption estimate gas consumption for actions except execution
	CalculateGasConsumption(intrinsicGas, payloadGas, payloadSize uint64) (uint64, error)
	// EstimateExecutionGasConsumption estimate gas consumption for execution action
	EstimateExecutionGasConsumption(ctx context.Context, sc *action.Execution, callerAddr address.Address) (uint64, error)
	// BlockMetas returns blockmetas response within the height range
	BlockMetas(start uint64, count uint64) ([]*iotextypes.BlockMeta, error)
	// BlockMetaByHash returns blockmeta response by block hash
	BlockMetaByHash(blkHash string) (*iotextypes.BlockMeta, error)
	// LogsInBlock filter logs in the block x
	LogsInBlock(filter *logfilter.LogFilter, blockNumber uint64) ([]*iotextypes.Log, error)
	// LogsInRange filter logs among [start, end] blocks
	LogsInRange(filter *logfilter.LogFilter, start, end, paginationSize uint64) ([]*iotextypes.Log, error)
	// EVMNetworkID returns the network id of evm
	EVMNetworkID() uint32
	// ChainID returns the chain id of evm
	ChainID() uint32
	// ReadContractStorage reads contract's storage
	ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error)

	// BlockChain returns the member bc
	BlockChain() blockchain.Blockchain
	// StateFactory return the member sf
	StateFactory() factory.Factory
	// BlockDao return the member dao
	BlockDao() blockdao.BlockDAO
	// Indexer return the member indexer
	Indexer() blockindex.Indexer
	// ActPool return the member ap
	ActPool() actpool.ActPool
	// Config return the member cfg
	Config() config.Config
	// Registry return the member registry
	Registry() *protocol.Registry
	// HasActionIndex return the member hasActionIndex
	HasActionIndex() bool
}
