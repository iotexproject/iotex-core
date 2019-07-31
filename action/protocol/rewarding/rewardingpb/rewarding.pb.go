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
	BeringHeight                   uint64   `protobuf:"varint,8,opt,name=beringHeight,proto3" json:"beringHeight,omitempty"`
	BeringBlockReward              string   `protobuf:"bytes,9,opt,name=beringBlockReward,proto3" json:"beringBlockReward,omitempty"`
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

func (m *Admin) GetBeringHeight() uint64 {
	if m != nil {
		return m.BeringHeight
	}
	return 0
}

func (m *Admin) GetBeringBlockReward() string {
	if m != nil {
		return m.BeringBlockReward
	}
	return ""
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
	// 447 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x93, 0xdf, 0x8f, 0x93, 0x40,
	0x10, 0xc7, 0xe5, 0x4a, 0x5b, 0x3b, 0x57, 0x2d, 0x4e, 0x4e, 0x43, 0x7c, 0xb8, 0x54, 0x7c, 0x69,
	0x8c, 0xe9, 0x83, 0x3f, 0x5e, 0x7c, 0x30, 0x29, 0xd7, 0x92, 0x1a, 0x9b, 0x62, 0xb0, 0xa7, 0x8f,
	0x97, 0x05, 0x56, 0xba, 0x11, 0x76, 0xc9, 0xb2, 0xa8, 0xfd, 0xc7, 0x7c, 0xf7, 0x3f, 0x33, 0x5d,
	0xe8, 0x49, 0x39, 0xf5, 0xde, 0x76, 0xbe, 0xf3, 0x99, 0xef, 0x4c, 0x98, 0x01, 0x46, 0x92, 0x7e,
	0x27, 0x32, 0x66, 0x3c, 0x99, 0xe6, 0x52, 0x28, 0x81, 0xa7, 0xd7, 0x42, 0x1e, 0x3a, 0xbf, 0x3a,
	0xd0, 0x9d, 0xc5, 0x19, 0xe3, 0x38, 0x86, 0xd3, 0x30, 0x15, 0xd1, 0xd7, 0x40, 0x67, 0x6d, 0x63,
	0x6c, 0x4c, 0x06, 0x41, 0x53, 0xda, 0x13, 0x34, 0x17, 0xd1, 0xb6, 0x26, 0x4e, 0x2a, 0xa2, 0x21,
	0xe1, 0x5b, 0x78, 0xcc, 0xcb, 0x6c, 0x4e, 0x53, 0x9a, 0x10, 0x45, 0x0b, 0x4f, 0xc8, 0x45, 0xa3,
	0xa0, 0x33, 0x36, 0x26, 0x66, 0xf0, 0x1f, 0x02, 0x27, 0x30, 0xfa, 0x22, 0x4a, 0x1e, 0x13, 0xc5,
	0x04, 0x77, 0x05, 0x2f, 0x0b, 0xdb, 0xd4, 0x5d, 0xda, 0x32, 0x7a, 0x70, 0xde, 0xf2, 0xf1, 0x5a,
	0x85, 0x5d, 0xdd, 0xed, 0x16, 0x0a, 0xdf, 0x80, 0xdd, 0xb2, 0x5e, 0x91, 0x42, 0xe9, 0x99, 0xec,
	0x9e, 0x76, 0xf8, 0x67, 0x1e, 0x5f, 0xc1, 0xc3, 0x5c, 0x8a, 0xb8, 0x8c, 0x14, 0xfb, 0xc6, 0xd4,
	0x6e, 0xb3, 0x95, 0xb4, 0xd8, 0x8a, 0x34, 0xb6, 0xfb, 0xba, 0xf0, 0xef, 0x49, 0x74, 0x60, 0x18,
	0x52, 0xc9, 0x78, 0xb2, 0xa4, 0x2c, 0xd9, 0x2a, 0xfb, 0xae, 0x86, 0x8f, 0x34, 0x7c, 0x0e, 0x0f,
	0xaa, 0xd8, 0x6d, 0x6c, 0x64, 0xa0, 0xbf, 0xc4, 0xcd, 0x84, 0xf3, 0x09, 0x4c, 0xaf, 0xe4, 0xda,
	0x59, 0x09, 0x45, 0x52, 0x97, 0xa4, 0x84, 0x47, 0xb4, 0x5e, 0xe1, 0x91, 0x86, 0xcf, 0xc0, 0x2a,
	0x79, 0x94, 0x12, 0x96, 0xd1, 0xf8, 0xc0, 0x55, 0x8b, 0xbc, 0xa1, 0x3b, 0x23, 0xb8, 0x57, 0x75,
	0x58, 0xb2, 0x42, 0x09, 0xb9, 0x73, 0x9e, 0x42, 0x7f, 0x16, 0x45, 0xa2, 0xe4, 0x0a, 0x6d, 0xe8,
	0x87, 0x47, 0x6d, 0x0e, 0xa1, 0x73, 0x0e, 0xbd, 0xc5, 0x0f, 0x9a, 0xe5, 0x0a, 0xcf, 0xa0, 0x4b,
	0xe2, 0x58, 0x16, 0xb6, 0x31, 0xee, 0x4c, 0x86, 0x41, 0x15, 0x38, 0x3f, 0x0d, 0x18, 0x54, 0xb6,
	0x2b, 0x91, 0xe0, 0x6b, 0x30, 0xd5, 0x2e, 0xaf, 0x4c, 0xee, 0xbf, 0x78, 0x32, 0x6d, 0xdc, 0xe6,
	0xf4, 0x9a, 0xaa, 0x5f, 0x9b, 0x5d, 0x4e, 0x03, 0x8d, 0x23, 0x82, 0xb9, 0x77, 0xab, 0x47, 0xd7,
	0x6f, 0x7c, 0x04, 0x3d, 0x92, 0xed, 0x87, 0xd3, 0x87, 0x36, 0x08, 0xea, 0xc8, 0xf1, 0x00, 0xfe,
	0xd4, 0xa3, 0x05, 0x43, 0x77, 0xe5, 0x5f, 0xbc, 0xbf, 0x0a, 0x16, 0x9f, 0x67, 0xc1, 0xdc, 0xba,
	0xb3, 0x57, 0x16, 0x1f, 0xfc, 0x8b, 0xe5, 0x41, 0x31, 0xf0, 0x0c, 0x2c, 0xcf, 0xbf, 0x5c, 0xcf,
	0x67, 0x9b, 0x77, 0xfe, 0xfa, 0xca, 0xf5, 0xd7, 0x97, 0x1f, 0xad, 0x93, 0xb0, 0xa7, 0x7f, 0x9f,
	0x97, 0xbf, 0x03, 0x00, 0x00, 0xff, 0xff, 0xf2, 0xa8, 0x70, 0xfb, 0x51, 0x03, 0x00, 0x00,
}
