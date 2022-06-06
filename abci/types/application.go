package types

import (
	"context"

	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/api"
)

// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
type Application interface {
	CheckTx(ctx context.Context, in *api.RequestCheckTx, opts ...grpc.CallOption) (*api.ResponseCheckTx, error)
}
