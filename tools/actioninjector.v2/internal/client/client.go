package client

import (
	"context"

	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
)

// Client is the blockchain API client.
type Client struct {
	api iotexapi.APIServiceClient
}

// New creates a new Client.
func New(serverAddr string) (*Client, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Client{
		api: iotexapi.NewAPIServiceClient(conn),
	}, nil
}

// SendAction sends an action to blockchain.
func (c *Client) SendAction(ctx context.Context, selp action.SealedEnvelope) error {
	_, err := c.api.SendAction(ctx, &iotexapi.SendActionRequest{Action: selp.Proto()})
	return err
}

// GetAccount returns a given account.
func (c *Client) GetAccount(ctx context.Context, addr string) (*iotexapi.GetAccountResponse, error) {
	return c.api.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
}
