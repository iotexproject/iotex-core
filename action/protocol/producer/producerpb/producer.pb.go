// Code generated by protoc-gen-go. DO NOT EDIT.
// source: producer.proto

package producerpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Admin struct {
	Admin                []byte   `protobuf:"bytes,1,opt,name=admin,proto3" json:"admin,omitempty"`
	BlockReward          []byte   `protobuf:"bytes,2,opt,name=blockReward,proto3" json:"blockReward,omitempty"`
	EpochReward          []byte   `protobuf:"bytes,3,opt,name=epochReward,proto3" json:"epochReward,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Admin) Reset()         { *m = Admin{} }
func (m *Admin) String() string { return proto.CompactTextString(m) }
func (*Admin) ProtoMessage()    {}
func (*Admin) Descriptor() ([]byte, []int) {
	return fileDescriptor_producer_6ec2892fad41ed17, []int{0}
}
func (m *Admin) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Admin.Unmarshal(m, b)
}
func (m *Admin) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Admin.Marshal(b, m, deterministic)
}
func (dst *Admin) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Admin.Merge(dst, src)
}
func (m *Admin) XXX_Size() int {
	return xxx_messageInfo_Admin.Size(m)
}
func (m *Admin) XXX_DiscardUnknown() {
	xxx_messageInfo_Admin.DiscardUnknown(m)
}

var xxx_messageInfo_Admin proto.InternalMessageInfo

func (m *Admin) GetAdmin() []byte {
	if m != nil {
		return m.Admin
	}
	return nil
}

func (m *Admin) GetBlockReward() []byte {
	if m != nil {
		return m.BlockReward
	}
	return nil
}

func (m *Admin) GetEpochReward() []byte {
	if m != nil {
		return m.EpochReward
	}
	return nil
}

type Fund struct {
	TotalBalance         []byte   `protobuf:"bytes,1,opt,name=totalBalance,proto3" json:"totalBalance,omitempty"`
	AvailableBalance     []byte   `protobuf:"bytes,2,opt,name=availableBalance,proto3" json:"availableBalance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Fund) Reset()         { *m = Fund{} }
func (m *Fund) String() string { return proto.CompactTextString(m) }
func (*Fund) ProtoMessage()    {}
func (*Fund) Descriptor() ([]byte, []int) {
	return fileDescriptor_producer_6ec2892fad41ed17, []int{1}
}
func (m *Fund) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Fund.Unmarshal(m, b)
}
func (m *Fund) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Fund.Marshal(b, m, deterministic)
}
func (dst *Fund) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Fund.Merge(dst, src)
}
func (m *Fund) XXX_Size() int {
	return xxx_messageInfo_Fund.Size(m)
}
func (m *Fund) XXX_DiscardUnknown() {
	xxx_messageInfo_Fund.DiscardUnknown(m)
}

var xxx_messageInfo_Fund proto.InternalMessageInfo

func (m *Fund) GetTotalBalance() []byte {
	if m != nil {
		return m.TotalBalance
	}
	return nil
}

func (m *Fund) GetAvailableBalance() []byte {
	if m != nil {
		return m.AvailableBalance
	}
	return nil
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
	return fileDescriptor_producer_6ec2892fad41ed17, []int{2}
}
func (m *RewardHistory) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RewardHistory.Unmarshal(m, b)
}
func (m *RewardHistory) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RewardHistory.Marshal(b, m, deterministic)
}
func (dst *RewardHistory) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RewardHistory.Merge(dst, src)
}
func (m *RewardHistory) XXX_Size() int {
	return xxx_messageInfo_RewardHistory.Size(m)
}
func (m *RewardHistory) XXX_DiscardUnknown() {
	xxx_messageInfo_RewardHistory.DiscardUnknown(m)
}

var xxx_messageInfo_RewardHistory proto.InternalMessageInfo

type Account struct {
	Balance              []byte   `protobuf:"bytes,2,opt,name=balance,proto3" json:"balance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Account) Reset()         { *m = Account{} }
func (m *Account) String() string { return proto.CompactTextString(m) }
func (*Account) ProtoMessage()    {}
func (*Account) Descriptor() ([]byte, []int) {
	return fileDescriptor_producer_6ec2892fad41ed17, []int{3}
}
func (m *Account) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Account.Unmarshal(m, b)
}
func (m *Account) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Account.Marshal(b, m, deterministic)
}
func (dst *Account) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Account.Merge(dst, src)
}
func (m *Account) XXX_Size() int {
	return xxx_messageInfo_Account.Size(m)
}
func (m *Account) XXX_DiscardUnknown() {
	xxx_messageInfo_Account.DiscardUnknown(m)
}

var xxx_messageInfo_Account proto.InternalMessageInfo

func (m *Account) GetBalance() []byte {
	if m != nil {
		return m.Balance
	}
	return nil
}

func init() {
	proto.RegisterType((*Admin)(nil), "producerpb.Admin")
	proto.RegisterType((*Fund)(nil), "producerpb.Fund")
	proto.RegisterType((*RewardHistory)(nil), "producerpb.RewardHistory")
	proto.RegisterType((*Account)(nil), "producerpb.Account")
}

func init() { proto.RegisterFile("producer.proto", fileDescriptor_producer_6ec2892fad41ed17) }

var fileDescriptor_producer_6ec2892fad41ed17 = []byte{
	// 191 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2b, 0x28, 0xca, 0x4f,
	0x29, 0x4d, 0x4e, 0x2d, 0xd2, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x82, 0xf1, 0x0b, 0x92,
	0x94, 0x12, 0xb9, 0x58, 0x1d, 0x53, 0x72, 0x33, 0xf3, 0x84, 0x44, 0xb8, 0x58, 0x13, 0x41, 0x0c,
	0x09, 0x46, 0x05, 0x46, 0x0d, 0x9e, 0x20, 0x08, 0x47, 0x48, 0x81, 0x8b, 0x3b, 0x29, 0x27, 0x3f,
	0x39, 0x3b, 0x28, 0xb5, 0x3c, 0xb1, 0x28, 0x45, 0x82, 0x09, 0x2c, 0x87, 0x2c, 0x04, 0x52, 0x91,
	0x5a, 0x90, 0x9f, 0x9c, 0x01, 0x55, 0xc1, 0x0c, 0x51, 0x81, 0x24, 0xa4, 0x14, 0xc6, 0xc5, 0xe2,
	0x56, 0x9a, 0x97, 0x22, 0xa4, 0xc4, 0xc5, 0x53, 0x92, 0x5f, 0x92, 0x98, 0xe3, 0x94, 0x98, 0x93,
	0x98, 0x97, 0x9c, 0x0a, 0xb5, 0x08, 0x45, 0x4c, 0x48, 0x8b, 0x4b, 0x20, 0xb1, 0x2c, 0x31, 0x33,
	0x27, 0x31, 0x29, 0x27, 0x15, 0xa6, 0x0e, 0x62, 0x29, 0x86, 0xb8, 0x12, 0x3f, 0x17, 0x2f, 0xc4,
	0x06, 0x8f, 0xcc, 0xe2, 0x92, 0xfc, 0xa2, 0x4a, 0x25, 0x65, 0x2e, 0x76, 0xc7, 0xe4, 0xe4, 0xfc,
	0xd2, 0xbc, 0x12, 0x21, 0x09, 0x2e, 0xf6, 0x24, 0x14, 0xed, 0x30, 0x6e, 0x12, 0x1b, 0x38, 0x0c,
	0x8c, 0x01, 0x01, 0x00, 0x00, 0xff, 0xff, 0x83, 0xe3, 0x95, 0xef, 0x15, 0x01, 0x00, 0x00,
}
