package api

import (
	"context"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"google.golang.org/grpc"
)

// ServiceClient is the api service client interface corresponding to the one in github.com/iotexproject/iotex-proto.
// This interface is used by mockgen for test purposes.
// Remember to update it whenever definitions in api.proto change.
type ServiceClient interface {
	// get the address detail of an address
	GetAccount(ctx context.Context, in *iotexapi.GetAccountRequest, opts ...grpc.CallOption) (*iotexapi.GetAccountResponse, error)
	// get action(s) by:
	// 1. start index and action count
	// 2. action hash
	// 3. address with start index and action count
	// 4. get unconfirmed actions by address with start index and action count
	// 5. block hash with start index and action count
	GetActions(ctx context.Context, in *iotexapi.GetActionsRequest, opts ...grpc.CallOption) (*iotexapi.GetActionsResponse, error)
	// get block metadata(s) by:
	// 1. start index and block count
	// 2. block hash
	GetBlockMetas(ctx context.Context, in *iotexapi.GetBlockMetasRequest, opts ...grpc.CallOption) (*iotexapi.GetBlockMetasResponse, error)
	// get chain metadata
	GetChainMeta(ctx context.Context, in *iotexapi.GetChainMetaRequest, opts ...grpc.CallOption) (*iotexapi.GetChainMetaResponse, error)
	// get server version
	GetServerMeta(ctx context.Context, in *iotexapi.GetServerMetaRequest, opts ...grpc.CallOption) (*iotexapi.GetServerMetaResponse, error)
	// sendAction
	SendAction(ctx context.Context, in *iotexapi.SendActionRequest, opts ...grpc.CallOption) (*iotexapi.SendActionResponse, error)
	// get receipt by action Hash
	GetReceiptByAction(ctx context.Context, in *iotexapi.GetReceiptByActionRequest, opts ...grpc.CallOption) (*iotexapi.GetReceiptByActionResponse, error)
	// TODO: read contract
	ReadContract(ctx context.Context, in *iotexapi.ReadContractRequest, opts ...grpc.CallOption) (*iotexapi.ReadContractResponse, error)
	// suggest gas price
	SuggestGasPrice(ctx context.Context, in *iotexapi.SuggestGasPriceRequest, opts ...grpc.CallOption) (*iotexapi.SuggestGasPriceResponse, error)
	// estimate gas for action
	EstimateGasForAction(ctx context.Context, in *iotexapi.EstimateGasForActionRequest, opts ...grpc.CallOption) (*iotexapi.EstimateGasForActionResponse, error)
	// estimate gas for action and transfer not sealed
	EstimateActionGasConsumption(ctx context.Context, in *iotexapi.EstimateActionGasConsumptionRequest, opts ...grpc.CallOption) (*iotexapi.EstimateActionGasConsumptionResponse, error)
	// read state from blockchain
	ReadState(ctx context.Context, in *iotexapi.ReadStateRequest, opts ...grpc.CallOption) (*iotexapi.ReadStateResponse, error)
	// get epoch metadata
	GetEpochMeta(ctx context.Context, in *iotexapi.GetEpochMetaRequest, opts ...grpc.CallOption) (*iotexapi.GetEpochMetaResponse, error)
	// get raw blocks data
	GetRawBlocks(ctx context.Context, in *iotexapi.GetRawBlocksRequest, opts ...grpc.CallOption) (*iotexapi.GetRawBlocksResponse, error)
	// get logs filtered by contract address and topics
	GetLogs(ctx context.Context, in *iotexapi.GetLogsRequest, opts ...grpc.CallOption) (*iotexapi.GetLogsResponse, error)
	// deprecated
	GetEvmTransfersByActionHash(ctx context.Context, in *iotexapi.GetEvmTransfersByActionHashRequest, opts ...grpc.CallOption) (*iotexapi.GetEvmTransfersByActionHashResponse, error)
	// deprecated
	GetEvmTransfersByBlockHeight(ctx context.Context, in *iotexapi.GetEvmTransfersByBlockHeightRequest, opts ...grpc.CallOption) (*iotexapi.GetEvmTransfersByBlockHeightResponse, error)
	// get transaction log by action hash
	GetTransactionLogByActionHash(ctx context.Context, in *iotexapi.GetTransactionLogByActionHashRequest, opts ...grpc.CallOption) (*iotexapi.GetTransactionLogByActionHashResponse, error)
	// get transaction log by block height
	GetTransactionLogByBlockHeight(ctx context.Context, in *iotexapi.GetTransactionLogByBlockHeightRequest, opts ...grpc.CallOption) (*iotexapi.GetTransactionLogByBlockHeightResponse, error)
	// get block info in stream
	StreamBlocks(ctx context.Context, in *iotexapi.StreamBlocksRequest, opts ...grpc.CallOption) (iotexapi.APIService_StreamBlocksClient, error)
	// get filtered logs in stream
	StreamLogs(ctx context.Context, in *iotexapi.StreamLogsRequest, opts ...grpc.CallOption) (iotexapi.APIService_StreamLogsClient, error)
	// get native election buckets
	GetElectionBuckets(ctx context.Context, in *iotexapi.GetElectionBucketsRequest, opts ...grpc.CallOption) (*iotexapi.GetElectionBucketsResponse, error)
	// get actions from act pool
	GetActPoolActions(ctx context.Context, in *iotexapi.GetActPoolActionsRequest, opts ...grpc.CallOption) (*iotexapi.GetActPoolActionsResponse, error)
	// read contract storage
	ReadContractStorage(ctx context.Context, in *iotexapi.ReadContractStorageRequest, opts ...grpc.CallOption) (*iotexapi.ReadContractStorageResponse, error)
	// TraceTransactionStructLogs get trace transaction struct logs
	TraceTransactionStructLogs(ctx context.Context, in *iotexapi.TraceTransactionStructLogsRequest, opts ...grpc.CallOption) (*iotexapi.TraceTransactionStructLogsResponse, error)
}
