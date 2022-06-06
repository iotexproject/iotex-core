package local

import (
	"context"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/infra/rpc/coretypes"
	"github.com/iotexproject/iotex-core/infra/types"
)

/*
Local is a Client implementation that directly executes the rpc
functions on a given node, without going through HTTP or GRPC.

This implementation is useful for:

* Running tests against a node in-process without the overhead
of going through an http server
* Communication between an ABCI app and Tendermint core when they
are compiled in process.

For real clients, you probably want to use client.HTTP.  For more
powerful control during testing, you probably want the "client/mock" package.

You can subscribe for any event published by Tendermint using Subscribe method.
Note delivery is best-effort. If you don't read events fast enough, Tendermint
might cancel the subscription. The client will attempt to resubscribe (you
don't need to do anything). It will keep trying indefinitely with exponential
backoff (10ms -> 20ms -> 40ms) until successful.
*/
type Local struct {
	Logger *zap.Logger
	// env    *rpccore.Environment
}

// NodeService describes the portion of the node interface that the
// local RPC client constructor needs to build a local client.
type NodeService interface {
	// RPCEnvironment() *rpccore.Environment
	// EventBus() *eventbus.EventBus
}

// New configures a client that calls the Node directly.
func New(logger *zap.Logger, node NodeService) (*Local, error) {
	// env := node.RPCEnvironment()
	// if env == nil {
	// 	return nil, errors.New("rpc is nil")
	// }
	return &Local{
		// EventBus: node.EventBus(),
		Logger: logger,
		// env:    env,
	}, nil
}

// var _ rpcclient.Client = (*Local)(nil)

func (c *Local) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	// return c.env.CheckTx(ctx, &coretypes.RequestCheckTx{Tx: tx})
}
