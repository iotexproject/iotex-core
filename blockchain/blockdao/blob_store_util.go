// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao/blobindexpb"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type blobIndex struct {
	hashes [][]byte
}

func (bs *blobIndex) serialize() []byte {
	return byteutil.Must(proto.Marshal(bs.toProto()))
}

func (bs *blobIndex) toProto() *blobindexpb.BlobIndex {
	return &blobindexpb.BlobIndex{
		Hashes: bs.hashes,
	}
}

func fromProtoBlobIndex(pb *blobindexpb.BlobIndex) *blobIndex {
	return &blobIndex{
		hashes: pb.Hashes,
	}
}

func deserializeBlobIndex(buf []byte) (*blobIndex, error) {
	var bs blobindexpb.BlobIndex
	if err := proto.Unmarshal(buf, &bs); err != nil {
		return nil, err
	}
	return fromProtoBlobIndex(&bs), nil
}

func keyForBlock(b uint64) []byte {
	return byteutil.Uint64ToBytesBigEndian(b)
}
