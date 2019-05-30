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
	// read state from blockchain
	ReadState(ctx context.Context, in *iotexapi.ReadStateRequest, opts ...grpc.CallOption) (*iotexapi.ReadStateResponse, error)
	// get epoch metadata
	GetEpochMeta(ctx context.Context, in *iotexapi.GetEpochMetaRequest, opts ...grpc.CallOption) (*iotexapi.GetEpochMetaResponse, error)
	// get raw blocks data
	GetRawBlocks(ctx context.Context, in *iotexapi.GetRawBlocksRequest, opts ...grpc.CallOption) (*iotexapi.GetRawBlocksResponse, error)
	// get block info in stream
	StreamBlocks(ctx context.Context, in *iotexapi.StreamBlocksRequest, opts ...grpc.CallOption) (iotexapi.APIService_StreamBlocksClient, error)
}
