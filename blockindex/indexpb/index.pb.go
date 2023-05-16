// Copyright (c) 2019 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// To compile the proto, run:
//      protoc --go_out=plugins=grpc:. *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.17.3
// source: index.proto

package indexpb

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

type BlockIndex struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NumAction uint32 `protobuf:"varint,1,opt,name=numAction,proto3" json:"numAction,omitempty"`
	Hash      []byte `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	TsfAmount []byte `protobuf:"bytes,3,opt,name=tsfAmount,proto3" json:"tsfAmount,omitempty"`
}

func (x *BlockIndex) Reset() {
	*x = BlockIndex{}
	if protoimpl.UnsafeEnabled {
		mi := &file_index_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockIndex) ProtoMessage() {}

func (x *BlockIndex) ProtoReflect() protoreflect.Message {
	mi := &file_index_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockIndex.ProtoReflect.Descriptor instead.
func (*BlockIndex) Descriptor() ([]byte, []int) {
	return file_index_proto_rawDescGZIP(), []int{0}
}

func (x *BlockIndex) GetNumAction() uint32 {
	if x != nil {
		return x.NumAction
	}
	return 0
}

func (x *BlockIndex) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *BlockIndex) GetTsfAmount() []byte {
	if x != nil {
		return x.TsfAmount
	}
	return nil
}

type ActionIndex struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlkHeight uint64 `protobuf:"varint,1,opt,name=blkHeight,proto3" json:"blkHeight,omitempty"`
}

func (x *ActionIndex) Reset() {
	*x = ActionIndex{}
	if protoimpl.UnsafeEnabled {
		mi := &file_index_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ActionIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ActionIndex) ProtoMessage() {}

func (x *ActionIndex) ProtoReflect() protoreflect.Message {
	mi := &file_index_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ActionIndex.ProtoReflect.Descriptor instead.
func (*ActionIndex) Descriptor() ([]byte, []int) {
	return file_index_proto_rawDescGZIP(), []int{1}
}

func (x *ActionIndex) GetBlkHeight() uint64 {
	if x != nil {
		return x.BlkHeight
	}
	return 0
}

type SGDIndex struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Contract string `protobuf:"bytes,1,opt,name=contract,proto3" json:"contract,omitempty"`
	Receiver string `protobuf:"bytes,2,opt,name=receiver,proto3" json:"receiver,omitempty"`
	Approved bool   `protobuf:"varint,3,opt,name=approved,proto3" json:"approved,omitempty"`
}

func (x *SGDIndex) Reset() {
	*x = SGDIndex{}
	if protoimpl.UnsafeEnabled {
		mi := &file_index_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SGDIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SGDIndex) ProtoMessage() {}

func (x *SGDIndex) ProtoReflect() protoreflect.Message {
	mi := &file_index_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SGDIndex.ProtoReflect.Descriptor instead.
func (*SGDIndex) Descriptor() ([]byte, []int) {
	return file_index_proto_rawDescGZIP(), []int{2}
}

func (x *SGDIndex) GetContract() string {
	if x != nil {
		return x.Contract
	}
	return ""
}

func (x *SGDIndex) GetReceiver() string {
	if x != nil {
		return x.Receiver
	}
	return ""
}

func (x *SGDIndex) GetApproved() bool {
	if x != nil {
		return x.Approved
	}
	return false
}

var File_index_proto protoreflect.FileDescriptor

var file_index_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x69,
	0x6e, 0x64, 0x65, 0x78, 0x70, 0x62, 0x22, 0x5c, 0x0a, 0x0a, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x49,
	0x6e, 0x64, 0x65, 0x78, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x75, 0x6d, 0x41, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x09, 0x6e, 0x75, 0x6d, 0x41, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x73, 0x66, 0x41, 0x6d, 0x6f,
	0x75, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x74, 0x73, 0x66, 0x41, 0x6d,
	0x6f, 0x75, 0x6e, 0x74, 0x22, 0x2b, 0x0a, 0x0b, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x6e,
	0x64, 0x65, 0x78, 0x12, 0x1c, 0x0a, 0x09, 0x62, 0x6c, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x62, 0x6c, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68,
	0x74, 0x22, 0x5e, 0x0a, 0x08, 0x53, 0x47, 0x44, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x1a, 0x0a,
	0x08, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x63,
	0x65, 0x69, 0x76, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x63,
	0x65, 0x69, 0x76, 0x65, 0x72, 0x12, 0x1a, 0x0a, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65,
	0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65,
	0x64, 0x42, 0x0c, 0x5a, 0x0a, 0x2e, 0x2e, 0x2f, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x70, 0x62, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_index_proto_rawDescOnce sync.Once
	file_index_proto_rawDescData = file_index_proto_rawDesc
)

func file_index_proto_rawDescGZIP() []byte {
	file_index_proto_rawDescOnce.Do(func() {
		file_index_proto_rawDescData = protoimpl.X.CompressGZIP(file_index_proto_rawDescData)
	})
	return file_index_proto_rawDescData
}

var file_index_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_index_proto_goTypes = []interface{}{
	(*BlockIndex)(nil),  // 0: indexpb.BlockIndex
	(*ActionIndex)(nil), // 1: indexpb.ActionIndex
	(*SGDIndex)(nil),    // 2: indexpb.SGDIndex
}
var file_index_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_index_proto_init() }
func file_index_proto_init() {
	if File_index_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_index_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlockIndex); i {
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
		file_index_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ActionIndex); i {
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
		file_index_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SGDIndex); i {
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
			RawDescriptor: file_index_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_index_proto_goTypes,
		DependencyIndexes: file_index_proto_depIdxs,
		MessageInfos:      file_index_proto_msgTypes,
	}.Build()
	File_index_proto = out.File
	file_index_proto_rawDesc = nil
	file_index_proto_goTypes = nil
	file_index_proto_depIdxs = nil
}
