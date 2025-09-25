// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package stakingindex

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockindex/nativestaking/stakingindexpb"
)

type bucketList struct {
	maxBucket uint64
	deleted   []uint64
}

func (bl *bucketList) serialize() ([]byte, error) {
	return proto.Marshal(bl.toProto())
}

func (bl *bucketList) toProto() *stakingindexpb.BucketList {
	return &stakingindexpb.BucketList{
		MaxBucket: bl.maxBucket,
		Deleted:   bl.deleted,
	}
}

func fromProtoBucketList(pb *stakingindexpb.BucketList) *bucketList {
	return &bucketList{
		maxBucket: pb.MaxBucket,
		deleted:   pb.Deleted,
	}
}

func deserializeBucketList(buf []byte) (*bucketList, error) {
	pb := stakingindexpb.BucketList{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return nil, err
	}
	return fromProtoBucketList(&pb), nil
}

type candList struct {
	id [][]byte
}

func (cl *candList) serialize() ([]byte, error) {
	return proto.Marshal(cl.toProto())
}

func (cl *candList) toProto() *stakingindexpb.CandList {
	return &stakingindexpb.CandList{
		Id: cl.id,
	}
}

func fromProtoCandList(pb *stakingindexpb.CandList) *candList {
	return &candList{
		id: pb.Id,
	}
}

func deserializeCandList(buf []byte) (*candList, error) {
	pb := stakingindexpb.CandList{}
	if err := proto.Unmarshal(buf, &pb); err != nil {
		return nil, err
	}
	return fromProtoCandList(&pb), nil
}
