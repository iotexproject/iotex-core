// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao/blockdaopb"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type grpcBlockDAO struct {
	url                    string
	insecure               bool
	conn                   *grpc.ClientConn
	client                 blockdaopb.BlockDAOServiceClient
	containsTransactionLog bool
	deserializer           *block.Deserializer
	cache                  *memoryDao
}

var (
	// ErrRemoteHeightTooLow is the error that remote height is too low
	ErrRemoteHeightTooLow = fmt.Errorf("remote height is too low")
	// ErrAlreadyExist is the error that block already exists
	ErrAlreadyExist = fmt.Errorf("block already exists")
)

// NewGrpcBlockDAO returns a GrpcBlockDAO instance
func NewGrpcBlockDAO(
	url string,
	insecure bool,
	deserializer *block.Deserializer,
	cacheSize uint64,
) BlockStore {
	return &grpcBlockDAO{
		url:          url,
		insecure:     insecure,
		deserializer: deserializer,
		cache:        newMemoryDao(cacheSize),
	}
}

func (gbd *grpcBlockDAO) Start(ctx context.Context) error {
	log.L().Debug("Starting gRPC block DAO...", zap.String("url", gbd.url))
	var err error
	opts := []grpc.DialOption{}
	if gbd.insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	}
	gbd.conn, err = grpc.NewClient(gbd.url, opts...)
	if err != nil {
		return err
	}
	gbd.client = blockdaopb.NewBlockDAOServiceClient(gbd.conn)

	response, err := gbd.client.ContainsTransactionLog(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	gbd.containsTransactionLog = response.Yes
	// init local height with remote height
	height, err := gbd.rpcHeight()
	if err != nil {
		return err
	}
	blk, err := gbd.blockByHeight(height)
	if err != nil {
		return err
	}

	// NOTE: it won't work correctly if block height is zero
	return gbd.cache.PutBlock(blk)
}

func (gbd *grpcBlockDAO) Stop(ctx context.Context) error {
	return gbd.conn.Close()
}

func (gbd *grpcBlockDAO) Height() (uint64, error) {
	return gbd.cache.TipHeight(), nil
}

func (gbd *grpcBlockDAO) rpcHeight() (uint64, error) {
	response, err := gbd.client.Height(context.Background(), &emptypb.Empty{})
	if err != nil {
		return 0, err
	}
	return response.Height, nil
}

func (gbd *grpcBlockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	h, ok := gbd.cache.BlockHash(height)
	if ok {
		return h, nil
	}
	response, err := gbd.client.GetBlockHash(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return hash.ZeroHash256, err
	}
	h, err = hash.HexStringToHash256(response.Hash)
	if err != nil {
		return hash.ZeroHash256, err
	}
	return h, nil
}

func (gbd *grpcBlockDAO) GetBlockHeight(h hash.Hash256) (uint64, error) {
	height, ok := gbd.cache.BlockHeight(h)
	if ok {
		return height, nil
	}
	response, err := gbd.client.GetBlockHeight(context.Background(), &blockdaopb.BlockHashRequest{
		Hash: hex.EncodeToString(h[:]),
	})
	if err != nil {
		return 0, err
	}

	return response.Height, nil
}

func (gbd *grpcBlockDAO) GetBlock(h hash.Hash256) (*block.Block, error) {
	blk, ok := gbd.cache.BlockByHash(h)
	if ok {
		return blk, nil
	}
	response, err := gbd.client.GetBlock(context.Background(), &blockdaopb.BlockHashRequest{
		Hash: hex.EncodeToString(h[:]),
	})
	if err != nil {
		return nil, err
	}

	return gbd.deserializer.FromBlockProto(response.Block)
}

func (gbd *grpcBlockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	if height == 0 {
		return block.GenesisBlock(), nil
	}
	blk, ok := gbd.cache.BlockByHeight(height)
	if ok {
		return blk, nil
	}

	return gbd.blockByHeight(height)
}

func (gbd *grpcBlockDAO) blockByHeight(height uint64) (*block.Block, error) {
	response, err := gbd.client.GetBlockByHeight(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	return gbd.deserializer.FromBlockProto(response.Block)
}

func (gbd *grpcBlockDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	response, err := gbd.client.GetReceipts(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	receipts := make([]*action.Receipt, 0, len(response.Receipts))
	for _, receiptpb := range response.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptpb)
		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

func (gbd *grpcBlockDAO) ContainsTransactionLog() bool {
	return gbd.containsTransactionLog
}

func (gbd *grpcBlockDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	response, err := gbd.client.TransactionLogs(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	return response.TransactionLogs, nil
}

func (gbd *grpcBlockDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	remoteHeight, err := gbd.rpcHeight()
	if err != nil {
		return err
	}
	if blk.Height() <= remoteHeight {
		// remote block is already exist
		return gbd.cache.PutBlock(blk)
	}
	return errors.Wrapf(ErrRemoteHeightTooLow, "block height %d, remote height %d", blk.Height(), remoteHeight)
}

func (gbd *grpcBlockDAO) Header(h hash.Hash256) (*block.Header, error) {
	if header, ok := gbd.cache.HeaderByHash(h); ok {
		return header, nil
	}
	response, err := gbd.client.Header(context.Background(), &blockdaopb.BlockHashRequest{
		Hash: hex.EncodeToString(h[:]),
	})
	if err != nil {
		return nil, err
	}
	header := &block.Header{}
	if err := header.LoadFromBlockHeaderProto(response.Header); err != nil {
		return nil, err
	}
	return header, nil
}

func (gbd *grpcBlockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	if header, ok := gbd.cache.HeaderByHeight(height); ok {
		return header, nil
	}
	response, err := gbd.client.HeaderByHeight(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}
	header := &block.Header{}
	if err := header.LoadFromBlockHeaderProto(response.Header); err != nil {
		return nil, err
	}
	return header, nil
}

func (gbd *grpcBlockDAO) FooterByHeight(height uint64) (*block.Footer, error) {
	if footer, ok := gbd.cache.FooterByHeight(height); ok {
		return footer, nil
	}
	response, err := gbd.client.FooterByHeight(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}
	footer := &block.Footer{}
	if err := footer.ConvertFromBlockFooterPb(response.Footer); err != nil {
		return nil, err
	}
	return footer, nil
}
