// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// ActionIndex change private to public for mock Indexer
type ActionIndex struct {
	blkHeight uint64
	// txNumber is the %-th of action in a block
	// txNumber starts from 1
	// 0 means no txNumber for backward compatibility
	txNumber uint32
}

// Height returns the block height of action
func (a *ActionIndex) BlockHeight() uint64 {
	return a.blkHeight
}

// TxNumber returns the transaction number of action
func (a *ActionIndex) TxNumber() uint32 {
	return a.txNumber
}

// Serialize into byte stream
func (a *ActionIndex) Serialize() []byte {
	return byteutil.Must(proto.Marshal(a.toProto()))
}

// Deserialize from byte stream
func (a *ActionIndex) Deserialize(buf []byte) error {
	pb := &indexpb.ActionIndex{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	return a.fromProto(pb)
}

// toProto converts to protobuf
func (a *ActionIndex) toProto() *indexpb.ActionIndex {
	return &indexpb.ActionIndex{
		BlkHeight: a.blkHeight,
		TxNumber:  a.txNumber,
	}
}

// fromProto converts from protobuf
func (a *ActionIndex) fromProto(pbIndex *indexpb.ActionIndex) error {
	if pbIndex == nil {
		return errors.New("empty protobuf")
	}
	a.blkHeight = pbIndex.BlkHeight
	a.txNumber = pbIndex.TxNumber
	return nil
}
