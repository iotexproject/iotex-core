package abci

import (
	"context"

	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/api"
)

type Abci interface {
	CheckTx(ctx context.Context, in *api.RequestCheckTx, opts ...grpc.CallOption) (*api.ResponseCheckTx, error)
}
