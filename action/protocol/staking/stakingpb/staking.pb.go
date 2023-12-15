// Copyright (c) 2020 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// To compile the proto, run:
//      protoc --go_out=plugins=grpc:. *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v4.23.3
// source: action/protocol/staking/stakingpb/staking.proto

package stakingpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Bucket struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Index                     uint64                 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	CandidateAddress          string                 `protobuf:"bytes,2,opt,name=candidateAddress,proto3" json:"candidateAddress,omitempty"`
	StakedAmount              string                 `protobuf:"bytes,3,opt,name=stakedAmount,proto3" json:"stakedAmount,omitempty"`
	StakedDuration            uint32                 `protobuf:"varint,4,opt,name=stakedDuration,proto3" json:"stakedDuration,omitempty"`
	CreateTime                *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=createTime,proto3" json:"createTime,omitempty"`
	StakeStartTime            *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=stakeStartTime,proto3" json:"stakeStartTime,omitempty"`
	UnstakeStartTime          *timestamppb.Timestamp `protobuf:"bytes,7,opt,name=unstakeStartTime,proto3" json:"unstakeStartTime,omitempty"`
	AutoStake                 bool                   `protobuf:"varint,8,opt,name=autoStake,proto3" json:"autoStake,omitempty"`
	Owner                     string                 `protobuf:"bytes,9,opt,name=owner,proto3" json:"owner,omitempty"`
	ContractAddress           string                 `protobuf:"bytes,10,opt,name=contractAddress,proto3" json:"contractAddress,omitempty"`
	StakedDurationBlockNumber uint64                 `protobuf:"varint,11,opt,name=stakedDurationBlockNumber,proto3" json:"stakedDurationBlockNumber,omitempty"`
	CreateBlockHeight         uint64                 `protobuf:"varint,12,opt,name=createBlockHeight,proto3" json:"createBlockHeight,omitempty"`
	StakeStartBlockHeight     uint64                 `protobuf:"varint,13,opt,name=stakeStartBlockHeight,proto3" json:"stakeStartBlockHeight,omitempty"`
	UnstakeStartBlockHeight   uint64                 `protobuf:"varint,14,opt,name=unstakeStartBlockHeight,proto3" json:"unstakeStartBlockHeight,omitempty"`
}

func (x *Bucket) Reset() {
	*x = Bucket{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Bucket) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Bucket) ProtoMessage() {}

func (x *Bucket) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Bucket.ProtoReflect.Descriptor instead.
func (*Bucket) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{0}
}

func (x *Bucket) GetIndex() uint64 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *Bucket) GetCandidateAddress() string {
	if x != nil {
		return x.CandidateAddress
	}
	return ""
}

func (x *Bucket) GetStakedAmount() string {
	if x != nil {
		return x.StakedAmount
	}
	return ""
}

func (x *Bucket) GetStakedDuration() uint32 {
	if x != nil {
		return x.StakedDuration
	}
	return 0
}

func (x *Bucket) GetCreateTime() *timestamppb.Timestamp {
	if x != nil {
		return x.CreateTime
	}
	return nil
}

func (x *Bucket) GetStakeStartTime() *timestamppb.Timestamp {
	if x != nil {
		return x.StakeStartTime
	}
	return nil
}

func (x *Bucket) GetUnstakeStartTime() *timestamppb.Timestamp {
	if x != nil {
		return x.UnstakeStartTime
	}
	return nil
}

func (x *Bucket) GetAutoStake() bool {
	if x != nil {
		return x.AutoStake
	}
	return false
}

func (x *Bucket) GetOwner() string {
	if x != nil {
		return x.Owner
	}
	return ""
}

func (x *Bucket) GetContractAddress() string {
	if x != nil {
		return x.ContractAddress
	}
	return ""
}

func (x *Bucket) GetStakedDurationBlockNumber() uint64 {
	if x != nil {
		return x.StakedDurationBlockNumber
	}
	return 0
}

func (x *Bucket) GetCreateBlockHeight() uint64 {
	if x != nil {
		return x.CreateBlockHeight
	}
	return 0
}

func (x *Bucket) GetStakeStartBlockHeight() uint64 {
	if x != nil {
		return x.StakeStartBlockHeight
	}
	return 0
}

func (x *Bucket) GetUnstakeStartBlockHeight() uint64 {
	if x != nil {
		return x.UnstakeStartBlockHeight
	}
	return 0
}

type BucketIndices struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Indices []uint64 `protobuf:"varint,1,rep,packed,name=indices,proto3" json:"indices,omitempty"`
}

func (x *BucketIndices) Reset() {
	*x = BucketIndices{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BucketIndices) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BucketIndices) ProtoMessage() {}

func (x *BucketIndices) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BucketIndices.ProtoReflect.Descriptor instead.
func (*BucketIndices) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{1}
}

func (x *BucketIndices) GetIndices() []uint64 {
	if x != nil {
		return x.Indices
	}
	return nil
}

type Candidate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OwnerAddress       string `protobuf:"bytes,1,opt,name=ownerAddress,proto3" json:"ownerAddress,omitempty"`
	OperatorAddress    string `protobuf:"bytes,2,opt,name=operatorAddress,proto3" json:"operatorAddress,omitempty"`
	RewardAddress      string `protobuf:"bytes,3,opt,name=rewardAddress,proto3" json:"rewardAddress,omitempty"`
	Name               string `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	Votes              string `protobuf:"bytes,5,opt,name=votes,proto3" json:"votes,omitempty"`
	SelfStakeBucketIdx uint64 `protobuf:"varint,6,opt,name=selfStakeBucketIdx,proto3" json:"selfStakeBucketIdx,omitempty"`
	SelfStake          string `protobuf:"bytes,7,opt,name=selfStake,proto3" json:"selfStake,omitempty"`
}

func (x *Candidate) Reset() {
	*x = Candidate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Candidate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Candidate) ProtoMessage() {}

func (x *Candidate) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Candidate.ProtoReflect.Descriptor instead.
func (*Candidate) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{2}
}

func (x *Candidate) GetOwnerAddress() string {
	if x != nil {
		return x.OwnerAddress
	}
	return ""
}

func (x *Candidate) GetOperatorAddress() string {
	if x != nil {
		return x.OperatorAddress
	}
	return ""
}

func (x *Candidate) GetRewardAddress() string {
	if x != nil {
		return x.RewardAddress
	}
	return ""
}

func (x *Candidate) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Candidate) GetVotes() string {
	if x != nil {
		return x.Votes
	}
	return ""
}

func (x *Candidate) GetSelfStakeBucketIdx() uint64 {
	if x != nil {
		return x.SelfStakeBucketIdx
	}
	return 0
}

func (x *Candidate) GetSelfStake() string {
	if x != nil {
		return x.SelfStake
	}
	return ""
}

type Candidates struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Candidates []*Candidate `protobuf:"bytes,1,rep,name=candidates,proto3" json:"candidates,omitempty"`
}

func (x *Candidates) Reset() {
	*x = Candidates{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Candidates) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Candidates) ProtoMessage() {}

func (x *Candidates) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Candidates.ProtoReflect.Descriptor instead.
func (*Candidates) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{3}
}

func (x *Candidates) GetCandidates() []*Candidate {
	if x != nil {
		return x.Candidates
	}
	return nil
}

type TotalAmount struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Amount string `protobuf:"bytes,1,opt,name=amount,proto3" json:"amount,omitempty"`
	Count  uint64 `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
}

func (x *TotalAmount) Reset() {
	*x = TotalAmount{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TotalAmount) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TotalAmount) ProtoMessage() {}

func (x *TotalAmount) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TotalAmount.ProtoReflect.Descriptor instead.
func (*TotalAmount) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{4}
}

func (x *TotalAmount) GetAmount() string {
	if x != nil {
		return x.Amount
	}
	return ""
}

func (x *TotalAmount) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

type BucketType struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Amount      string `protobuf:"bytes,1,opt,name=amount,proto3" json:"amount,omitempty"`
	Duration    uint64 `protobuf:"varint,2,opt,name=duration,proto3" json:"duration,omitempty"`
	ActivatedAt uint64 `protobuf:"varint,3,opt,name=activatedAt,proto3" json:"activatedAt,omitempty"`
}

func (x *BucketType) Reset() {
	*x = BucketType{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BucketType) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BucketType) ProtoMessage() {}

func (x *BucketType) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BucketType.ProtoReflect.Descriptor instead.
func (*BucketType) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{5}
}

func (x *BucketType) GetAmount() string {
	if x != nil {
		return x.Amount
	}
	return ""
}

func (x *BucketType) GetDuration() uint64 {
	if x != nil {
		return x.Duration
	}
	return 0
}

func (x *BucketType) GetActivatedAt() uint64 {
	if x != nil {
		return x.ActivatedAt
	}
	return 0
}

type Endorsement struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExpireHeight uint64 `protobuf:"varint,1,opt,name=expireHeight,proto3" json:"expireHeight,omitempty"`
}

func (x *Endorsement) Reset() {
	*x = Endorsement{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Endorsement) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Endorsement) ProtoMessage() {}

func (x *Endorsement) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_staking_stakingpb_staking_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Endorsement.ProtoReflect.Descriptor instead.
func (*Endorsement) Descriptor() ([]byte, []int) {
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP(), []int{6}
}

func (x *Endorsement) GetExpireHeight() uint64 {
	if x != nil {
		return x.ExpireHeight
	}
	return 0
}

var File_action_protocol_staking_stakingpb_staking_proto protoreflect.FileDescriptor

var file_action_protocol_staking_stakingpb_staking_proto_rawDesc = []byte{
	0x0a, 0x2f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6c, 0x2f, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x2f, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e,
	0x67, 0x70, 0x62, 0x2f, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x09, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x98, 0x05,
	0x0a, 0x06, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65,
	0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x2a,
	0x0a, 0x10, 0x63, 0x61, 0x6e, 0x64, 0x69, 0x64, 0x61, 0x74, 0x65, 0x41, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x63, 0x61, 0x6e, 0x64, 0x69, 0x64,
	0x61, 0x74, 0x65, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x22, 0x0a, 0x0c, 0x73, 0x74,
	0x61, 0x6b, 0x65, 0x64, 0x41, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0c, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x41, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x26,
	0x0a, 0x0e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x44, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x3a, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x54, 0x69, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69,
	0x6d, 0x65, 0x12, 0x42, 0x0a, 0x0e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x54, 0x69, 0x6d, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61,
	0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x46, 0x0a, 0x10, 0x75, 0x6e, 0x73, 0x74, 0x61, 0x6b,
	0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x10, 0x75, 0x6e,
	0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x1c,
	0x0a, 0x09, 0x61, 0x75, 0x74, 0x6f, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x09, 0x61, 0x75, 0x74, 0x6f, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x12, 0x14, 0x0a, 0x05,
	0x6f, 0x77, 0x6e, 0x65, 0x72, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6f, 0x77, 0x6e,
	0x65, 0x72, 0x12, 0x28, 0x0a, 0x0f, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x41, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x63, 0x6f, 0x6e,
	0x74, 0x72, 0x61, 0x63, 0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x3c, 0x0a, 0x19,
	0x73, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x6c,
	0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x19, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x2c, 0x0a, 0x11, 0x63, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x04, 0x52, 0x11, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x34, 0x0a, 0x15, 0x73, 0x74, 0x61, 0x6b,
	0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68,
	0x74, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x04, 0x52, 0x15, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74,
	0x61, 0x72, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x38,
	0x0a, 0x17, 0x75, 0x6e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x42, 0x6c,
	0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x17, 0x75, 0x6e, 0x73, 0x74, 0x61, 0x6b, 0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x22, 0x29, 0x0a, 0x0d, 0x42, 0x75, 0x63, 0x6b,
	0x65, 0x74, 0x49, 0x6e, 0x64, 0x69, 0x63, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x69, 0x6e, 0x64,
	0x69, 0x63, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x04, 0x52, 0x07, 0x69, 0x6e, 0x64, 0x69,
	0x63, 0x65, 0x73, 0x22, 0xf7, 0x01, 0x0a, 0x09, 0x43, 0x61, 0x6e, 0x64, 0x69, 0x64, 0x61, 0x74,
	0x65, 0x12, 0x22, 0x0a, 0x0c, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x41, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f,
	0x72, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f,
	0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x24, 0x0a, 0x0d, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x41, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x6f, 0x74,
	0x65, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x6f, 0x74, 0x65, 0x73, 0x12,
	0x2e, 0x0a, 0x12, 0x73, 0x65, 0x6c, 0x66, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x42, 0x75, 0x63, 0x6b,
	0x65, 0x74, 0x49, 0x64, 0x78, 0x18, 0x06, 0x20, 0x01, 0x28, 0x04, 0x52, 0x12, 0x73, 0x65, 0x6c,
	0x66, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x49, 0x64, 0x78, 0x12,
	0x1c, 0x0a, 0x09, 0x73, 0x65, 0x6c, 0x66, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x73, 0x65, 0x6c, 0x66, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x22, 0x42, 0x0a,
	0x0a, 0x43, 0x61, 0x6e, 0x64, 0x69, 0x64, 0x61, 0x74, 0x65, 0x73, 0x12, 0x34, 0x0a, 0x0a, 0x63,
	0x61, 0x6e, 0x64, 0x69, 0x64, 0x61, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x14, 0x2e, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x43, 0x61, 0x6e, 0x64,
	0x69, 0x64, 0x61, 0x74, 0x65, 0x52, 0x0a, 0x63, 0x61, 0x6e, 0x64, 0x69, 0x64, 0x61, 0x74, 0x65,
	0x73, 0x22, 0x3b, 0x0a, 0x0b, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x41, 0x6d, 0x6f, 0x75, 0x6e, 0x74,
	0x12, 0x16, 0x0a, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x22, 0x62,
	0x0a, 0x0a, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x06,
	0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61, 0x6d,
	0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x20, 0x0a, 0x0b, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x61, 0x63, 0x74, 0x69, 0x76, 0x61, 0x74, 0x65, 0x64,
	0x41, 0x74, 0x22, 0x31, 0x0a, 0x0b, 0x45, 0x6e, 0x64, 0x6f, 0x72, 0x73, 0x65, 0x6d, 0x65, 0x6e,
	0x74, 0x12, 0x22, 0x0a, 0x0c, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x48, 0x65, 0x69, 0x67, 0x68,
	0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0c, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x48,
	0x65, 0x69, 0x67, 0x68, 0x74, 0x42, 0x46, 0x5a, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6f, 0x74, 0x65, 0x78, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74,
	0x2f, 0x69, 0x6f, 0x74, 0x65, 0x78, 0x2d, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x73, 0x74, 0x61, 0x6b,
	0x69, 0x6e, 0x67, 0x2f, 0x73, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_action_protocol_staking_stakingpb_staking_proto_rawDescOnce sync.Once
	file_action_protocol_staking_stakingpb_staking_proto_rawDescData = file_action_protocol_staking_stakingpb_staking_proto_rawDesc
)

func file_action_protocol_staking_stakingpb_staking_proto_rawDescGZIP() []byte {
	file_action_protocol_staking_stakingpb_staking_proto_rawDescOnce.Do(func() {
		file_action_protocol_staking_stakingpb_staking_proto_rawDescData = protoimpl.X.CompressGZIP(file_action_protocol_staking_stakingpb_staking_proto_rawDescData)
	})
	return file_action_protocol_staking_stakingpb_staking_proto_rawDescData
}

var file_action_protocol_staking_stakingpb_staking_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_action_protocol_staking_stakingpb_staking_proto_goTypes = []interface{}{
	(*Bucket)(nil),                // 0: stakingpb.Bucket
	(*BucketIndices)(nil),         // 1: stakingpb.BucketIndices
	(*Candidate)(nil),             // 2: stakingpb.Candidate
	(*Candidates)(nil),            // 3: stakingpb.Candidates
	(*TotalAmount)(nil),           // 4: stakingpb.TotalAmount
	(*BucketType)(nil),            // 5: stakingpb.BucketType
	(*Endorsement)(nil),           // 6: stakingpb.Endorsement
	(*timestamppb.Timestamp)(nil), // 7: google.protobuf.Timestamp
}
var file_action_protocol_staking_stakingpb_staking_proto_depIdxs = []int32{
	7, // 0: stakingpb.Bucket.createTime:type_name -> google.protobuf.Timestamp
	7, // 1: stakingpb.Bucket.stakeStartTime:type_name -> google.protobuf.Timestamp
	7, // 2: stakingpb.Bucket.unstakeStartTime:type_name -> google.protobuf.Timestamp
	2, // 3: stakingpb.Candidates.candidates:type_name -> stakingpb.Candidate
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_action_protocol_staking_stakingpb_staking_proto_init() }
func file_action_protocol_staking_stakingpb_staking_proto_init() {
	if File_action_protocol_staking_stakingpb_staking_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Bucket); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BucketIndices); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Candidate); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Candidates); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TotalAmount); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BucketType); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_action_protocol_staking_stakingpb_staking_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Endorsement); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_action_protocol_staking_stakingpb_staking_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_action_protocol_staking_stakingpb_staking_proto_goTypes,
		DependencyIndexes: file_action_protocol_staking_stakingpb_staking_proto_depIdxs,
		MessageInfos:      file_action_protocol_staking_stakingpb_staking_proto_msgTypes,
	}.Build()
	File_action_protocol_staking_stakingpb_staking_proto = out.File
	file_action_protocol_staking_stakingpb_staking_proto_rawDesc = nil
	file_action_protocol_staking_stakingpb_staking_proto_goTypes = nil
	file_action_protocol_staking_stakingpb_staking_proto_depIdxs = nil
}
