// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// ActionIndex change private to public for mock Indexer
type ActionIndex struct {
	blkHeight uint64
}

// Height returns the block height of action
func (a *ActionIndex) BlockHeight() uint64 {
	return a.blkHeight
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
	}
}

// fromProto converts from protobuf
func (a *ActionIndex) fromProto(pbIndex *indexpb.ActionIndex) error {
	if pbIndex == nil {
		return errors.New("empty protobuf")
	}
	a.blkHeight = pbIndex.BlkHeight
	return nil
}
