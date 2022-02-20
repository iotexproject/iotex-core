// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
)

type (
	// Store defines block storage schema
	Store struct {
		Block    *Block
		Receipts []*action.Receipt
	}
)

// Serialize returns the serialized byte stream of Store
func (in *Store) Serialize() ([]byte, error) {
	return proto.Marshal(in.ToProto())
}

// ToProto converts to proto message
func (in *Store) ToProto() *iotextypes.BlockStore {
	receipts := []*iotextypes.Receipt{}
	for _, r := range in.Receipts {
		receipts = append(receipts, r.ConvertToReceiptPb())
	}
	return &iotextypes.BlockStore{
		Block:    in.Block.ConvertToBlockPb(),
		Receipts: receipts,
	}
}

// FromProto converts from proto message
func (in *Store) FromProto(pb *iotextypes.BlockStore) error {
	in.Block = &Block{}
	if err := in.Block.ConvertFromBlockPb(pb.Block); err != nil {
		return err
	}
	// verify merkle root can match after deserialize
	if err := in.Block.VerifyTxRoot(); err != nil {
		return err
	}

	in.Receipts = []*action.Receipt{}
	for _, receiptPb := range pb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		in.Receipts = append(in.Receipts, receipt)
	}
	return nil
}

// Deserialize parses the byte stream into Store
func (in *Store) Deserialize(buf []byte) error {
	pbStore := &iotextypes.BlockStore{}
	if err := proto.Unmarshal(buf, pbStore); err != nil {
		return err
	}
	return in.FromProto(pbStore)
}

// DeserializeBlockStoresPb decode byte stream into BlockStores pb message
func DeserializeBlockStoresPb(buf []byte) (*iotextypes.BlockStores, error) {
	pbStores := &iotextypes.BlockStores{}
	if err := proto.Unmarshal(buf, pbStores); err != nil {
		return nil, err
	}
	return pbStores, nil
}
