package client

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	iotexapi "github.com/iotexproject/iotex-core/proto/api"
	"google.golang.org/grpc"
)

type Client struct {
	api iotexapi.APIServiceClient
}

func New(serverAddr string) (*Client, error) {
	conn, err := grpc.Dial(serverAddr)
	if err != nil {
		return nil, err
	}
	return &Client{
		api: iotexapi.NewAPIServiceClient(conn),
	}, nil
}

func (c *Client) SendAction(ctx context.Context, selp action.SealedEnvelope) error {
	_, err := c.api.SendAction(ctx, &iotexapi.SendActionRequest{Action: selp.Proto()})
	return err
}

func (c *Client) GetAccount(ctx context.Context, addr string) (*iotexapi.GetAccountResponse, error) {
	return c.api.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: addr})
}
