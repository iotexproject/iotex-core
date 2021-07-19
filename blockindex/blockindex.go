// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type blockIndex struct {
	hash      []byte
	numAction uint32
	tsfAmount *big.Int
}

// Hash returns the hash
func (b *blockIndex) Hash() []byte {
	return b.hash
}

// NumAction returns number of actions
func (b *blockIndex) NumAction() uint32 {
	return b.numAction
}

// TsfAmount returns transfer amount
func (b *blockIndex) TsfAmount() *big.Int {
	return b.tsfAmount
}

// Serialize into byte stream
func (b *blockIndex) Serialize() []byte {
	return byteutil.Must(proto.Marshal(b.toProto()))
}

// Desrialize from byte stream
func (b *blockIndex) Deserialize(buf []byte) error {
	pb := &indexpb.BlockIndex{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return err
	}
	return b.fromProto(pb)
}

// toProto converts to protobuf
func (b *blockIndex) toProto() *indexpb.BlockIndex {
	index := &indexpb.BlockIndex{
		NumAction: b.numAction,
		Hash:      b.hash,
	}
	if b.tsfAmount != nil {
		index.TsfAmount = b.tsfAmount.Bytes()
	}
	return index
}

// fromProto converts from protobuf
func (b *blockIndex) fromProto(pbIndex *indexpb.BlockIndex) error {
	if pbIndex == nil {
		return errors.New("empty protobuf")
	}
	b.numAction = pbIndex.NumAction
	b.hash = nil
	if len(pbIndex.Hash) > 0 {
		b.hash = make([]byte, len(pbIndex.Hash))
		copy(b.hash, pbIndex.Hash)
	}
	b.tsfAmount = big.NewInt(0)
	if len(pbIndex.TsfAmount) > 0 {
		b.tsfAmount.SetBytes(pbIndex.TsfAmount)
	}
	return nil
}
