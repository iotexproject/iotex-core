// Code generated by protoc-gen-go. DO NOT EDIT.
// source: consensus.proto

package iotextypes // import "github.com/iotexproject/iotex-core/protogen/iotextypes"

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

type ConsensusVote_Topic int32

const (
	ConsensusVote_PROPOSAL ConsensusVote_Topic = 0
	ConsensusVote_LOCK     ConsensusVote_Topic = 1
	ConsensusVote_COMMIT   ConsensusVote_Topic = 2
)

var ConsensusVote_Topic_name = map[int32]string{
	0: "PROPOSAL",
	1: "LOCK",
	2: "COMMIT",
}
var ConsensusVote_Topic_value = map[string]int32{
	"PROPOSAL": 0,
	"LOCK":     1,
	"COMMIT":   2,
}

func (x ConsensusVote_Topic) String() string {
	return proto.EnumName(ConsensusVote_Topic_name, int32(x))
}
func (ConsensusVote_Topic) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_consensus_edaab123cdf8fe85, []int{1, 0}
}

type BlockProposal struct {
	Block                *Block         `protobuf:"bytes,1,opt,name=block,proto3" json:"block,omitempty"`
	Endorsements         []*Endorsement `protobuf:"bytes,2,rep,name=endorsements,proto3" json:"endorsements,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *BlockProposal) Reset()         { *m = BlockProposal{} }
func (m *BlockProposal) String() string { return proto.CompactTextString(m) }
func (*BlockProposal) ProtoMessage()    {}
func (*BlockProposal) Descriptor() ([]byte, []int) {
	return fileDescriptor_consensus_edaab123cdf8fe85, []int{0}
}
func (m *BlockProposal) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockProposal.Unmarshal(m, b)
}
func (m *BlockProposal) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockProposal.Marshal(b, m, deterministic)
}
func (dst *BlockProposal) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockProposal.Merge(dst, src)
}
func (m *BlockProposal) XXX_Size() int {
	return xxx_messageInfo_BlockProposal.Size(m)
}
func (m *BlockProposal) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockProposal.DiscardUnknown(m)
}

var xxx_messageInfo_BlockProposal proto.InternalMessageInfo

func (m *BlockProposal) GetBlock() *Block {
	if m != nil {
		return m.Block
	}
	return nil
}

func (m *BlockProposal) GetEndorsements() []*Endorsement {
	if m != nil {
		return m.Endorsements
	}
	return nil
}

type ConsensusVote struct {
	BlockHash            []byte              `protobuf:"bytes,1,opt,name=blockHash,proto3" json:"blockHash,omitempty"`
	Topic                ConsensusVote_Topic `protobuf:"varint,2,opt,name=topic,proto3,enum=iotextypes.ConsensusVote_Topic" json:"topic,omitempty"`
	XXX_NoUnkeyedLiteral struct{}            `json:"-"`
	XXX_unrecognized     []byte              `json:"-"`
	XXX_sizecache        int32               `json:"-"`
}

func (m *ConsensusVote) Reset()         { *m = ConsensusVote{} }
func (m *ConsensusVote) String() string { return proto.CompactTextString(m) }
func (*ConsensusVote) ProtoMessage()    {}
func (*ConsensusVote) Descriptor() ([]byte, []int) {
	return fileDescriptor_consensus_edaab123cdf8fe85, []int{1}
}
func (m *ConsensusVote) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConsensusVote.Unmarshal(m, b)
}
func (m *ConsensusVote) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConsensusVote.Marshal(b, m, deterministic)
}
func (dst *ConsensusVote) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConsensusVote.Merge(dst, src)
}
func (m *ConsensusVote) XXX_Size() int {
	return xxx_messageInfo_ConsensusVote.Size(m)
}
func (m *ConsensusVote) XXX_DiscardUnknown() {
	xxx_messageInfo_ConsensusVote.DiscardUnknown(m)
}

var xxx_messageInfo_ConsensusVote proto.InternalMessageInfo

func (m *ConsensusVote) GetBlockHash() []byte {
	if m != nil {
		return m.BlockHash
	}
	return nil
}

func (m *ConsensusVote) GetTopic() ConsensusVote_Topic {
	if m != nil {
		return m.Topic
	}
	return ConsensusVote_PROPOSAL
}

type ConsensusMessage struct {
	Height      uint64       `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	Endorsement *Endorsement `protobuf:"bytes,2,opt,name=endorsement,proto3" json:"endorsement,omitempty"`
	// Types that are valid to be assigned to Msg:
	//	*ConsensusMessage_BlockProposal
	//	*ConsensusMessage_Vote
	Msg                  isConsensusMessage_Msg `protobuf_oneof:"msg"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *ConsensusMessage) Reset()         { *m = ConsensusMessage{} }
func (m *ConsensusMessage) String() string { return proto.CompactTextString(m) }
func (*ConsensusMessage) ProtoMessage()    {}
func (*ConsensusMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_consensus_edaab123cdf8fe85, []int{2}
}
func (m *ConsensusMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ConsensusMessage.Unmarshal(m, b)
}
func (m *ConsensusMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ConsensusMessage.Marshal(b, m, deterministic)
}
func (dst *ConsensusMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConsensusMessage.Merge(dst, src)
}
func (m *ConsensusMessage) XXX_Size() int {
	return xxx_messageInfo_ConsensusMessage.Size(m)
}
func (m *ConsensusMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_ConsensusMessage.DiscardUnknown(m)
}

var xxx_messageInfo_ConsensusMessage proto.InternalMessageInfo

func (m *ConsensusMessage) GetHeight() uint64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *ConsensusMessage) GetEndorsement() *Endorsement {
	if m != nil {
		return m.Endorsement
	}
	return nil
}

type isConsensusMessage_Msg interface {
	isConsensusMessage_Msg()
}

type ConsensusMessage_BlockProposal struct {
	BlockProposal *BlockProposal `protobuf:"bytes,100,opt,name=blockProposal,proto3,oneof"`
}

type ConsensusMessage_Vote struct {
	Vote *ConsensusVote `protobuf:"bytes,101,opt,name=vote,proto3,oneof"`
}

func (*ConsensusMessage_BlockProposal) isConsensusMessage_Msg() {}

func (*ConsensusMessage_Vote) isConsensusMessage_Msg() {}

func (m *ConsensusMessage) GetMsg() isConsensusMessage_Msg {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *ConsensusMessage) GetBlockProposal() *BlockProposal {
	if x, ok := m.GetMsg().(*ConsensusMessage_BlockProposal); ok {
		return x.BlockProposal
	}
	return nil
}

func (m *ConsensusMessage) GetVote() *ConsensusVote {
	if x, ok := m.GetMsg().(*ConsensusMessage_Vote); ok {
		return x.Vote
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*ConsensusMessage) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _ConsensusMessage_OneofMarshaler, _ConsensusMessage_OneofUnmarshaler, _ConsensusMessage_OneofSizer, []interface{}{
		(*ConsensusMessage_BlockProposal)(nil),
		(*ConsensusMessage_Vote)(nil),
	}
}

func _ConsensusMessage_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*ConsensusMessage)
	// msg
	switch x := m.Msg.(type) {
	case *ConsensusMessage_BlockProposal:
		b.EncodeVarint(100<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.BlockProposal); err != nil {
			return err
		}
	case *ConsensusMessage_Vote:
		b.EncodeVarint(101<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Vote); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("ConsensusMessage.Msg has unexpected type %T", x)
	}
	return nil
}

func _ConsensusMessage_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*ConsensusMessage)
	switch tag {
	case 100: // msg.blockProposal
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(BlockProposal)
		err := b.DecodeMessage(msg)
		m.Msg = &ConsensusMessage_BlockProposal{msg}
		return true, err
	case 101: // msg.vote
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(ConsensusVote)
		err := b.DecodeMessage(msg)
		m.Msg = &ConsensusMessage_Vote{msg}
		return true, err
	default:
		return false, nil
	}
}

func _ConsensusMessage_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*ConsensusMessage)
	// msg
	switch x := m.Msg.(type) {
	case *ConsensusMessage_BlockProposal:
		s := proto.Size(x.BlockProposal)
		n += 2 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *ConsensusMessage_Vote:
		s := proto.Size(x.Vote)
		n += 2 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

func init() {
	proto.RegisterType((*BlockProposal)(nil), "iotextypes.BlockProposal")
	proto.RegisterType((*ConsensusVote)(nil), "iotextypes.ConsensusVote")
	proto.RegisterType((*ConsensusMessage)(nil), "iotextypes.ConsensusMessage")
	proto.RegisterEnum("iotextypes.ConsensusVote_Topic", ConsensusVote_Topic_name, ConsensusVote_Topic_value)
}

func init() { proto.RegisterFile("consensus.proto", fileDescriptor_consensus_edaab123cdf8fe85) }

var fileDescriptor_consensus_edaab123cdf8fe85 = []byte{
	// 361 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x92, 0x51, 0x4b, 0xc2, 0x50,
	0x14, 0xc7, 0x9d, 0x3a, 0xb1, 0xa3, 0xd6, 0xbc, 0x0f, 0xb5, 0x24, 0x48, 0xf6, 0x92, 0x10, 0x6d,
	0x60, 0x14, 0x45, 0x4f, 0x2a, 0x81, 0x91, 0x32, 0xb9, 0x49, 0x0f, 0xbd, 0x6d, 0xf3, 0xb0, 0xad,
	0x74, 0x77, 0xec, 0x5e, 0xa3, 0x3e, 0x46, 0xdf, 0xb0, 0x8f, 0x12, 0xde, 0x65, 0xdb, 0x10, 0x7a,
	0x3c, 0xe7, 0xfc, 0xfe, 0xff, 0x7b, 0xce, 0x9f, 0x0b, 0x07, 0x1e, 0x8b, 0x38, 0x46, 0x7c, 0xcd,
	0xcd, 0x38, 0x61, 0x82, 0x11, 0x08, 0x99, 0xc0, 0x0f, 0xf1, 0x19, 0x23, 0xef, 0x68, 0xee, 0x92,
	0x79, 0x6f, 0x5e, 0xe0, 0x84, 0x51, 0x3a, 0xed, 0xb4, 0x31, 0x5a, 0xb0, 0x84, 0xe3, 0x0a, 0x23,
	0x91, 0xb6, 0x8c, 0x35, 0xb4, 0x86, 0x1b, 0x6c, 0x96, 0xb0, 0x98, 0x71, 0x67, 0x49, 0xce, 0x40,
	0x95, 0x3a, 0x5d, 0xe9, 0x2a, 0xbd, 0x46, 0xbf, 0x6d, 0x66, 0x8e, 0xa6, 0x24, 0x69, 0x3a, 0x27,
	0x77, 0xd0, 0xcc, 0xd9, 0x71, 0xbd, 0xdc, 0xad, 0xf4, 0x1a, 0xfd, 0xa3, 0x3c, 0x7f, 0x9f, 0xcd,
	0x69, 0x01, 0x36, 0xbe, 0x14, 0x68, 0x8d, 0xb6, 0xbb, 0x3f, 0x33, 0x81, 0xe4, 0x04, 0xf6, 0xa4,
	0xef, 0xd8, 0xe1, 0x81, 0x7c, 0xbb, 0x49, 0xb3, 0x06, 0xb9, 0x02, 0x55, 0xb0, 0x38, 0xf4, 0xf4,
	0x72, 0x57, 0xe9, 0xed, 0xf7, 0x4f, 0xf3, 0xaf, 0x14, 0x7c, 0xcc, 0xf9, 0x06, 0xa3, 0x29, 0x6d,
	0x9c, 0x83, 0x2a, 0x6b, 0xd2, 0x84, 0xfa, 0x8c, 0xda, 0x33, 0xfb, 0x69, 0x30, 0xd1, 0x4a, 0xa4,
	0x0e, 0xd5, 0x89, 0x3d, 0x7a, 0xd4, 0x14, 0x02, 0x50, 0x1b, 0xd9, 0xd3, 0xe9, 0xc3, 0x5c, 0x2b,
	0x1b, 0xdf, 0x0a, 0x68, 0x7f, 0x5e, 0x53, 0xe4, 0xdc, 0xf1, 0x91, 0x1c, 0x42, 0x2d, 0xc0, 0xd0,
	0x0f, 0x84, 0xdc, 0xa9, 0x4a, 0x7f, 0x2b, 0x72, 0x0b, 0x8d, 0xdc, 0x41, 0x72, 0xad, 0x7f, 0x8e,
	0xcf, 0xb3, 0x64, 0x00, 0x2d, 0x37, 0x1f, 0xb9, 0xbe, 0x90, 0xe2, 0xe3, 0x9d, 0xa4, 0xb7, 0xc0,
	0xb8, 0x44, 0x8b, 0x0a, 0x62, 0x41, 0xf5, 0x9d, 0x09, 0xd4, 0x71, 0x57, 0x59, 0x48, 0x63, 0x5c,
	0xa2, 0x12, 0x1c, 0xaa, 0x50, 0x59, 0x71, 0x7f, 0x78, 0xf3, 0x72, 0xed, 0x87, 0x22, 0x58, 0xbb,
	0xa6, 0xc7, 0x56, 0x96, 0x54, 0xc5, 0x09, 0x7b, 0x45, 0x4f, 0xa4, 0xc5, 0x85, 0xc7, 0x12, 0xb4,
	0xe4, 0xcf, 0xf0, 0x31, 0xb2, 0x32, 0x5b, 0xb7, 0x26, 0x9b, 0x97, 0x3f, 0x01, 0x00, 0x00, 0xff,
	0xff, 0xe2, 0x37, 0x4d, 0xde, 0x72, 0x02, 0x00, 0x00,
}
