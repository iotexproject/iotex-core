// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/endorsement"
	iproto "github.com/iotexproject/iotex-core/proto"
)

// BlockFooter defines a set of proof of this block
type BlockFooter struct {
	commitTimestamp uint64
	endorsements    *endorsement.Set // COMMIT endorsements from more than 2/3 delegates
	receipts        []*action.Receipt
}

// ToProto converts block footer to protobuf format
func (bf *BlockFooter) ToProto() *iproto.BlockFooterPb {
	receipts := []*iproto.ReceiptPb{}
	for _, r := range bf.receipts {
		receipts = append(receipts, r.ConvertToReceiptPb())
	}

	return &iproto.BlockFooterPb{
		CommitTimestamp: bf.commitTimestamp,
		LockProof:       bf.endorsements.ToProto(),
		Receipts:        receipts,
	}
}

// LoadProto loads block footer from protobuf
func (bf *BlockFooter) LoadProto(pb *iproto.BlockFooterPb) error {
	receipts := []*action.Receipt{}
	for _, rPb := range pb.Receipts {
		r := &action.Receipt{}
		r.ConvertFromReceiptPb(rPb)
		receipts = append(receipts, r)
	}
	endorsements := &endorsement.Set{}
	if err := endorsements.FromProto(pb.GetLockProof()); err != nil {
		return err
	}
	bf.commitTimestamp = pb.CommitTimestamp
	bf.endorsements = endorsements
	bf.receipts = receipts

	return nil
}

// Serialize returns a serialized byte stream
func (bf *BlockFooter) Serialize() ([]byte, error) {
	return proto.Marshal(bf.ToProto())
}

// Deserialize loads the block footer from byte array
func (bf *BlockFooter) Deserialize(b []byte) error {
	pb := &iproto.BlockFooterPb{}
	if err := proto.Unmarshal(b, pb); err != nil {
		return err
	}
	return bf.LoadProto(pb)
}
