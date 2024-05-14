package ws

import (
	"context"
	_ "embed" // used to embed contract abi
	"encoding/hex"
	"testing"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var (
	//go:embed contracts/abis/ProjectRegistrar.json
	projectRegistrarContractABI     []byte
	projectRegistrarContractAddress = "0x0625f40fE525F604875444cd6502b6C87c066cBB"
	//go:embed contracts/abis/W3bstreamProject.json
	projectStoreContractABI     []byte
	projectStoreContractAddress = "0x45b69d57a12011df99a8398F46CFfFBa0333E513"
)

func TestUpdateProjectConfigAndRetrieveReceipt(t *testing.T) {
	// tx := "7eb04f3ac9af45b25a98e8364af01eeae0ae2a03fc360fcb24fc8800a8968c19"
	// tx := "28721f293e841d5994c407ef453750153fa6fa81c687492d31190eb2015fb0fe"
	// tx := "1055f8f99ea84c4398029316c6037d18435444d65421dd7c4cfda60d7a1a7ce4"
	// tx := "3cfbde68ec4b5c30198153a9b858cfd8461a4d4bf071c55e67d81a143ea7b353" // W3bstreamProver.setMinter
	tx := "e07d583d9c725960d09c362f1e3aa7dc3d41d72f9655f7747f20ee35307c44d2"
	config.ReadConfig.Endpoint = "api.testnet.iotex.one:443"
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		t.Fatal(errors.Wrapf(err, "failed to connect to chain endpoint: %s", config.ReadConfig.Endpoint))
	}

	client := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	if metadata, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(metadata).ToOutgoing(ctx)
	}
	response, err := client.GetReceiptByAction(ctx, &iotexapi.GetReceiptByActionRequest{ActionHash: tx})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hex.EncodeToString(response.ReceiptInfo.Receipt.ActHash))
	t.Log(response.ReceiptInfo.Receipt.Status)
}
