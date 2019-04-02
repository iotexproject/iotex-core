// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rewarding.proto

package rewardingpb

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type RewardLog_RewardType int32

const (
	RewardLog_BLOCK_REWARD     RewardLog_RewardType = 0
	RewardLog_EPOCH_REWARD     RewardLog_RewardType = 1
	RewardLog_FOUNDATION_BONUS RewardLog_RewardType = 2
)

var RewardLog_RewardType_name = map[int32]string{
	0: "BLOCK_REWARD",
	1: "EPOCH_REWARD",
	2: "FOUNDATION_BONUS",
}

var RewardLog_RewardType_value = map[string]int32{
	"BLOCK_REWARD":     0,
	"EPOCH_REWARD":     1,
	"FOUNDATION_BONUS": 2,
}

func (x RewardLog_RewardType) String() string {
	return proto.EnumName(RewardLog_RewardType_name, int32(x))
}

func (RewardLog_RewardType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{5, 0}
}

type Admin struct {
	BlockReward                    string   `protobuf:"bytes,1,opt,name=blockReward,proto3" json:"blockReward,omitempty"`
	EpochReward                    string   `protobuf:"bytes,2,opt,name=epochReward,proto3" json:"epochReward,omitempty"`
	NumDelegatesForEpochReward     uint64   `protobuf:"varint,3,opt,name=numDelegatesForEpochReward,proto3" json:"numDelegatesForEpochReward,omitempty"`
	FoundationBonus                string   `protobuf:"bytes,4,opt,name=foundationBonus,proto3" json:"foundationBonus,omitempty"`
	NumDelegatesForFoundationBonus uint64   `protobuf:"varint,5,opt,name=numDelegatesForFoundationBonus,proto3" json:"numDelegatesForFoundationBonus,omitempty"`
	FoundationBonusLastEpoch       uint64   `protobuf:"varint,6,opt,name=foundationBonusLastEpoch,proto3" json:"foundationBonusLastEpoch,omitempty"`
	ProductivityThreshold          uint64   `protobuf:"varint,7,opt,name=productivityThreshold,proto3" json:"productivityThreshold,omitempty"`
	XXX_NoUnkeyedLiteral           struct{} `json:"-"`
	XXX_unrecognized               []byte   `json:"-"`
	XXX_sizecache                  int32    `json:"-"`
}

func (m *Admin) Reset()         { *m = Admin{} }
func (m *Admin) String() string { return proto.CompactTextString(m) }
func (*Admin) ProtoMessage()    {}
func (*Admin) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{0}
}

func (m *Admin) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Admin.Unmarshal(m, b)
}
func (m *Admin) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Admin.Marshal(b, m, deterministic)
}
func (m *Admin) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Admin.Merge(m, src)
}
func (m *Admin) XXX_Size() int {
	return xxx_messageInfo_Admin.Size(m)
}
func (m *Admin) XXX_DiscardUnknown() {
	xxx_messageInfo_Admin.DiscardUnknown(m)
}

var xxx_messageInfo_Admin proto.InternalMessageInfo

func (m *Admin) GetBlockReward() string {
	if m != nil {
		return m.BlockReward
	}
	return ""
}

func (m *Admin) GetEpochReward() string {
	if m != nil {
		return m.EpochReward
	}
	return ""
}

func (m *Admin) GetNumDelegatesForEpochReward() uint64 {
	if m != nil {
		return m.NumDelegatesForEpochReward
	}
	return 0
}

func (m *Admin) GetFoundationBonus() string {
	if m != nil {
		return m.FoundationBonus
	}
	return ""
}

func (m *Admin) GetNumDelegatesForFoundationBonus() uint64 {
	if m != nil {
		return m.NumDelegatesForFoundationBonus
	}
	return 0
}

func (m *Admin) GetFoundationBonusLastEpoch() uint64 {
	if m != nil {
		return m.FoundationBonusLastEpoch
	}
	return 0
}

func (m *Admin) GetProductivityThreshold() uint64 {
	if m != nil {
		return m.ProductivityThreshold
	}
	return 0
}

type Fund struct {
	TotalBalance         string   `protobuf:"bytes,1,opt,name=totalBalance,proto3" json:"totalBalance,omitempty"`
	UnclaimedBalance     string   `protobuf:"bytes,2,opt,name=unclaimedBalance,proto3" json:"unclaimedBalance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Fund) Reset()         { *m = Fund{} }
func (m *Fund) String() string { return proto.CompactTextString(m) }
func (*Fund) ProtoMessage()    {}
func (*Fund) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{1}
}

func (m *Fund) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Fund.Unmarshal(m, b)
}
func (m *Fund) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Fund.Marshal(b, m, deterministic)
}
func (m *Fund) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Fund.Merge(m, src)
}
func (m *Fund) XXX_Size() int {
	return xxx_messageInfo_Fund.Size(m)
}
func (m *Fund) XXX_DiscardUnknown() {
	xxx_messageInfo_Fund.DiscardUnknown(m)
}

var xxx_messageInfo_Fund proto.InternalMessageInfo

func (m *Fund) GetTotalBalance() string {
	if m != nil {
		return m.TotalBalance
	}
	return ""
}

func (m *Fund) GetUnclaimedBalance() string {
	if m != nil {
		return m.UnclaimedBalance
	}
	return ""
}

type RewardHistory struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RewardHistory) Reset()         { *m = RewardHistory{} }
func (m *RewardHistory) String() string { return proto.CompactTextString(m) }
func (*RewardHistory) ProtoMessage()    {}
func (*RewardHistory) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{2}
}

func (m *RewardHistory) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RewardHistory.Unmarshal(m, b)
}
func (m *RewardHistory) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RewardHistory.Marshal(b, m, deterministic)
}
func (m *RewardHistory) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RewardHistory.Merge(m, src)
}
func (m *RewardHistory) XXX_Size() int {
	return xxx_messageInfo_RewardHistory.Size(m)
}
func (m *RewardHistory) XXX_DiscardUnknown() {
	xxx_messageInfo_RewardHistory.DiscardUnknown(m)
}

var xxx_messageInfo_RewardHistory proto.InternalMessageInfo

type Account struct {
	Balance              string   `protobuf:"bytes,1,opt,name=balance,proto3" json:"balance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Account) Reset()         { *m = Account{} }
func (m *Account) String() string { return proto.CompactTextString(m) }
func (*Account) ProtoMessage()    {}
func (*Account) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{3}
}

func (m *Account) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Account.Unmarshal(m, b)
}
func (m *Account) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Account.Marshal(b, m, deterministic)
}
func (m *Account) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Account.Merge(m, src)
}
func (m *Account) XXX_Size() int {
	return xxx_messageInfo_Account.Size(m)
}
func (m *Account) XXX_DiscardUnknown() {
	xxx_messageInfo_Account.DiscardUnknown(m)
}

var xxx_messageInfo_Account proto.InternalMessageInfo

func (m *Account) GetBalance() string {
	if m != nil {
		return m.Balance
	}
	return ""
}

type Exempt struct {
	Addrs                [][]byte `protobuf:"bytes,1,rep,name=addrs,proto3" json:"addrs,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Exempt) Reset()         { *m = Exempt{} }
func (m *Exempt) String() string { return proto.CompactTextString(m) }
func (*Exempt) ProtoMessage()    {}
func (*Exempt) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{4}
}

func (m *Exempt) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Exempt.Unmarshal(m, b)
}
func (m *Exempt) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Exempt.Marshal(b, m, deterministic)
}
func (m *Exempt) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Exempt.Merge(m, src)
}
func (m *Exempt) XXX_Size() int {
	return xxx_messageInfo_Exempt.Size(m)
}
func (m *Exempt) XXX_DiscardUnknown() {
	xxx_messageInfo_Exempt.DiscardUnknown(m)
}

var xxx_messageInfo_Exempt proto.InternalMessageInfo

func (m *Exempt) GetAddrs() [][]byte {
	if m != nil {
		return m.Addrs
	}
	return nil
}

type RewardLog struct {
	Type                 RewardLog_RewardType `protobuf:"varint,1,opt,name=type,proto3,enum=rewardingpb.RewardLog_RewardType" json:"type,omitempty"`
	Addr                 string               `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Amount               string               `protobuf:"bytes,3,opt,name=amount,proto3" json:"amount,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *RewardLog) Reset()         { *m = RewardLog{} }
func (m *RewardLog) String() string { return proto.CompactTextString(m) }
func (*RewardLog) ProtoMessage()    {}
func (*RewardLog) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5a8d72c965c1359, []int{5}
}

func (m *RewardLog) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RewardLog.Unmarshal(m, b)
}
func (m *RewardLog) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RewardLog.Marshal(b, m, deterministic)
}
func (m *RewardLog) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RewardLog.Merge(m, src)
}
func (m *RewardLog) XXX_Size() int {
	return xxx_messageInfo_RewardLog.Size(m)
}
func (m *RewardLog) XXX_DiscardUnknown() {
	xxx_messageInfo_RewardLog.DiscardUnknown(m)
}

var xxx_messageInfo_RewardLog proto.InternalMessageInfo

func (m *RewardLog) GetType() RewardLog_RewardType {
	if m != nil {
		return m.Type
	}
	return RewardLog_BLOCK_REWARD
}

func (m *RewardLog) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

func (m *RewardLog) GetAmount() string {
	if m != nil {
		return m.Amount
	}
	return ""
}

func init() {
	proto.RegisterEnum("rewardingpb.RewardLog_RewardType", RewardLog_RewardType_name, RewardLog_RewardType_value)
	proto.RegisterType((*Admin)(nil), "rewardingpb.Admin")
	proto.RegisterType((*Fund)(nil), "rewardingpb.Fund")
	proto.RegisterType((*RewardHistory)(nil), "rewardingpb.RewardHistory")
	proto.RegisterType((*Account)(nil), "rewardingpb.Account")
	proto.RegisterType((*Exempt)(nil), "rewardingpb.Exempt")
	proto.RegisterType((*RewardLog)(nil), "rewardingpb.RewardLog")
}

func init() { proto.RegisterFile("rewarding.proto", fileDescriptor_a5a8d72c965c1359) }

var fileDescriptor_a5a8d72c965c1359 = []byte{
	// 420 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x92, 0xcd, 0x6e, 0xd3, 0x40,
	0x10, 0xc7, 0x71, 0xea, 0x24, 0xca, 0x34, 0x10, 0x6b, 0x54, 0x90, 0xc5, 0xa1, 0x0a, 0xcb, 0x25,
	0xe2, 0x90, 0x03, 0x1f, 0x17, 0x0e, 0x48, 0x49, 0x13, 0xab, 0x88, 0x28, 0x46, 0x4b, 0x0a, 0xc7,
	0x6a, 0xe3, 0x5d, 0x12, 0x0b, 0x7b, 0xd7, 0x5a, 0xaf, 0x81, 0xbc, 0x18, 0xaf, 0xc5, 0x2b, 0x20,
	0xaf, 0x9d, 0xe2, 0x9a, 0x8f, 0xde, 0x66, 0xfe, 0xf3, 0x9b, 0xff, 0x8c, 0x46, 0x03, 0x23, 0x2d,
	0xbe, 0x31, 0xcd, 0x63, 0xb9, 0x9b, 0x66, 0x5a, 0x19, 0x85, 0xa7, 0x37, 0x42, 0xb6, 0x25, 0x3f,
	0x3b, 0xd0, 0x9d, 0xf1, 0x34, 0x96, 0x38, 0x86, 0xd3, 0x6d, 0xa2, 0xa2, 0x2f, 0xd4, 0x56, 0x7d,
	0x67, 0xec, 0x4c, 0x06, 0xb4, 0x29, 0x95, 0x84, 0xc8, 0x54, 0xb4, 0xaf, 0x89, 0x4e, 0x45, 0x34,
	0x24, 0x7c, 0x03, 0x8f, 0x65, 0x91, 0x2e, 0x44, 0x22, 0x76, 0xcc, 0x88, 0x3c, 0x50, 0x7a, 0xd9,
	0x68, 0x38, 0x19, 0x3b, 0x13, 0x97, 0xfe, 0x87, 0xc0, 0x09, 0x8c, 0x3e, 0xab, 0x42, 0x72, 0x66,
	0x62, 0x25, 0xe7, 0x4a, 0x16, 0xb9, 0xef, 0xda, 0x29, 0x6d, 0x19, 0x03, 0x38, 0x6f, 0xf9, 0x04,
	0xad, 0xc6, 0xae, 0x9d, 0x76, 0x07, 0x85, 0xaf, 0xc1, 0x6f, 0x59, 0xaf, 0x58, 0x6e, 0xec, 0x4e,
	0x7e, 0xcf, 0x3a, 0xfc, 0xb3, 0x8e, 0x2f, 0xe1, 0x61, 0xa6, 0x15, 0x2f, 0x22, 0x13, 0x7f, 0x8d,
	0xcd, 0x61, 0xb3, 0xd7, 0x22, 0xdf, 0xab, 0x84, 0xfb, 0x7d, 0xdb, 0xf8, 0xf7, 0x22, 0xf9, 0x08,
	0x6e, 0x50, 0x48, 0x8e, 0x04, 0x86, 0x46, 0x19, 0x96, 0xcc, 0x59, 0xc2, 0x64, 0x24, 0xea, 0x83,
	0xdf, 0xd2, 0xf0, 0x19, 0x78, 0x85, 0x8c, 0x12, 0x16, 0xa7, 0x82, 0x1f, 0xb9, 0xea, 0xec, 0x7f,
	0xe8, 0x64, 0x04, 0xf7, 0xab, 0x2b, 0x5e, 0xc6, 0xb9, 0x51, 0xfa, 0x40, 0x9e, 0x42, 0x7f, 0x16,
	0x45, 0xaa, 0x90, 0x06, 0x7d, 0xe8, 0x6f, 0x6f, 0x8d, 0x39, 0xa6, 0xe4, 0x1c, 0x7a, 0xcb, 0xef,
	0x22, 0xcd, 0x0c, 0x9e, 0x41, 0x97, 0x71, 0xae, 0x73, 0xdf, 0x19, 0x9f, 0x4c, 0x86, 0xb4, 0x4a,
	0xc8, 0x0f, 0x07, 0x06, 0x95, 0xed, 0x4a, 0xed, 0xf0, 0x15, 0xb8, 0xe6, 0x90, 0x55, 0x26, 0x0f,
	0x9e, 0x3f, 0x99, 0x36, 0x3e, 0x69, 0x7a, 0x43, 0xd5, 0xd1, 0xe6, 0x90, 0x09, 0x6a, 0x71, 0x44,
	0x70, 0x4b, 0xb7, 0x7a, 0x75, 0x1b, 0xe3, 0x23, 0xe8, 0xb1, 0xb4, 0x5c, 0xce, 0xbe, 0xc5, 0x80,
	0xd6, 0x19, 0x09, 0x00, 0x7e, 0xf7, 0xa3, 0x07, 0xc3, 0xf9, 0x2a, 0xbc, 0x78, 0x77, 0x4d, 0x97,
	0x9f, 0x66, 0x74, 0xe1, 0xdd, 0x2b, 0x95, 0xe5, 0xfb, 0xf0, 0xe2, 0xf2, 0xa8, 0x38, 0x78, 0x06,
	0x5e, 0x10, 0x5e, 0xad, 0x17, 0xb3, 0xcd, 0xdb, 0x70, 0x7d, 0x3d, 0x0f, 0xd7, 0x57, 0x1f, 0xbc,
	0xce, 0xb6, 0x67, 0x9f, 0xfd, 0xc5, 0xaf, 0x00, 0x00, 0x00, 0xff, 0xff, 0xba, 0x21, 0x0f, 0x1b,
	0xff, 0x02, 0x00, 0x00,
}
