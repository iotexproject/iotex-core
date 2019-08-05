package grpcutil

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-core/action"
)

// ConnectToEndpoint connect to endpoint
func ConnectToEndpoint(url string, secure bool) (*grpc.ClientConn, error) {
	endpoint := url
	if endpoint == "" {
		return nil, errors.New(`endpoint is empty`)
	}
	if !secure {
		return grpc.Dial(endpoint, grpc.WithInsecure())
	}
	return grpc.Dial(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
}

// GetReceiptByActionHash get receipt by action hash
func GetReceiptByActionHash(url string, secure bool, hash string) error {
	conn, err := ConnectToEndpoint(url, secure)
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	request := iotexapi.GetReceiptByActionRequest{ActionHash: hash}
	response, err := cli.GetReceiptByAction(context.Background(), &request)
	if err != nil {
		return err
	}
	if response.ReceiptInfo.Receipt.Status != action.SuccessReceiptStatus {
		return errors.New("action fail:" + hash)
	}
	return nil
}

// SendAction send action to endpoint
func SendAction(url string, secure bool, action *iotextypes.Action) error {
	conn, err := ConnectToEndpoint(url, secure)
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	req := &iotexapi.SendActionRequest{Action: action}
	if _, err = cli.SendAction(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// GetNonce get nonce of address
func GetNonce(url string, secure bool, address string) (nonce uint64, err error) {
	conn, err := ConnectToEndpoint(url, secure)
	if err != nil {
		return
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetAccountRequest{Address: address}
	response, err := cli.GetAccount(context.Background(), &request)
	if err != nil {
		return
	}
	nonce = response.AccountMeta.PendingNonce
	return
}
