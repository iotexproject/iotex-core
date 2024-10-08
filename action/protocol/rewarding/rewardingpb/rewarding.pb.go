// Copyright (c) 2019 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// To compile the proto, run:
//      protoc --go_out=plugins=grpc:. *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v4.23.3
// source: action/protocol/rewarding/rewardingpb/rewarding.proto

package rewardingpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RewardLog_RewardType int32

const (
	RewardLog_BLOCK_REWARD     RewardLog_RewardType = 0
	RewardLog_EPOCH_REWARD     RewardLog_RewardType = 1
	RewardLog_FOUNDATION_BONUS RewardLog_RewardType = 2
	RewardLog_PRIORITY_BONUS   RewardLog_RewardType = 3
)

// Enum value maps for RewardLog_RewardType.
var (
	RewardLog_RewardType_name = map[int32]string{
		0: "BLOCK_REWARD",
		1: "EPOCH_REWARD",
		2: "FOUNDATION_BONUS",
		3: "PRIORITY_BONUS",
	}
	RewardLog_RewardType_value = map[string]int32{
		"BLOCK_REWARD":     0,
		"EPOCH_REWARD":     1,
		"FOUNDATION_BONUS": 2,
		"PRIORITY_BONUS":   3,
	}
)

func (x RewardLog_RewardType) Enum() *RewardLog_RewardType {
	p := new(RewardLog_RewardType)
	*p = x
	return p
}

func (x RewardLog_RewardType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (RewardLog_RewardType) Descriptor() protoreflect.EnumDescriptor {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_enumTypes[0].Descriptor()
}

func (RewardLog_RewardType) Type() protoreflect.EnumType {
	return &file_action_protocol_rewarding_rewardingpb_rewarding_proto_enumTypes[0]
}

func (x RewardLog_RewardType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use RewardLog_RewardType.Descriptor instead.
func (RewardLog_RewardType) EnumDescriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{5, 0}
}

type Admin struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlockReward                    string `protobuf:"bytes,1,opt,name=blockReward,proto3" json:"blockReward,omitempty"`
	EpochReward                    string `protobuf:"bytes,2,opt,name=epochReward,proto3" json:"epochReward,omitempty"`
	NumDelegatesForEpochReward     uint64 `protobuf:"varint,3,opt,name=numDelegatesForEpochReward,proto3" json:"numDelegatesForEpochReward,omitempty"`
	FoundationBonus                string `protobuf:"bytes,4,opt,name=foundationBonus,proto3" json:"foundationBonus,omitempty"`
	NumDelegatesForFoundationBonus uint64 `protobuf:"varint,5,opt,name=numDelegatesForFoundationBonus,proto3" json:"numDelegatesForFoundationBonus,omitempty"`
	FoundationBonusLastEpoch       uint64 `protobuf:"varint,6,opt,name=foundationBonusLastEpoch,proto3" json:"foundationBonusLastEpoch,omitempty"`
	ProductivityThreshold          uint64 `protobuf:"varint,7,opt,name=productivityThreshold,proto3" json:"productivityThreshold,omitempty"`
}

func (x *Admin) Reset() {
	*x = Admin{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Admin) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Admin) ProtoMessage() {}

func (x *Admin) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Admin.ProtoReflect.Descriptor instead.
func (*Admin) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{0}
}

func (x *Admin) GetBlockReward() string {
	if x != nil {
		return x.BlockReward
	}
	return ""
}

func (x *Admin) GetEpochReward() string {
	if x != nil {
		return x.EpochReward
	}
	return ""
}

func (x *Admin) GetNumDelegatesForEpochReward() uint64 {
	if x != nil {
		return x.NumDelegatesForEpochReward
	}
	return 0
}

func (x *Admin) GetFoundationBonus() string {
	if x != nil {
		return x.FoundationBonus
	}
	return ""
}

func (x *Admin) GetNumDelegatesForFoundationBonus() uint64 {
	if x != nil {
		return x.NumDelegatesForFoundationBonus
	}
	return 0
}

func (x *Admin) GetFoundationBonusLastEpoch() uint64 {
	if x != nil {
		return x.FoundationBonusLastEpoch
	}
	return 0
}

func (x *Admin) GetProductivityThreshold() uint64 {
	if x != nil {
		return x.ProductivityThreshold
	}
	return 0
}

type Fund struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TotalBalance     string `protobuf:"bytes,1,opt,name=totalBalance,proto3" json:"totalBalance,omitempty"`
	UnclaimedBalance string `protobuf:"bytes,2,opt,name=unclaimedBalance,proto3" json:"unclaimedBalance,omitempty"`
}

func (x *Fund) Reset() {
	*x = Fund{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Fund) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fund) ProtoMessage() {}

func (x *Fund) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Fund.ProtoReflect.Descriptor instead.
func (*Fund) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{1}
}

func (x *Fund) GetTotalBalance() string {
	if x != nil {
		return x.TotalBalance
	}
	return ""
}

func (x *Fund) GetUnclaimedBalance() string {
	if x != nil {
		return x.UnclaimedBalance
	}
	return ""
}

type RewardHistory struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RewardHistory) Reset() {
	*x = RewardHistory{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RewardHistory) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RewardHistory) ProtoMessage() {}

func (x *RewardHistory) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RewardHistory.ProtoReflect.Descriptor instead.
func (*RewardHistory) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{2}
}

type Account struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Balance string `protobuf:"bytes,1,opt,name=balance,proto3" json:"balance,omitempty"`
}

func (x *Account) Reset() {
	*x = Account{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Account) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Account) ProtoMessage() {}

func (x *Account) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Account.ProtoReflect.Descriptor instead.
func (*Account) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{3}
}

func (x *Account) GetBalance() string {
	if x != nil {
		return x.Balance
	}
	return ""
}

type Exempt struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addrs [][]byte `protobuf:"bytes,1,rep,name=addrs,proto3" json:"addrs,omitempty"`
}

func (x *Exempt) Reset() {
	*x = Exempt{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Exempt) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Exempt) ProtoMessage() {}

func (x *Exempt) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Exempt.ProtoReflect.Descriptor instead.
func (*Exempt) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{4}
}

func (x *Exempt) GetAddrs() [][]byte {
	if x != nil {
		return x.Addrs
	}
	return nil
}

type RewardLog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type   RewardLog_RewardType `protobuf:"varint,1,opt,name=type,proto3,enum=rewardingpb.RewardLog_RewardType" json:"type,omitempty"`
	Addr   string               `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Amount string               `protobuf:"bytes,3,opt,name=amount,proto3" json:"amount,omitempty"`
}

func (x *RewardLog) Reset() {
	*x = RewardLog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RewardLog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RewardLog) ProtoMessage() {}

func (x *RewardLog) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RewardLog.ProtoReflect.Descriptor instead.
func (*RewardLog) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{5}
}

func (x *RewardLog) GetType() RewardLog_RewardType {
	if x != nil {
		return x.Type
	}
	return RewardLog_BLOCK_REWARD
}

func (x *RewardLog) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *RewardLog) GetAmount() string {
	if x != nil {
		return x.Amount
	}
	return ""
}

type RewardLogs struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Logs []*RewardLog `protobuf:"bytes,1,rep,name=logs,proto3" json:"logs,omitempty"`
}

func (x *RewardLogs) Reset() {
	*x = RewardLogs{}
	if protoimpl.UnsafeEnabled {
		mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RewardLogs) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RewardLogs) ProtoMessage() {}

func (x *RewardLogs) ProtoReflect() protoreflect.Message {
	mi := &file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RewardLogs.ProtoReflect.Descriptor instead.
func (*RewardLogs) Descriptor() ([]byte, []int) {
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP(), []int{6}
}

func (x *RewardLogs) GetLogs() []*RewardLog {
	if x != nil {
		return x.Logs
	}
	return nil
}

var File_action_protocol_rewarding_rewardingpb_rewarding_proto protoreflect.FileDescriptor

var file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDesc = []byte{
	0x0a, 0x35, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6c, 0x2f, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69, 0x6e, 0x67, 0x2f, 0x72, 0x65, 0x77, 0x61,
	0x72, 0x64, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2f, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69, 0x6e,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0b, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69,
	0x6e, 0x67, 0x70, 0x62, 0x22, 0xef, 0x02, 0x0a, 0x05, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x12, 0x20,
	0x0a, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64,
	0x12, 0x20, 0x0a, 0x0b, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x52, 0x65, 0x77, 0x61,
	0x72, 0x64, 0x12, 0x3e, 0x0a, 0x1a, 0x6e, 0x75, 0x6d, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74,
	0x65, 0x73, 0x46, 0x6f, 0x72, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x1a, 0x6e, 0x75, 0x6d, 0x44, 0x65, 0x6c, 0x65, 0x67,
	0x61, 0x74, 0x65, 0x73, 0x46, 0x6f, 0x72, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x52, 0x65, 0x77, 0x61,
	0x72, 0x64, 0x12, 0x28, 0x0a, 0x0f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x42, 0x6f, 0x6e, 0x75, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x6f, 0x6e, 0x75, 0x73, 0x12, 0x46, 0x0a, 0x1e,
	0x6e, 0x75, 0x6d, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x65, 0x73, 0x46, 0x6f, 0x72, 0x46,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x6f, 0x6e, 0x75, 0x73, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x1e, 0x6e, 0x75, 0x6d, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74,
	0x65, 0x73, 0x46, 0x6f, 0x72, 0x46, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42,
	0x6f, 0x6e, 0x75, 0x73, 0x12, 0x3a, 0x0a, 0x18, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x42, 0x6f, 0x6e, 0x75, 0x73, 0x4c, 0x61, 0x73, 0x74, 0x45, 0x70, 0x6f, 0x63, 0x68,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x04, 0x52, 0x18, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x42, 0x6f, 0x6e, 0x75, 0x73, 0x4c, 0x61, 0x73, 0x74, 0x45, 0x70, 0x6f, 0x63, 0x68,
	0x12, 0x34, 0x0a, 0x15, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x69, 0x76, 0x69, 0x74, 0x79,
	0x54, 0x68, 0x72, 0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64, 0x18, 0x07, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x15, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x74, 0x69, 0x76, 0x69, 0x74, 0x79, 0x54, 0x68, 0x72,
	0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64, 0x22, 0x56, 0x0a, 0x04, 0x46, 0x75, 0x6e, 0x64, 0x12, 0x22,
	0x0a, 0x0c, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x42, 0x61, 0x6c, 0x61, 0x6e,
	0x63, 0x65, 0x12, 0x2a, 0x0a, 0x10, 0x75, 0x6e, 0x63, 0x6c, 0x61, 0x69, 0x6d, 0x65, 0x64, 0x42,
	0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x75, 0x6e,
	0x63, 0x6c, 0x61, 0x69, 0x6d, 0x65, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x22, 0x0f,
	0x0a, 0x0d, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x48, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x79, 0x22,
	0x23, 0x0a, 0x07, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x62, 0x61,
	0x6c, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x62, 0x61, 0x6c,
	0x61, 0x6e, 0x63, 0x65, 0x22, 0x1e, 0x0a, 0x06, 0x45, 0x78, 0x65, 0x6d, 0x70, 0x74, 0x12, 0x14,
	0x0a, 0x05, 0x61, 0x64, 0x64, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x05, 0x61,
	0x64, 0x64, 0x72, 0x73, 0x22, 0xca, 0x01, 0x0a, 0x09, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x4c,
	0x6f, 0x67, 0x12, 0x35, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x21, 0x2e, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x52,
	0x65, 0x77, 0x61, 0x72, 0x64, 0x4c, 0x6f, 0x67, 0x2e, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x16, 0x0a,
	0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61,
	0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x22, 0x5a, 0x0a, 0x0a, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x0c, 0x42, 0x4c, 0x4f, 0x43, 0x4b, 0x5f, 0x52, 0x45, 0x57,
	0x41, 0x52, 0x44, 0x10, 0x00, 0x12, 0x10, 0x0a, 0x0c, 0x45, 0x50, 0x4f, 0x43, 0x48, 0x5f, 0x52,
	0x45, 0x57, 0x41, 0x52, 0x44, 0x10, 0x01, 0x12, 0x14, 0x0a, 0x10, 0x46, 0x4f, 0x55, 0x4e, 0x44,
	0x41, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x42, 0x4f, 0x4e, 0x55, 0x53, 0x10, 0x02, 0x12, 0x12, 0x0a,
	0x0e, 0x50, 0x52, 0x49, 0x4f, 0x52, 0x49, 0x54, 0x59, 0x5f, 0x42, 0x4f, 0x4e, 0x55, 0x53, 0x10,
	0x03, 0x22, 0x38, 0x0a, 0x0a, 0x52, 0x65, 0x77, 0x61, 0x72, 0x64, 0x4c, 0x6f, 0x67, 0x73, 0x12,
	0x2a, 0x0a, 0x04, 0x6c, 0x6f, 0x67, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x16, 0x2e,
	0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x52, 0x65, 0x77, 0x61,
	0x72, 0x64, 0x4c, 0x6f, 0x67, 0x52, 0x04, 0x6c, 0x6f, 0x67, 0x73, 0x42, 0x4a, 0x5a, 0x48, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6f, 0x74, 0x65, 0x78, 0x70,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x2f, 0x69, 0x6f, 0x74, 0x65, 0x78, 0x2d, 0x63, 0x6f, 0x72,
	0x65, 0x2f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6c, 0x2f, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x69, 0x6e, 0x67, 0x2f, 0x72, 0x65, 0x77, 0x61,
	0x72, 0x64, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescOnce sync.Once
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescData = file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDesc
)

func file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescGZIP() []byte {
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescOnce.Do(func() {
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescData = protoimpl.X.CompressGZIP(file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescData)
	})
	return file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDescData
}

var file_action_protocol_rewarding_rewardingpb_rewarding_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_action_protocol_rewarding_rewardingpb_rewarding_proto_goTypes = []interface{}{
	(RewardLog_RewardType)(0), // 0: rewardingpb.RewardLog.RewardType
	(*Admin)(nil),             // 1: rewardingpb.Admin
	(*Fund)(nil),              // 2: rewardingpb.Fund
	(*RewardHistory)(nil),     // 3: rewardingpb.RewardHistory
	(*Account)(nil),           // 4: rewardingpb.Account
	(*Exempt)(nil),            // 5: rewardingpb.Exempt
	(*RewardLog)(nil),         // 6: rewardingpb.RewardLog
	(*RewardLogs)(nil),        // 7: rewardingpb.RewardLogs
}
var file_action_protocol_rewarding_rewardingpb_rewarding_proto_depIdxs = []int32{
	0, // 0: rewardingpb.RewardLog.type:type_name -> rewardingpb.RewardLog.RewardType
	6, // 1: rewardingpb.RewardLogs.logs:type_name -> rewardingpb.RewardLog
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_action_protocol_rewarding_rewardingpb_rewarding_proto_init() }
func file_action_protocol_rewarding_rewardingpb_rewarding_proto_init() {
	if File_action_protocol_rewarding_rewardingpb_rewarding_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Admin); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Fund); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RewardHistory); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Account); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Exempt); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RewardLog); i {
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
		file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RewardLogs); i {
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
			RawDescriptor: file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_action_protocol_rewarding_rewardingpb_rewarding_proto_goTypes,
		DependencyIndexes: file_action_protocol_rewarding_rewardingpb_rewarding_proto_depIdxs,
		EnumInfos:         file_action_protocol_rewarding_rewardingpb_rewarding_proto_enumTypes,
		MessageInfos:      file_action_protocol_rewarding_rewardingpb_rewarding_proto_msgTypes,
	}.Build()
	File_action_protocol_rewarding_rewardingpb_rewarding_proto = out.File
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_rawDesc = nil
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_goTypes = nil
	file_action_protocol_rewarding_rewardingpb_rewarding_proto_depIdxs = nil
}
