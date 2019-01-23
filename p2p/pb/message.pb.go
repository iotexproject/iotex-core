// Code generated by protoc-gen-go. DO NOT EDIT.
// source: message.proto

package p2ppb

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

type BroadcastMsg struct {
	ChainId              uint32   `protobuf:"varint,1,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	Addr                 string   `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	MsgType              uint32   `protobuf:"varint,3,opt,name=msg_type,json=msgType,proto3" json:"msg_type,omitempty"`
	MsgBody              []byte   `protobuf:"bytes,4,opt,name=msg_body,json=msgBody,proto3" json:"msg_body,omitempty"`
	PeerId               string   `protobuf:"bytes,5,opt,name=peer_id,json=peerId,proto3" json:"peer_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BroadcastMsg) Reset()         { *m = BroadcastMsg{} }
func (m *BroadcastMsg) String() string { return proto.CompactTextString(m) }
func (*BroadcastMsg) ProtoMessage()    {}
func (*BroadcastMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_fb34bb067a4226be, []int{0}
}
func (m *BroadcastMsg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BroadcastMsg.Unmarshal(m, b)
}
func (m *BroadcastMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BroadcastMsg.Marshal(b, m, deterministic)
}
func (dst *BroadcastMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BroadcastMsg.Merge(dst, src)
}
func (m *BroadcastMsg) XXX_Size() int {
	return xxx_messageInfo_BroadcastMsg.Size(m)
}
func (m *BroadcastMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_BroadcastMsg.DiscardUnknown(m)
}

var xxx_messageInfo_BroadcastMsg proto.InternalMessageInfo

func (m *BroadcastMsg) GetChainId() uint32 {
	if m != nil {
		return m.ChainId
	}
	return 0
}

func (m *BroadcastMsg) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

func (m *BroadcastMsg) GetMsgType() uint32 {
	if m != nil {
		return m.MsgType
	}
	return 0
}

func (m *BroadcastMsg) GetMsgBody() []byte {
	if m != nil {
		return m.MsgBody
	}
	return nil
}

func (m *BroadcastMsg) GetPeerId() string {
	if m != nil {
		return m.PeerId
	}
	return ""
}

type UnicastMsg struct {
	ChainId              uint32   `protobuf:"varint,1,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	Addr                 string   `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	MsgType              uint32   `protobuf:"varint,3,opt,name=msg_type,json=msgType,proto3" json:"msg_type,omitempty"`
	MsgBody              []byte   `protobuf:"bytes,4,opt,name=msg_body,json=msgBody,proto3" json:"msg_body,omitempty"`
	PeerId               string   `protobuf:"bytes,5,opt,name=peer_id,json=peerId,proto3" json:"peer_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *UnicastMsg) Reset()         { *m = UnicastMsg{} }
func (m *UnicastMsg) String() string { return proto.CompactTextString(m) }
func (*UnicastMsg) ProtoMessage()    {}
func (*UnicastMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_fb34bb067a4226be, []int{1}
}
func (m *UnicastMsg) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_UnicastMsg.Unmarshal(m, b)
}
func (m *UnicastMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_UnicastMsg.Marshal(b, m, deterministic)
}
func (dst *UnicastMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_UnicastMsg.Merge(dst, src)
}
func (m *UnicastMsg) XXX_Size() int {
	return xxx_messageInfo_UnicastMsg.Size(m)
}
func (m *UnicastMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_UnicastMsg.DiscardUnknown(m)
}

var xxx_messageInfo_UnicastMsg proto.InternalMessageInfo

func (m *UnicastMsg) GetChainId() uint32 {
	if m != nil {
		return m.ChainId
	}
	return 0
}

func (m *UnicastMsg) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

func (m *UnicastMsg) GetMsgType() uint32 {
	if m != nil {
		return m.MsgType
	}
	return 0
}

func (m *UnicastMsg) GetMsgBody() []byte {
	if m != nil {
		return m.MsgBody
	}
	return nil
}

func (m *UnicastMsg) GetPeerId() string {
	if m != nil {
		return m.PeerId
	}
	return ""
}

func init() {
	proto.RegisterType((*BroadcastMsg)(nil), "p2ppb.BroadcastMsg")
	proto.RegisterType((*UnicastMsg)(nil), "p2ppb.UnicastMsg")
}

func init() { proto.RegisterFile("message.proto", fileDescriptor_message_fb34bb067a4226be) }

var fileDescriptor_message_fb34bb067a4226be = []byte{
	// 177 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcd, 0x4d, 0x2d, 0x2e,
	0x4e, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2d, 0x30, 0x2a, 0x28, 0x48,
	0x52, 0xea, 0x61, 0xe4, 0xe2, 0x71, 0x2a, 0xca, 0x4f, 0x4c, 0x49, 0x4e, 0x2c, 0x2e, 0xf1, 0x2d,
	0x4e, 0x17, 0x92, 0xe4, 0xe2, 0x48, 0xce, 0x48, 0xcc, 0xcc, 0x8b, 0xcf, 0x4c, 0x91, 0x60, 0x54,
	0x60, 0xd4, 0xe0, 0x0d, 0x62, 0x07, 0xf3, 0x3d, 0x53, 0x84, 0x84, 0xb8, 0x58, 0x12, 0x53, 0x52,
	0x8a, 0x24, 0x98, 0x14, 0x18, 0x35, 0x38, 0x83, 0xc0, 0x6c, 0x90, 0xf2, 0xdc, 0xe2, 0xf4, 0xf8,
	0x92, 0xca, 0x82, 0x54, 0x09, 0x66, 0x88, 0xf2, 0xdc, 0xe2, 0xf4, 0x90, 0xca, 0x82, 0x54, 0x98,
	0x54, 0x52, 0x7e, 0x4a, 0xa5, 0x04, 0x8b, 0x02, 0xa3, 0x06, 0x0f, 0x58, 0xca, 0x29, 0x3f, 0xa5,
	0x52, 0x48, 0x9c, 0x8b, 0xbd, 0x20, 0x35, 0xb5, 0x08, 0x64, 0x07, 0x2b, 0xd8, 0x30, 0x36, 0x10,
	0xd7, 0x33, 0x45, 0xa9, 0x8b, 0x91, 0x8b, 0x2b, 0x34, 0x2f, 0x73, 0x50, 0x38, 0x26, 0x89, 0x0d,
	0x1c, 0x52, 0xc6, 0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0xd4, 0xdb, 0x0d, 0x19, 0x3a, 0x01, 0x00,
	0x00,
}
