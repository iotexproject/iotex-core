// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"encoding/hex"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao/blockdaopb"
	"github.com/iotexproject/iotex-core/pkg/log"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type GrpcBlockDAO struct {
	url                    string
	conn                   *grpc.ClientConn
	client                 blockdaopb.BlockDAOServiceClient
	containsTransactionLog bool
	deserializer           *block.Deserializer
	localHeight            atomic.Uint64
}

func NewGrpcBlockDAO(
	url string,
	deserializer *block.Deserializer,
) *GrpcBlockDAO {
	return &GrpcBlockDAO{
		url:          url,
		deserializer: deserializer,
	}
}

func (gbd *GrpcBlockDAO) Start(ctx context.Context) error {
	var err error
	gbd.conn, err = grpc.Dial(gbd.url)
	if err != nil {
		return err
	}
	gbd.client = blockdaopb.NewBlockDAOServiceClient(gbd.conn)

	response, err := gbd.client.ContainsTransactionLog(ctx, nil)
	if err != nil {
		return err
	}
	gbd.containsTransactionLog = response.Yes
	// init local height with remote height
	height, err := gbd.Height()
	if err != nil {
		return err
	}
	gbd.localHeight.Store(height)
	return nil
}

func (gbd *GrpcBlockDAO) Stop(ctx context.Context) error {
	return gbd.conn.Close()
}

func (gbd *GrpcBlockDAO) Height() (uint64, error) {
	return gbd.localHeight.Load(), nil
}

func (gbd *GrpcBlockDAO) rpcHeight() (uint64, error) {
	response, err := gbd.client.Height(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	return response.Height, nil
}

func (gbd *GrpcBlockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	response, err := gbd.client.GetBlockHash(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return hash.ZeroHash256, err
	}
	h, err := hash.HexStringToHash256(response.Hash)
	if err != nil {
		return hash.ZeroHash256, err
	}
	return h, nil
}

func (gbd *GrpcBlockDAO) GetBlockHeight(h hash.Hash256) (uint64, error) {
	response, err := gbd.client.GetBlockHeight(context.Background(), &blockdaopb.BlockHashRequest{
		Hash: hex.EncodeToString(h[:]),
	})
	if err != nil {
		return 0, err
	}

	return response.Height, nil
}

func (gbd *GrpcBlockDAO) GetBlock(h hash.Hash256) (*block.Block, error) {
	response, err := gbd.client.GetBlock(context.Background(), &blockdaopb.BlockHashRequest{
		Hash: hex.EncodeToString(h[:]),
	})
	if err != nil {
		return nil, err
	}

	return gbd.deserializer.FromBlockProto(response.Block)
}

func (gbd *GrpcBlockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	response, err := gbd.client.GetBlockByHeight(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	return gbd.deserializer.FromBlockProto(response.Block)
}

func (gbd *GrpcBlockDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
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

func (gbd *GrpcBlockDAO) ContainsTransactionLog() bool {
	return gbd.containsTransactionLog
}

func (gbd *GrpcBlockDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	response, err := gbd.client.TransactionLogs(context.Background(), &blockdaopb.BlockHeightRequest{
		Height: height,
	})
	if err != nil {
		return nil, err
	}

	return response.TransactionLogs, nil
}

func (gbd *GrpcBlockDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	localHeight := gbd.localHeight.Load()
	switch {
	case blk.Height() <= localHeight:
		return errors.Errorf("block height %d is less than or equal to local height %d", blk.Height(), localHeight)
	case blk.Height() > localHeight+1:
		return errors.Errorf("block height %d is larger than local height %d + 1", blk.Height(), localHeight)
	}

	remoteHeight, err := gbd.rpcHeight()
	if err != nil {
		return err
	}
	if blk.Height() <= remoteHeight {
		gbd.localHeight.Store(blk.Height())
		// remote block is already exist
		return nil
	}
	// wait for remote block to be synced
	select {
	case <-gbd.subscribeBlock(ctx, blk.Height()):
		gbd.localHeight.Store(blk.Height())
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (gbd *GrpcBlockDAO) subscribeBlock(ctx context.Context, height uint64) chan struct{} {
	ch := make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}

			remoteHeight, err := gbd.rpcHeight()
			if err != nil {
				log.L().Error("failed to get remote height", zap.Error(err))
				continue
			}
			if remoteHeight >= height {
				close(ch)
				return
			}
		}
	}()
	return ch
}

func (gbd *GrpcBlockDAO) Header(h hash.Hash256) (*block.Header, error) {
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

func (gbd *GrpcBlockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
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

func (gbd *GrpcBlockDAO) FooterByHeight(height uint64) (*block.Footer, error) {
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
