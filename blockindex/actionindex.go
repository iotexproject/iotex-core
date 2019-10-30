// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type actionIndex struct {
	blkHeight uint64
	blkHash   []byte
}

// Height returns the block height of action
func (a *actionIndex) BlockHeight() uint64 {
	return a.blkHeight
}

// BlockHash returns the block hash of action
func (a *actionIndex) BlockHash() []byte {
	return a.blkHash
}

// Serialize into byte stream
func (a *actionIndex) Serialize() []byte {
	return byteutil.Must(proto.Marshal(a.toProto()))
}

// Desrialize from byte stream
func (a *actionIndex) Deserialize(buf []byte) error {
	pb := &indexpb.ActionIndex{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	return a.fromProto(pb)
}

// toProto converts to protobuf
func (a *actionIndex) toProto() *indexpb.ActionIndex {
	return &indexpb.ActionIndex{
		BlkHeight: a.blkHeight,
		BlkHash:   a.blkHash,
	}
}

// fromProto converts from protobuf
func (a *actionIndex) fromProto(pbIndex *indexpb.ActionIndex) error {
	if pbIndex == nil {
		return errors.New("empty protobuf")
	}
	a.blkHeight = pbIndex.BlkHeight
	a.blkHash = nil
	if len(pbIndex.BlkHash) > 0 {
		a.blkHash = make([]byte, len(pbIndex.BlkHash))
		copy(a.blkHash, pbIndex.BlkHash)
	}
	return nil
}
