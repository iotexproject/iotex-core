package cmd

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-core/pkg/log"
)

func TestChainHeight(t *testing.T) {
	r := require.New(t)
	var conn *grpc.ClientConn
	var err error
	grpcctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	endpoint := "api.nightly-cluster-2.iotex.one:443"
	log.L().Info("Server Addr", zap.String("endpoint", endpoint))
	if rawInjectCfg.insecure {
		conn, err = grpc.DialContext(grpcctx, endpoint, grpc.WithBlock(), grpc.WithInsecure())
	} else {
		conn, err = grpc.DialContext(grpcctx, endpoint, grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	r.NoError(err)
	log.L().Info("server connected")
	api := iotexapi.NewAPIServiceClient(conn)
	chainMeta, err := api.GetChainMeta(grpcctx, &iotexapi.GetChainMetaRequest{})
	r.NoError(err)
	log.L().Info("ChainMeta", zap.Uint64("height", chainMeta.ChainMeta.Height))
	blockRangeTime := 20 * time.Minute
	blockRangeSize := uint64(blockRangeTime / time.Second / 5)
	resp, err := api.GetBlockMetas(grpcctx, &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
				Start: chainMeta.ChainMeta.Height - blockRangeSize + 1,
				Count: blockRangeSize,
			},
		},
	})
	r.NoError(err)
	for _, blkMeta := range resp.BlkMetas {
		fmt.Printf("%d: %d actions, %s, %s\n", blkMeta.Height, blkMeta.NumActions, blkMeta.Timestamp.AsTime().Local().String(), blkMeta.ProducerAddress)
	}
	// resp2, err := api.GetActPoolActions(grpcctx, &iotexapi.GetActPoolActionsRequest{
	// 	ActionHashes: []string{"75ef053802fc887e0ad9f32f4f14dea49228d1e5eb7fee8e7308e9899fc9f5f6"},
	// })
	// r.NoError(err)
	// fmt.Print(resp2)
}

func TestFloat64(t *testing.T) {
	r := require.New(t)
	r.Equal(1.0, loadAtomicGasPriceMultiplier())
	storeAtomicGasPriceMultiplier(1.2)
	r.Equal(1.2, loadAtomicGasPriceMultiplier())
}

func TestPayload(t *testing.T) {
	key := 11
	value := 10
	method := "7f07cccd"
	txt := method + fmt.Sprintf("%064x", key) + fmt.Sprintf("%064x", value)
	data := "7f07cccd000000000000000000000000000000000000000000000000000000000000000b000000000000000000000000000000000000000000000000000000000000000a"
	r := require.New(t)
	r.Equal(txt, data)

	fmt.Println(hex.EncodeToString(executionPayloadGenerate()))
	fmt.Println(hex.EncodeToString(executionPayloadGenerate()))
	fmt.Println(hex.EncodeToString(executionPayloadGenerate()))
}
