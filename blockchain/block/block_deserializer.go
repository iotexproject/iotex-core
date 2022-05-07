// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// Deserializer de-serializes a block
type Deserializer struct {
	withChainID bool
}

// FromBlockProto converts protobuf to block
func (bd *Deserializer) FromBlockProto(pbBlock *iotextypes.Block) (*Block, error) {
	var (
		b   = Block{}
		err error
	)
	if err := b.Header.LoadFromBlockHeaderProto(pbBlock.GetHeader()); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block header")
	}
	if bd.withChainID {
		err = b.Body.LoadProtoWithChainID(pbBlock.GetBody())
	} else {
		err = b.Body.LoadProto(pbBlock.GetBody())
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block body")
	}
	if err := b.ConvertFromBlockFooterPb(pbBlock.GetFooter()); err != nil {
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

// FromBodyProto converts protobuf to body
func (bd *Deserializer) FromBodyProto(pbBody *iotextypes.BlockBody) (*Body, error) {
	var (
		b   = Body{}
		err error
	)
	if bd.withChainID {
		err = b.LoadProtoWithChainID(pbBody)
	} else {
		err = b.LoadProto(pbBody)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize block body")
	}
	return &b, nil
}

// DeserializeBody de-serializes a block body
func (bd *Deserializer) DeserializeBody(buf []byte) (*Body, error) {
	pb := iotextypes.BlockBody{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block body")
	}
	return bd.FromBodyProto(&pb)
}

// WithChainID sets whether or not to use chainID
func (bd *Deserializer) WithChainID(with bool) *Deserializer {
	bd.withChainID = with
	return bd
}
