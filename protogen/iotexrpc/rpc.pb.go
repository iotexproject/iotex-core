// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

package iotexrpc // import "github.com/iotexproject/iotex-core/protogen/iotexrpc"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import timestamp "github.com/golang/protobuf/ptypes/timestamp"
import iotextypes "github.com/iotexproject/iotex-core/protogen/iotextypes"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ConsensusPb_ConsensusMessageType int32

const (
	ConsensusPb_PROPOSAL    ConsensusPb_ConsensusMessageType = 0
	ConsensusPb_ENDORSEMENT ConsensusPb_ConsensusMessageType = 1
)

var ConsensusPb_ConsensusMessageType_name = map[int32]string{
	0: "PROPOSAL",
	1: "ENDORSEMENT",
}
var ConsensusPb_ConsensusMessageType_value = map[string]int32{
	"PROPOSAL":    0,
	"ENDORSEMENT": 1,
}

func (x ConsensusPb_ConsensusMessageType) String() string {
	return proto.EnumName(ConsensusPb_ConsensusMessageType_name, int32(x))
}
func (ConsensusPb_ConsensusMessageType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_rpc_030e7b18e34d83ff, []int{2, 0}
}

type BlockSync struct {
	Start                uint64   `protobuf:"varint,2,opt,name=start,proto3" json:"start,omitempty"`
	End                  uint64   `protobuf:"varint,3,opt,name=end,proto3" json:"end,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BlockSync) Reset()         { *m = BlockSync{} }
func (m *BlockSync) String() string { return proto.CompactTextString(m) }
func (*BlockSync) ProtoMessage()    {}
func (*BlockSync) Descriptor() ([]byte, []int) {
	return fileDescriptor_rpc_030e7b18e34d83ff, []int{0}
}
func (m *BlockSync) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockSync.Unmarshal(m, b)
}
func (m *BlockSync) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockSync.Marshal(b, m, deterministic)
}
func (dst *BlockSync) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockSync.Merge(dst, src)
}
func (m *BlockSync) XXX_Size() int {
	return xxx_messageInfo_BlockSync.Size(m)
}
func (m *BlockSync) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockSync.DiscardUnknown(m)
}

var xxx_messageInfo_BlockSync proto.InternalMessageInfo

func (m *BlockSync) GetStart() uint64 {
	if m != nil {
		return m.Start
	}
	return 0
}

func (m *BlockSync) GetEnd() uint64 {
	if m != nil {
		return m.End
	}
	return 0
}

// block container
// used to send old/existing blocks in block sync
type BlockContainer struct {
	Block                *iotextypes.BlockPb `protobuf:"bytes,1,opt,name=block,proto3" json:"block,omitempty"`
	XXX_NoUnkeyedLiteral struct{}            `json:"-"`
	XXX_unrecognized     []byte              `json:"-"`
	XXX_sizecache        int32               `json:"-"`
}

func (m *BlockContainer) Reset()         { *m = BlockContainer{} }
func (m *BlockContainer) String() string { return proto.CompactTextString(m) }
func (*BlockContainer) ProtoMessage()    {}
func (*BlockContainer) Descriptor() ([]byte, []int) {
	return fileDescriptor_rpc_030e7b18e34d83ff, []int{1}
}
func (m *BlockContainer) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockContainer.Unmarshal(m, b)
}
func (m *BlockContainer) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockContainer.Marshal(b, m, deterministic)
}
func (dst *BlockContainer) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockContainer.Merge(dst, src)
}
func (m *BlockContainer) XXX_Size() int {
	return xxx_messageInfo_BlockContainer.Size(m)
}
func (m *BlockContainer) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockContainer.DiscardUnknown(m)
}

var xxx_messageInfo_BlockContainer proto.InternalMessageInfo

func (m *BlockContainer) GetBlock() *iotextypes.BlockPb {
	if m != nil {
		return m.Block
	}
	return nil
}

type ConsensusPb struct {
	Height               uint64                           `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	Round                uint32                           `protobuf:"varint,2,opt,name=round,proto3" json:"round,omitempty"`
	Type                 ConsensusPb_ConsensusMessageType `protobuf:"varint,3,opt,name=type,proto3,enum=iotexrpc.ConsensusPb_ConsensusMessageType" json:"type,omitempty"`
	Timestamp            *timestamp.Timestamp             `protobuf:"bytes,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Data                 []byte                           `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                         `json:"-"`
	XXX_unrecognized     []byte                           `json:"-"`
	XXX_sizecache        int32                            `json:"-"`
}

func (m *ConsensusPb) Reset()         { *m = ConsensusPb{} }
func (m *ConsensusPb) String() string { return proto.CompactTextString(m) }
func (*ConsensusPb) ProtoMessage()    {}
func (*ConsensusPb) Descriptor() ([]byte, []int) {
	return fileDescriptor_rpc_030e7b18e34d83ff, []int{2}
}
func (m *ConsensusPb) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConsensusPb.Unmarshal(m, b)
}
func (m *ConsensusPb) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConsensusPb.Marshal(b, m, deterministic)
}
func (dst *ConsensusPb) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConsensusPb.Merge(dst, src)
}
func (m *ConsensusPb) XXX_Size() int {
	return xxx_messageInfo_ConsensusPb.Size(m)
}
func (m *ConsensusPb) XXX_DiscardUnknown() {
	xxx_messageInfo_ConsensusPb.DiscardUnknown(m)
}

var xxx_messageInfo_ConsensusPb proto.InternalMessageInfo

func (m *ConsensusPb) GetHeight() uint64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *ConsensusPb) GetRound() uint32 {
	if m != nil {
		return m.Round
	}
	return 0
}

func (m *ConsensusPb) GetType() ConsensusPb_ConsensusMessageType {
	if m != nil {
		return m.Type
	}
	return ConsensusPb_PROPOSAL
}

func (m *ConsensusPb) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *ConsensusPb) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterType((*BlockSync)(nil), "iotexrpc.BlockSync")
	proto.RegisterType((*BlockContainer)(nil), "iotexrpc.BlockContainer")
	proto.RegisterType((*ConsensusPb)(nil), "iotexrpc.ConsensusPb")
	proto.RegisterEnum("iotexrpc.ConsensusPb_ConsensusMessageType", ConsensusPb_ConsensusMessageType_name, ConsensusPb_ConsensusMessageType_value)
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor_rpc_030e7b18e34d83ff) }

var fileDescriptor_rpc_030e7b18e34d83ff = []byte{
	// 351 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x51, 0x41, 0x4f, 0xf2, 0x40,
	0x10, 0xfd, 0x0a, 0x85, 0xc0, 0xc2, 0xc7, 0xd7, 0xec, 0x47, 0x4c, 0xc3, 0x45, 0xd2, 0x13, 0x9a,
	0xb8, 0x4d, 0x40, 0x8d, 0x89, 0x89, 0x89, 0x20, 0x37, 0x81, 0x66, 0xe1, 0xe4, 0xad, 0x5d, 0xd6,
	0xb6, 0x0a, 0xbb, 0xcd, 0xee, 0x36, 0x91, 0x9b, 0x3f, 0xdd, 0x74, 0x4a, 0xc5, 0x83, 0xb7, 0x79,
	0xb3, 0xef, 0xcd, 0xbc, 0x79, 0x8b, 0xda, 0x2a, 0x63, 0x24, 0x53, 0xd2, 0x48, 0xdc, 0x4a, 0xa5,
	0xe1, 0x1f, 0x2a, 0x63, 0x03, 0x27, 0xda, 0x49, 0xf6, 0xce, 0x92, 0x30, 0x15, 0xe5, 0xdb, 0xe0,
	0x3c, 0x96, 0x32, 0xde, 0x71, 0x1f, 0x50, 0x94, 0xbf, 0xfa, 0x26, 0xdd, 0x73, 0x6d, 0xc2, 0x7d,
	0x56, 0x12, 0xbc, 0x09, 0x6a, 0x4f, 0x0b, 0xd1, 0xfa, 0x20, 0x18, 0xee, 0xa3, 0x86, 0x36, 0xa1,
	0x32, 0x6e, 0x6d, 0x68, 0x8d, 0x6c, 0x5a, 0x02, 0xec, 0xa0, 0x3a, 0x17, 0x5b, 0xb7, 0x0e, 0xbd,
	0xa2, 0xf4, 0xee, 0x51, 0x0f, 0x44, 0x33, 0x29, 0x4c, 0x98, 0x0a, 0xae, 0xf0, 0x05, 0x6a, 0xc0,
	0x6e, 0xd7, 0x1a, 0x5a, 0xa3, 0xce, 0xf8, 0x3f, 0x01, 0x4f, 0xe6, 0x90, 0x71, 0x4d, 0x80, 0x1a,
	0x44, 0xb4, 0x64, 0x78, 0x9f, 0x35, 0xd4, 0x99, 0x49, 0xa1, 0xb9, 0xd0, 0xb9, 0x0e, 0x22, 0x7c,
	0x86, 0x9a, 0x09, 0x4f, 0xe3, 0xc4, 0x80, 0xd6, 0xa6, 0x47, 0x54, 0x98, 0x51, 0x32, 0x17, 0x5b,
	0x30, 0xf3, 0x97, 0x96, 0x00, 0x3f, 0x20, 0xbb, 0x98, 0x0a, 0x6e, 0x7a, 0xe3, 0x4b, 0x52, 0xdd,
	0x4e, 0x7e, 0x8c, 0x3c, 0xd5, 0x0b, 0xae, 0x75, 0x18, 0xf3, 0xcd, 0x21, 0xe3, 0x14, 0x74, 0xf8,
	0x0e, 0xb5, 0xbf, 0x23, 0x70, 0x6d, 0x30, 0x3b, 0x20, 0x65, 0x48, 0xa4, 0x0a, 0x89, 0x6c, 0x2a,
	0x06, 0x3d, 0x91, 0x31, 0x46, 0xf6, 0x36, 0x34, 0xa1, 0xdb, 0x18, 0x5a, 0xa3, 0x2e, 0x85, 0xda,
	0xbb, 0x41, 0xfd, 0xdf, 0x76, 0xe1, 0x2e, 0x6a, 0x05, 0x74, 0x15, 0xac, 0xd6, 0x8f, 0xcf, 0xce,
	0x1f, 0xfc, 0x0f, 0x75, 0xe6, 0xcb, 0xa7, 0x15, 0x5d, 0xcf, 0x17, 0xf3, 0xe5, 0xc6, 0xb1, 0xa6,
	0xb7, 0x2f, 0xd7, 0x71, 0x6a, 0x92, 0x3c, 0x22, 0x4c, 0xee, 0x7d, 0x38, 0x21, 0x53, 0xf2, 0x8d,
	0x33, 0x53, 0x82, 0x2b, 0x26, 0xd5, 0xf1, 0xcf, 0x62, 0x2e, 0xfc, 0xea, 0xc6, 0xa8, 0x09, 0xad,
	0xc9, 0x57, 0x00, 0x00, 0x00, 0xff, 0xff, 0x32, 0x2f, 0x1a, 0x66, 0xfd, 0x01, 0x00, 0x00,
}
