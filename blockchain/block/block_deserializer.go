// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
)

// Deserializer de-serializes a block
//
// It's a wrapper to set certain parameters in order to correctly de-serialize a block
// Currently the parameter is EVM network ID for tx in web3 format, it is called like
//
// blk, err := (&Deserializer{}).SetEvmNetworkID(id).FromBlockProto(pbBlock)
// blk, err := (&Deserializer{}).SetEvmNetworkID(id).DeserializeBlock(buf)
type Deserializer struct {
	evmNetworkID uint32
}

// NewDeserializer creates a new deserializer
func NewDeserializer(evmNetworkID uint32) *Deserializer {
	return &Deserializer{
		evmNetworkID: evmNetworkID,
	}
}

// EvmNetworkID get evm network ID
func (bd *Deserializer) EvmNetworkID() uint32 { return bd.evmNetworkID }

// SetEvmNetworkID sets the evm network ID for web3 actions
func (bd *Deserializer) SetEvmNetworkID(id uint32) *Deserializer {
	bd.evmNetworkID = id
	return bd
}

// FromBlockProto converts protobuf to block
func (bd *Deserializer) FromBlockProto(pbBlock *iotextypes.Block) (*Block, error) {
	var (
		b   = Block{}
		err error
	)
	if err = b.Header.LoadFromBlockHeaderProto(pbBlock.GetHeader()); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block header")
	}
	if b.Body, err = bd.fromBodyProto(pbBlock.GetBody()); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block body")
	}
	if err = b.ConvertFromBlockFooterPb(pbBlock.GetFooter()); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block footer")
	}
	return &b, nil
}

// DeserializeBlock de-serializes a block
func (bd *Deserializer) DeserializeBlock(buf []byte) (*Block, error) {
	pbBlock := iotextypes.Block{}
	if err := proto.Unmarshal(buf, &pbBlock); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block")
	}
	b, err := bd.FromBlockProto(&pbBlock)
	if err != nil {
		return nil, err
	}
	b.Receipts = nil
	if err = b.VerifyTxRoot(); err != nil {
		return nil, err
	}
	return b, nil
}

// fromBodyProto converts protobuf to body
func (bd *Deserializer) fromBodyProto(pbBody *iotextypes.BlockBody) (Body, error) {
	b := Body{}
	for _, actPb := range pbBody.Actions {
		act, err := (&action.Deserializer{}).SetEvmNetworkID(bd.evmNetworkID).ActionToSealedEnvelope(actPb)
		if err != nil {
			return b, errors.Wrap(err, "failed to deserialize block body")
		}
		b.Actions = append(b.Actions, act)
	}
	return b, nil
}

// DeserializeBody de-serializes a block body
func (bd *Deserializer) DeserializeBody(buf []byte) (*Body, error) {
	pb := iotextypes.BlockBody{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block body")
	}
	b, err := bd.fromBodyProto(&pb)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (bd *Deserializer) BlockFromBlockStoreProto(pb *iotextypes.BlockStore) (*Block, error) {
	return bd.blockFromBlockStoreProto(pb)
}

func (bd *Deserializer) blockFromBlockStoreProto(pb *iotextypes.BlockStore) (*Block, error) {
	blk, err := bd.FromBlockProto(pb.Block)
	if err != nil {
		return nil, err
	}
	// TODO: Reenable this if necessary
	// // verify merkle root can match after deserialize
	// if err := blk.VerifyTxRoot(); err != nil {
	// 	return nil, err
	// }
	return blk, nil
}

func (bd *Deserializer) ReceiptsFromBlockStoreProto(pb *iotextypes.BlockStore) ([]*action.Receipt, error) {
	return bd.receiptsFromBlockStoreProto(pb)
}

func (bd *Deserializer) receiptsFromBlockStoreProto(pb *iotextypes.BlockStore) ([]*action.Receipt, error) {
	receipts := make([]*action.Receipt, 0)
	for _, receiptPb := range pb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

// DeserializeBlockStore de-serializes a block store
func (bd *Deserializer) DeserializeBlockStore(buf []byte) (*Store, error) {
	pb := iotextypes.BlockStore{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block store")
	}
	blk, err := bd.blockFromBlockStoreProto(&pb)
	if err != nil {
		return nil, err
	}
	receipts, err := bd.receiptsFromBlockStoreProto(&pb)
	if err != nil {
		return nil, err
	}
	return &Store{
		Block:    blk,
		Receipts: receipts,
	}, nil
}
