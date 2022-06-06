package client

import (
	"context"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/abci/types"
)

// Client defines the interface for an ABCI client.
type Client interface {
	// service.Service
	types.Application

	Error() error
	Flush(context.Context) error
	// Echo(context.Context, string) (*types.ResponseEcho, error)
}

type localClient struct {
	// service.BaseService
	types.Application
}

var _ Client = (*localClient)(nil)

// NewLocalClient creates a local client, which will be directly calling the
// methods of the given app.
//
// The client methods ignore their context argument.
func NewLocalClient(logger *zap.Logger, app types.Application) Client {
	cli := &localClient{
		Application: app,
	}
	// cli.BaseService = *service.NewBaseService(logger, "localClient", cli)
	return cli
}

func (*localClient) OnStart(context.Context) error { return nil }
func (*localClient) OnStop()                       {}
func (*localClient) Error() error                  { return nil }
func (*localClient) Flush(context.Context) error   { return nil }
