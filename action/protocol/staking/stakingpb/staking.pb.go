// Code generated by protoc-gen-go. DO NOT EDIT.
// source: staking.proto

package stakingpb

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
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

type Bucket struct {
	CandidateAddress     string               `protobuf:"bytes,1,opt,name=candidateAddress,proto3" json:"candidateAddress,omitempty"`
	StakedAmount         string               `protobuf:"bytes,2,opt,name=stakedAmount,proto3" json:"stakedAmount,omitempty"`
	StakedDuration       uint32               `protobuf:"varint,3,opt,name=stakedDuration,proto3" json:"stakedDuration,omitempty"`
	CreateTime           *timestamp.Timestamp `protobuf:"bytes,4,opt,name=createTime,proto3" json:"createTime,omitempty"`
	StakeStartTime       *timestamp.Timestamp `protobuf:"bytes,5,opt,name=stakeStartTime,proto3" json:"stakeStartTime,omitempty"`
	UnstakeStartTime     *timestamp.Timestamp `protobuf:"bytes,6,opt,name=unstakeStartTime,proto3" json:"unstakeStartTime,omitempty"`
	AutoStake            bool                 `protobuf:"varint,7,opt,name=autoStake,proto3" json:"autoStake,omitempty"`
	Owner                string               `protobuf:"bytes,8,opt,name=owner,proto3" json:"owner,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *Bucket) Reset()         { *m = Bucket{} }
func (m *Bucket) String() string { return proto.CompactTextString(m) }
func (*Bucket) ProtoMessage()    {}
func (*Bucket) Descriptor() ([]byte, []int) {
	return fileDescriptor_289e7c8aea278311, []int{0}
}

func (m *Bucket) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Bucket.Unmarshal(m, b)
}
func (m *Bucket) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Bucket.Marshal(b, m, deterministic)
}
func (m *Bucket) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Bucket.Merge(m, src)
}
func (m *Bucket) XXX_Size() int {
	return xxx_messageInfo_Bucket.Size(m)
}
func (m *Bucket) XXX_DiscardUnknown() {
	xxx_messageInfo_Bucket.DiscardUnknown(m)
}

var xxx_messageInfo_Bucket proto.InternalMessageInfo

func (m *Bucket) GetCandidateAddress() string {
	if m != nil {
		return m.CandidateAddress
	}
	return ""
}

func (m *Bucket) GetStakedAmount() string {
	if m != nil {
		return m.StakedAmount
	}
	return ""
}

func (m *Bucket) GetStakedDuration() uint32 {
	if m != nil {
		return m.StakedDuration
	}
	return 0
}

func (m *Bucket) GetCreateTime() *timestamp.Timestamp {
	if m != nil {
		return m.CreateTime
	}
	return nil
}

func (m *Bucket) GetStakeStartTime() *timestamp.Timestamp {
	if m != nil {
		return m.StakeStartTime
	}
	return nil
}

func (m *Bucket) GetUnstakeStartTime() *timestamp.Timestamp {
	if m != nil {
		return m.UnstakeStartTime
	}
	return nil
}

func (m *Bucket) GetAutoStake() bool {
	if m != nil {
		return m.AutoStake
	}
	return false
}

func (m *Bucket) GetOwner() string {
	if m != nil {
		return m.Owner
	}
	return ""
}

type BucketIndex struct {
	Index                uint64   `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	CandAddress          string   `protobuf:"bytes,2,opt,name=candAddress,proto3" json:"candAddress,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BucketIndex) Reset()         { *m = BucketIndex{} }
func (m *BucketIndex) String() string { return proto.CompactTextString(m) }
func (*BucketIndex) ProtoMessage()    {}
func (*BucketIndex) Descriptor() ([]byte, []int) {
	return fileDescriptor_289e7c8aea278311, []int{1}
}

func (m *BucketIndex) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BucketIndex.Unmarshal(m, b)
}
func (m *BucketIndex) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BucketIndex.Marshal(b, m, deterministic)
}
func (m *BucketIndex) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BucketIndex.Merge(m, src)
}
func (m *BucketIndex) XXX_Size() int {
	return xxx_messageInfo_BucketIndex.Size(m)
}
func (m *BucketIndex) XXX_DiscardUnknown() {
	xxx_messageInfo_BucketIndex.DiscardUnknown(m)
}

var xxx_messageInfo_BucketIndex proto.InternalMessageInfo

func (m *BucketIndex) GetIndex() uint64 {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *BucketIndex) GetCandAddress() string {
	if m != nil {
		return m.CandAddress
	}
	return ""
}

type BucketIndices struct {
	Indices              []*BucketIndex `protobuf:"bytes,1,rep,name=indices,proto3" json:"indices,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *BucketIndices) Reset()         { *m = BucketIndices{} }
func (m *BucketIndices) String() string { return proto.CompactTextString(m) }
func (*BucketIndices) ProtoMessage()    {}
func (*BucketIndices) Descriptor() ([]byte, []int) {
	return fileDescriptor_289e7c8aea278311, []int{2}
}

func (m *BucketIndices) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BucketIndices.Unmarshal(m, b)
}
func (m *BucketIndices) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BucketIndices.Marshal(b, m, deterministic)
}
func (m *BucketIndices) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BucketIndices.Merge(m, src)
}
func (m *BucketIndices) XXX_Size() int {
	return xxx_messageInfo_BucketIndices.Size(m)
}
func (m *BucketIndices) XXX_DiscardUnknown() {
	xxx_messageInfo_BucketIndices.DiscardUnknown(m)
}

var xxx_messageInfo_BucketIndices proto.InternalMessageInfo

func (m *BucketIndices) GetIndices() []*BucketIndex {
	if m != nil {
		return m.Indices
	}
	return nil
}

type Candidate struct {
	OwnerAddress         string   `protobuf:"bytes,1,opt,name=ownerAddress,proto3" json:"ownerAddress,omitempty"`
	OperatorAddress      string   `protobuf:"bytes,2,opt,name=operatorAddress,proto3" json:"operatorAddress,omitempty"`
	RewardAddress        string   `protobuf:"bytes,3,opt,name=rewardAddress,proto3" json:"rewardAddress,omitempty"`
	Name                 []byte   `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	Votes                []byte   `protobuf:"bytes,5,opt,name=votes,proto3" json:"votes,omitempty"`
	SelfStake            []byte   `protobuf:"bytes,6,opt,name=selfStake,proto3" json:"selfStake,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Candidate) Reset()         { *m = Candidate{} }
func (m *Candidate) String() string { return proto.CompactTextString(m) }
func (*Candidate) ProtoMessage()    {}
func (*Candidate) Descriptor() ([]byte, []int) {
	return fileDescriptor_289e7c8aea278311, []int{3}
}

func (m *Candidate) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Candidate.Unmarshal(m, b)
}
func (m *Candidate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Candidate.Marshal(b, m, deterministic)
}
func (m *Candidate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Candidate.Merge(m, src)
}
func (m *Candidate) XXX_Size() int {
	return xxx_messageInfo_Candidate.Size(m)
}
func (m *Candidate) XXX_DiscardUnknown() {
	xxx_messageInfo_Candidate.DiscardUnknown(m)
}

var xxx_messageInfo_Candidate proto.InternalMessageInfo

func (m *Candidate) GetOwnerAddress() string {
	if m != nil {
		return m.OwnerAddress
	}
	return ""
}

func (m *Candidate) GetOperatorAddress() string {
	if m != nil {
		return m.OperatorAddress
	}
	return ""
}

func (m *Candidate) GetRewardAddress() string {
	if m != nil {
		return m.RewardAddress
	}
	return ""
}

func (m *Candidate) GetName() []byte {
	if m != nil {
		return m.Name
	}
	return nil
}

func (m *Candidate) GetVotes() []byte {
	if m != nil {
		return m.Votes
	}
	return nil
}

func (m *Candidate) GetSelfStake() []byte {
	if m != nil {
		return m.SelfStake
	}
	return nil
}

type Candidates struct {
	Candidates           []*Candidate `protobuf:"bytes,1,rep,name=candidates,proto3" json:"candidates,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Candidates) Reset()         { *m = Candidates{} }
func (m *Candidates) String() string { return proto.CompactTextString(m) }
func (*Candidates) ProtoMessage()    {}
func (*Candidates) Descriptor() ([]byte, []int) {
	return fileDescriptor_289e7c8aea278311, []int{4}
}

func (m *Candidates) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Candidates.Unmarshal(m, b)
}
func (m *Candidates) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Candidates.Marshal(b, m, deterministic)
}
func (m *Candidates) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Candidates.Merge(m, src)
}
func (m *Candidates) XXX_Size() int {
	return xxx_messageInfo_Candidates.Size(m)
}
func (m *Candidates) XXX_DiscardUnknown() {
	xxx_messageInfo_Candidates.DiscardUnknown(m)
}

var xxx_messageInfo_Candidates proto.InternalMessageInfo

func (m *Candidates) GetCandidates() []*Candidate {
	if m != nil {
		return m.Candidates
	}
	return nil
}

func init() {
	proto.RegisterType((*Bucket)(nil), "stakingpb.Bucket")
	proto.RegisterType((*BucketIndex)(nil), "stakingpb.BucketIndex")
	proto.RegisterType((*BucketIndices)(nil), "stakingpb.BucketIndices")
	proto.RegisterType((*Candidate)(nil), "stakingpb.Candidate")
	proto.RegisterType((*Candidates)(nil), "stakingpb.Candidates")
}

func init() { proto.RegisterFile("staking.proto", fileDescriptor_289e7c8aea278311) }

var fileDescriptor_289e7c8aea278311 = []byte{
	// 410 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x52, 0x4d, 0x6b, 0xdc, 0x30,
	0x10, 0xc5, 0xd9, 0xcd, 0x26, 0x3b, 0xbb, 0x6e, 0xc3, 0x10, 0x8a, 0x09, 0x85, 0x1a, 0x53, 0x8a,
	0xe9, 0xc1, 0x29, 0x69, 0x4f, 0xbd, 0xed, 0xf6, 0x03, 0x7a, 0x55, 0xf2, 0x07, 0xb4, 0xf6, 0x64,
	0x11, 0x89, 0x25, 0x23, 0xc9, 0x4d, 0xff, 0x61, 0xff, 0x54, 0x0f, 0x45, 0xd2, 0xda, 0xb1, 0x9d,
	0xc3, 0xde, 0x66, 0x9e, 0xde, 0x7c, 0xe8, 0xbd, 0x81, 0xd8, 0x58, 0xfe, 0x20, 0xe4, 0xbe, 0x68,
	0xb4, 0xb2, 0x0a, 0x97, 0x87, 0xb4, 0xd9, 0x5d, 0xbd, 0xdb, 0x2b, 0xb5, 0x7f, 0xa4, 0x6b, 0xff,
	0xb0, 0x6b, 0xef, 0xaf, 0xad, 0xa8, 0xc9, 0x58, 0x5e, 0x37, 0x81, 0x9b, 0xfd, 0x3b, 0x81, 0xc5,
	0xb6, 0x2d, 0x1f, 0xc8, 0xe2, 0x47, 0xb8, 0x28, 0xb9, 0xac, 0x44, 0xc5, 0x2d, 0x6d, 0xaa, 0x4a,
	0x93, 0x31, 0x49, 0x94, 0x46, 0xf9, 0x92, 0xbd, 0xc0, 0x31, 0x83, 0xb5, 0x1b, 0x42, 0xd5, 0xa6,
	0x56, 0xad, 0xb4, 0xc9, 0x89, 0xe7, 0x8d, 0x30, 0xfc, 0x00, 0xaf, 0x42, 0xfe, 0xbd, 0xd5, 0xdc,
	0x0a, 0x25, 0x93, 0x59, 0x1a, 0xe5, 0x31, 0x9b, 0xa0, 0xf8, 0x15, 0xa0, 0xd4, 0xc4, 0x2d, 0xdd,
	0x89, 0x9a, 0x92, 0x79, 0x1a, 0xe5, 0xab, 0x9b, 0xab, 0x22, 0x2c, 0x5e, 0x74, 0x8b, 0x17, 0x77,
	0xdd, 0xe2, 0x6c, 0xc0, 0xc6, 0xed, 0x61, 0xc6, 0xad, 0xe5, 0xda, 0xfa, 0xfa, 0xd3, 0xa3, 0xf5,
	0x93, 0x0a, 0xfc, 0x09, 0x17, 0xad, 0x9c, 0x74, 0x59, 0x1c, 0xed, 0xf2, 0xa2, 0x06, 0xdf, 0xc2,
	0x92, 0xb7, 0x56, 0xdd, 0x3a, 0x34, 0x39, 0x4b, 0xa3, 0xfc, 0x9c, 0x3d, 0x03, 0x78, 0x09, 0xa7,
	0xea, 0x49, 0x92, 0x4e, 0xce, 0xbd, 0x54, 0x21, 0xc9, 0x7e, 0xc0, 0x2a, 0xa8, 0xff, 0x4b, 0x56,
	0xf4, 0xc7, 0x91, 0x84, 0x0b, 0xbc, 0xee, 0x73, 0x16, 0x12, 0x4c, 0x61, 0xe5, 0x0c, 0xe8, 0x3c,
	0x09, 0x5a, 0x0f, 0xa1, 0x6c, 0x03, 0x71, 0xdf, 0x46, 0x94, 0x64, 0xf0, 0x13, 0x9c, 0x89, 0x10,
	0x26, 0x51, 0x3a, 0xcb, 0x57, 0x37, 0x6f, 0x8a, 0xfe, 0x28, 0x8a, 0xc1, 0x44, 0xd6, 0xd1, 0xb2,
	0xbf, 0x11, 0x2c, 0xbf, 0x75, 0x36, 0x3b, 0x7f, 0xfd, 0x82, 0xe3, 0x3b, 0x18, 0x61, 0x98, 0xc3,
	0x6b, 0xd5, 0x90, 0xe6, 0x56, 0xe9, 0xf1, 0x6a, 0x53, 0x18, 0xdf, 0x43, 0xac, 0xe9, 0x89, 0xeb,
	0xfe, 0x0b, 0x33, 0xcf, 0x1b, 0x83, 0x88, 0x30, 0x97, 0xfc, 0x70, 0x01, 0x6b, 0xe6, 0x63, 0x27,
	0xc8, 0x6f, 0x65, 0xc9, 0x78, 0x5b, 0xd7, 0x2c, 0x24, 0x4e, 0x69, 0x43, 0x8f, 0xf7, 0x41, 0xe9,
	0x85, 0x7f, 0x79, 0x06, 0xb2, 0x2d, 0x40, 0xff, 0x11, 0x83, 0x5f, 0x00, 0xfa, 0xeb, 0xed, 0xc4,
	0xb8, 0x1c, 0x88, 0xd1, 0x53, 0xd9, 0x80, 0xb7, 0x5b, 0x78, 0xc7, 0x3f, 0xff, 0x0f, 0x00, 0x00,
	0xff, 0xff, 0x0d, 0xfe, 0x91, 0x5b, 0x5a, 0x03, 0x00, 0x00,
}
