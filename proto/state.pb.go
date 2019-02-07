// Code generated by protoc-gen-go. DO NOT EDIT.
// source: state.proto

package iproto

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

type AccountPb struct {
	// used by state-based model
	Nonce                uint64   `protobuf:"varint,1,opt,name=nonce" json:"nonce,omitempty"`
	Balance              []byte   `protobuf:"bytes,2,opt,name=balance,proto3" json:"balance,omitempty"`
	Root                 []byte   `protobuf:"bytes,3,opt,name=root,proto3" json:"root,omitempty"`
	CodeHash             []byte   `protobuf:"bytes,4,opt,name=codeHash,proto3" json:"codeHash,omitempty"`
	IsCandidate          bool     `protobuf:"varint,5,opt,name=isCandidate" json:"isCandidate,omitempty"`
	VotingWeight         []byte   `protobuf:"bytes,6,opt,name=votingWeight,proto3" json:"votingWeight,omitempty"`
	Votee                string   `protobuf:"bytes,7,opt,name=votee" json:"votee,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AccountPb) Reset()         { *m = AccountPb{} }
func (m *AccountPb) String() string { return proto.CompactTextString(m) }
func (*AccountPb) ProtoMessage()    {}
func (*AccountPb) Descriptor() ([]byte, []int) {
	return fileDescriptor_state_8c2f1a495c8ff520, []int{0}
}
func (m *AccountPb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AccountPb.Unmarshal(m, b)
}
func (m *AccountPb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AccountPb.Marshal(b, m, deterministic)
}
func (dst *AccountPb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AccountPb.Merge(dst, src)
}
func (m *AccountPb) XXX_Size() int {
	return xxx_messageInfo_AccountPb.Size(m)
}
func (m *AccountPb) XXX_DiscardUnknown() {
	xxx_messageInfo_AccountPb.DiscardUnknown(m)
}

var xxx_messageInfo_AccountPb proto.InternalMessageInfo

func (m *AccountPb) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *AccountPb) GetBalance() []byte {
	if m != nil {
		return m.Balance
	}
	return nil
}

func (m *AccountPb) GetRoot() []byte {
	if m != nil {
		return m.Root
	}
	return nil
}

func (m *AccountPb) GetCodeHash() []byte {
	if m != nil {
		return m.CodeHash
	}
	return nil
}

func (m *AccountPb) GetIsCandidate() bool {
	if m != nil {
		return m.IsCandidate
	}
	return false
}

func (m *AccountPb) GetVotingWeight() []byte {
	if m != nil {
		return m.VotingWeight
	}
	return nil
}

func (m *AccountPb) GetVotee() string {
	if m != nil {
		return m.Votee
	}
	return ""
}

// Account Metadata
type AccountMeta struct {
	Address              string   `protobuf:"bytes,1,opt,name=address" json:"address,omitempty"`
	Balance              string   `protobuf:"bytes,2,opt,name=balance" json:"balance,omitempty"`
	Nonce                uint64   `protobuf:"varint,3,opt,name=nonce" json:"nonce,omitempty"`
	PendingNonce         uint64   `protobuf:"varint,4,opt,name=pendingNonce" json:"pendingNonce,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AccountMeta) Reset()         { *m = AccountMeta{} }
func (m *AccountMeta) String() string { return proto.CompactTextString(m) }
func (*AccountMeta) ProtoMessage()    {}
func (*AccountMeta) Descriptor() ([]byte, []int) {
	return fileDescriptor_state_8c2f1a495c8ff520, []int{1}
}
func (m *AccountMeta) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AccountMeta.Unmarshal(m, b)
}
func (m *AccountMeta) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AccountMeta.Marshal(b, m, deterministic)
}
func (dst *AccountMeta) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AccountMeta.Merge(dst, src)
}
func (m *AccountMeta) XXX_Size() int {
	return xxx_messageInfo_AccountMeta.Size(m)
}
func (m *AccountMeta) XXX_DiscardUnknown() {
	xxx_messageInfo_AccountMeta.DiscardUnknown(m)
}

var xxx_messageInfo_AccountMeta proto.InternalMessageInfo

func (m *AccountMeta) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *AccountMeta) GetBalance() string {
	if m != nil {
		return m.Balance
	}
	return ""
}

func (m *AccountMeta) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *AccountMeta) GetPendingNonce() uint64 {
	if m != nil {
		return m.PendingNonce
	}
	return 0
}

func init() {
	proto.RegisterType((*AccountPb)(nil), "iproto.AccountPb")
	proto.RegisterType((*AccountMeta)(nil), "iproto.AccountMeta")
}

func init() { proto.RegisterFile("state.proto", fileDescriptor_state_8c2f1a495c8ff520) }

var fileDescriptor_state_8c2f1a495c8ff520 = []byte{
	// 231 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0x90, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x40, 0x65, 0x9a, 0xa6, 0xcd, 0x25, 0x93, 0xc5, 0x60, 0x31, 0x59, 0x99, 0x32, 0xb1, 0xf0,
	0x05, 0x88, 0x85, 0x05, 0x84, 0xbc, 0x30, 0x3b, 0xf6, 0x29, 0xb5, 0x84, 0x7c, 0x55, 0x7c, 0x74,
	0xe1, 0x03, 0xf9, 0x2d, 0x94, 0x2b, 0x2d, 0x2d, 0x93, 0xfd, 0xde, 0x49, 0xd6, 0x3d, 0x43, 0x5b,
	0xd8, 0x33, 0xde, 0xef, 0x67, 0x62, 0xd2, 0x75, 0x92, 0xb3, 0xff, 0x56, 0xd0, 0x3c, 0x86, 0x40,
	0x9f, 0x99, 0xdf, 0x46, 0x7d, 0x0b, 0xeb, 0x4c, 0x39, 0xa0, 0x51, 0x56, 0x0d, 0x95, 0x3b, 0x82,
	0x36, 0xb0, 0x19, 0xfd, 0x87, 0x5f, 0xfc, 0x8d, 0x55, 0x43, 0xe7, 0x4e, 0xa8, 0x35, 0x54, 0x33,
	0x11, 0x9b, 0x95, 0x68, 0xb9, 0xeb, 0x3b, 0xd8, 0x06, 0x8a, 0xf8, 0xec, 0xcb, 0xce, 0x54, 0xe2,
	0xcf, 0xac, 0x2d, 0xb4, 0xa9, 0x3c, 0xf9, 0x1c, 0x53, 0xf4, 0x8c, 0x66, 0x6d, 0xd5, 0xb0, 0x75,
	0x97, 0x4a, 0xf7, 0xd0, 0x1d, 0x88, 0x53, 0x9e, 0xde, 0x31, 0x4d, 0x3b, 0x36, 0xb5, 0xbc, 0x70,
	0xe5, 0x96, 0x2d, 0x0f, 0xc4, 0x88, 0x66, 0x63, 0xd5, 0xd0, 0xb8, 0x23, 0xf4, 0x5f, 0xd0, 0xfe,
	0x86, 0xbc, 0x20, 0xfb, 0x65, 0x69, 0x1f, 0xe3, 0x8c, 0xa5, 0x48, 0x4c, 0xe3, 0x4e, 0xf8, 0x3f,
	0xa7, 0xf9, 0xcb, 0x39, 0xe7, 0xaf, 0x2e, 0xf3, 0x7b, 0xe8, 0xf6, 0x98, 0x63, 0xca, 0xd3, 0xab,
	0x0c, 0x2b, 0x19, 0x5e, 0xb9, 0xb1, 0x96, 0xdf, 0x7c, 0xf8, 0x09, 0x00, 0x00, 0xff, 0xff, 0xe6,
	0x76, 0x0a, 0x77, 0x64, 0x01, 0x00, 0x00,
}
