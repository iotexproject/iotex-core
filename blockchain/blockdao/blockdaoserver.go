package blockdao

import (
	"context"
	"encoding/hex"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/blockdao/blockdaopb"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BlockDAOServer struct {
	dao BlockDAO
}

func (server *BlockDAOServer) Height(context.Context, *emptypb.Empty) (*blockdaopb.BlockHeightResponse, error) {
	height, err := server.dao.Height()
	if err != nil {
		return nil, err
	}
	return &blockdaopb.BlockHeightResponse{
		Height: height,
	}, nil
}

func (server *BlockDAOServer) GetBlockHash(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.BlockHashResponse, error) {
	h, err := server.dao.GetBlockHash(request.Height)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.BlockHashResponse{
		Hash: hex.EncodeToString(h[:]),
	}, nil
}

func (server *BlockDAOServer) GetBlockHeight(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.BlockHeightResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	height, err := server.dao.GetBlockHeight(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.BlockHeightResponse{
		Height: height,
	}, nil

}

func (server *BlockDAOServer) GetBlock(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.GetBlockResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	blk, err := server.dao.GetBlock(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.GetBlockResponse{
		Block: blk.ConvertToBlockPb(),
	}, nil
}

func (server *BlockDAOServer) GetBlockByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.GetBlockResponse, error) {
	blk, err := server.dao.GetBlockByHeight(request.Height)
	if err != nil {
		return nil, err
	}
	return &blockdaopb.GetBlockResponse{
		Block: blk.ConvertToBlockPb(),
	}, nil
}

func (server *BlockDAOServer) GetReceipts(_ context.Context, request *blockdaopb.BlockHeightRequest) (*iotextypes.Receipts, error) {
	receipts, err := server.dao.GetReceipts(request.Height)
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

func (server *BlockDAOServer) ContainsTransactionLog(context.Context, *emptypb.Empty) (*blockdaopb.ContainsTransactionLogResponse, error) {
	return &blockdaopb.ContainsTransactionLogResponse{
		Yes: server.dao.ContainsTransactionLog(),
	}, nil
}

func (server *BlockDAOServer) TransactionLogs(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.TransactionLogsResponse, error) {
	logs, err := server.dao.TransactionLogs(request.Height)
	if err != nil {
		return nil, err
	}
	return &blockdaopb.TransactionLogsResponse{
		TransactionLogs: logs,
	}, nil
}

func (server *BlockDAOServer) Header(_ context.Context, request *blockdaopb.BlockHashRequest) (*blockdaopb.HeaderResponse, error) {
	h, err := hash.HexStringToHash256(request.Hash)
	if err != nil {
		return nil, err
	}
	header, err := server.dao.Header(h)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.HeaderResponse{
		Header: header.Proto(),
	}, nil

}

func (server *BlockDAOServer) HeaderByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.HeaderResponse, error) {
	header, err := server.dao.HeaderByHeight(request.Height)
	if err != nil {
		return nil, err
	}

	return &blockdaopb.HeaderResponse{
		Header: header.Proto(),
	}, nil
}

func (server *BlockDAOServer) FooterByHeight(_ context.Context, request *blockdaopb.BlockHeightRequest) (*blockdaopb.FooterResponse, error) {
	footer, err := server.dao.FooterByHeight(request.Height)
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
