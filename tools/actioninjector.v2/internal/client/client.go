package client

import (
	"context"
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

// Client is the blockchain API client.
type Client struct {
	api iotexapi.APIServiceClient
}

// New creates a new Client.
func New(serverAddr string, insecure bool) (*Client, error) {
	grpcctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	var err error

	if insecure {
		log.L().Info("insecure connection")
		conn, err = grpc.DialContext(grpcctx, serverAddr, grpc.WithBlock(), grpc.WithInsecure())
	} else {
		log.L().Info("secure connection")
		conn, err = grpc.DialContext(grpcctx, serverAddr, grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	}
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
