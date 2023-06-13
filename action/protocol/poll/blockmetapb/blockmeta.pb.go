// Copyright (c) 2020 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// To compile the proto, run:
//      protoc --go_out=plugins=grpc:. *.proto

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.23.0
// 	protoc        v3.12.4
// source: blockmeta.proto

package blockmetapb

import (
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
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

type BlockMeta struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlockHeight   uint64               `protobuf:"varint,1,opt,name=blockHeight,proto3" json:"blockHeight,omitempty"`
	BlockProducer string               `protobuf:"bytes,2,opt,name=blockProducer,proto3" json:"blockProducer,omitempty"`
	BlockTime     *timestamp.Timestamp `protobuf:"bytes,3,opt,name=blockTime,proto3" json:"blockTime,omitempty"`
}

func (x *BlockMeta) Reset() {
	*x = BlockMeta{}
	if protoimpl.UnsafeEnabled {
		mi := &file_blockmeta_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockMeta) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockMeta) ProtoMessage() {}

func (x *BlockMeta) ProtoReflect() protoreflect.Message {
	mi := &file_blockmeta_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockMeta.ProtoReflect.Descriptor instead.
func (*BlockMeta) Descriptor() ([]byte, []int) {
	return file_blockmeta_proto_rawDescGZIP(), []int{0}
}

func (x *BlockMeta) GetBlockHeight() uint64 {
	if x != nil {
		return x.BlockHeight
	}
	return 0
}

func (x *BlockMeta) GetBlockProducer() string {
	if x != nil {
		return x.BlockProducer
	}
	return ""
}

func (x *BlockMeta) GetBlockTime() *timestamp.Timestamp {
	if x != nil {
		return x.BlockTime
	}
	return nil
}

var File_blockmeta_proto protoreflect.FileDescriptor

var file_blockmeta_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x6d, 0x65, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x6d, 0x65, 0x74, 0x61, 0x70, 0x62, 0x1a, 0x1f,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x8d, 0x01, 0x0a, 0x09, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4d, 0x65, 0x74, 0x61, 0x12, 0x20, 0x0a,
	0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12,
	0x24, 0x0a, 0x0d, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x72, 0x6f,
	0x64, 0x75, 0x63, 0x65, 0x72, 0x12, 0x38, 0x0a, 0x09, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x54, 0x69,
	0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x54, 0x69, 0x6d, 0x65, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_blockmeta_proto_rawDescOnce sync.Once
	file_blockmeta_proto_rawDescData = file_blockmeta_proto_rawDesc
)

func file_blockmeta_proto_rawDescGZIP() []byte {
	file_blockmeta_proto_rawDescOnce.Do(func() {
		file_blockmeta_proto_rawDescData = protoimpl.X.CompressGZIP(file_blockmeta_proto_rawDescData)
	})
	return file_blockmeta_proto_rawDescData
}

var file_blockmeta_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_blockmeta_proto_goTypes = []interface{}{
	(*BlockMeta)(nil),           // 0: blockmetapb.BlockMeta
	(*timestamp.Timestamp)(nil), // 1: google.protobuf.Timestamp
}
var file_blockmeta_proto_depIdxs = []int32{
	1, // 0: blockmetapb.BlockMeta.blockTime:type_name -> google.protobuf.Timestamp
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_blockmeta_proto_init() }
func file_blockmeta_proto_init() {
	if File_blockmeta_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_blockmeta_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlockMeta); i {
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
			RawDescriptor: file_blockmeta_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_blockmeta_proto_goTypes,
		DependencyIndexes: file_blockmeta_proto_depIdxs,
		MessageInfos:      file_blockmeta_proto_msgTypes,
	}.Build()
	File_blockmeta_proto = out.File
	file_blockmeta_proto_rawDesc = nil
	file_blockmeta_proto_goTypes = nil
	file_blockmeta_proto_depIdxs = nil
}
