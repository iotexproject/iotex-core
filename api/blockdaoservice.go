// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/blockdao/blockdaopb"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/types/known/emptypb"
)

type blockDAOService struct {
	dao blockdao.BlockDAO
}

func newBlockDAOService(dao blockdao.BlockDAO) *blockDAOService {
	return &blockDAOService{
		dao: dao,
	}
}

func (service *blockDAOService) Height(context.Context, *emptypb.Empty) (*blockdaopb.BlockHeightResponse, error) {
	height, err := service.dao.Height()
	if err != nil {
		return nil, err
	}
	return &blockdaopb.BlockHeightResponse{
		Height: height,
	}, nil
}

func (service *blockDAOService) GetBlockHash(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.BlockHashResponse, error) {
	h, err := service.dao.GetBlockHash(request.Height)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.BlockHashResponse{
		Hash: hex.EncodeToString(h[:]),
	}, nil
}

func (service *blockDAOService) GetBlockHeight(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.BlockHeightResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	height, err := service.dao.GetBlockHeight(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.BlockHeightResponse{
		Height: height,
	}, nil

}

func (service *blockDAOService) GetBlock(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.GetBlockResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	blk, err := service.dao.GetBlock(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.GetBlockResponse{
		Block: blk.ConvertToBlockPb(),
	}, nil
}

func (service *blockDAOService) GetBlockByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.GetBlockResponse, error) {
	blk, err := service.dao.GetBlockByHeight(request.Height)
	if err != nil {
		return nil, err
	}
	return &blockdaopb.GetBlockResponse{
		Block: blk.ConvertToBlockPb(),
	}, nil
}

func (service *blockDAOService) GetReceipts(_ context.Context, request *blockdaopb.BlockHeightRequest) (*iotextypes.Receipts, error) {
	receipts, err := service.dao.GetReceipts(request.Height)
	if err != nil {
		return nil, err
	}
	arr := make([]*iotextypes.Receipt, 0, len(receipts))
	for _, receipt := range receipts {
		arr = append(arr, receipt.ConvertToReceiptPb())
	}

	return &iotextypes.Receipts{
		Receipts: arr,
	}, nil
}

func (service *blockDAOService) ContainsTransactionLog(context.Context, *emptypb.Empty) (*blockdaopb.ContainsTransactionLogResponse, error) {
	return &blockdaopb.ContainsTransactionLogResponse{
		Yes: service.dao.ContainsTransactionLog(),
	}, nil
}

func (service *blockDAOService) TransactionLogs(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.TransactionLogsResponse, error) {
	logs, err := service.dao.TransactionLogs(request.Height)
	if err != nil {
		return nil, err
	}
	return &blockdaopb.TransactionLogsResponse{
		TransactionLogs: logs,
	}, nil
}

func (service *blockDAOService) Header(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.HeaderResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	header, err := service.dao.Header(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.HeaderResponse{
		Header: header.Proto(),
	}, nil

}

func (service *blockDAOService) HeaderByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.HeaderResponse, error) {
	header, err := service.dao.HeaderByHeight(request.Height)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.HeaderResponse{
		Header: header.Proto(),
	}, nil
}

func (service *blockDAOService) FooterByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.FooterResponse, error) {
	footer, err := service.dao.FooterByHeight(request.Height)
	if err != nil {
		return nil, err
	}
	footerpb, err := footer.ConvertToBlockFooterPb()
	if err != nil {
		return nil, err
	}

	return &blockdaopb.FooterResponse{
		Footer: footerpb,
	}, nil
}
