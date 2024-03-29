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
// source: execution.proto

package executionpb

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

type EvmTransfer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Amount string `protobuf:"bytes,1,opt,name=amount,proto3" json:"amount,omitempty"`
	From   string `protobuf:"bytes,2,opt,name=from,proto3" json:"from,omitempty"`
	To     string `protobuf:"bytes,3,opt,name=to,proto3" json:"to,omitempty"`
}

func (x *EvmTransfer) Reset() {
	*x = EvmTransfer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_execution_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EvmTransfer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EvmTransfer) ProtoMessage() {}

func (x *EvmTransfer) ProtoReflect() protoreflect.Message {
	mi := &file_execution_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EvmTransfer.ProtoReflect.Descriptor instead.
func (*EvmTransfer) Descriptor() ([]byte, []int) {
	return file_execution_proto_rawDescGZIP(), []int{0}
}

func (x *EvmTransfer) GetAmount() string {
	if x != nil {
		return x.Amount
	}
	return ""
}

func (x *EvmTransfer) GetFrom() string {
	if x != nil {
		return x.From
	}
	return ""
}

func (x *EvmTransfer) GetTo() string {
	if x != nil {
		return x.To
	}
	return ""
}

type EvmTransferList struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EvmTransfers []*EvmTransfer `protobuf:"bytes,1,rep,name=evmTransfers,proto3" json:"evmTransfers,omitempty"`
}

func (x *EvmTransferList) Reset() {
	*x = EvmTransferList{}
	if protoimpl.UnsafeEnabled {
		mi := &file_execution_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EvmTransferList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EvmTransferList) ProtoMessage() {}

func (x *EvmTransferList) ProtoReflect() protoreflect.Message {
	mi := &file_execution_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EvmTransferList.ProtoReflect.Descriptor instead.
func (*EvmTransferList) Descriptor() ([]byte, []int) {
	return file_execution_proto_rawDescGZIP(), []int{1}
}

func (x *EvmTransferList) GetEvmTransfers() []*EvmTransfer {
	if x != nil {
		return x.EvmTransfers
	}
	return nil
}

var File_execution_proto protoreflect.FileDescriptor

var file_execution_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x0b, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x70, 0x62, 0x22, 0x49,
	0x0a, 0x0b, 0x45, 0x76, 0x6d, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x12, 0x16, 0x0a,
	0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x61,
	0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x12, 0x0e, 0x0a, 0x02, 0x74, 0x6f, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x74, 0x6f, 0x22, 0x4f, 0x0a, 0x0f, 0x45, 0x76, 0x6d,
	0x54, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x3c, 0x0a, 0x0c,
	0x65, 0x76, 0x6d, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x18, 0x2e, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x70, 0x62,
	0x2e, 0x45, 0x76, 0x6d, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x52, 0x0c, 0x65, 0x76,
	0x6d, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_execution_proto_rawDescOnce sync.Once
	file_execution_proto_rawDescData = file_execution_proto_rawDesc
)

func file_execution_proto_rawDescGZIP() []byte {
	file_execution_proto_rawDescOnce.Do(func() {
		file_execution_proto_rawDescData = protoimpl.X.CompressGZIP(file_execution_proto_rawDescData)
	})
	return file_execution_proto_rawDescData
}

var file_execution_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_execution_proto_goTypes = []interface{}{
	(*EvmTransfer)(nil),     // 0: executionpb.EvmTransfer
	(*EvmTransferList)(nil), // 1: executionpb.EvmTransferList
}
var file_execution_proto_depIdxs = []int32{
	0, // 0: executionpb.EvmTransferList.evmTransfers:type_name -> executionpb.EvmTransfer
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_execution_proto_init() }
func file_execution_proto_init() {
	if File_execution_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_execution_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EvmTransfer); i {
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
		file_execution_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EvmTransferList); i {
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
			RawDescriptor: file_execution_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_execution_proto_goTypes,
		DependencyIndexes: file_execution_proto_depIdxs,
		MessageInfos:      file_execution_proto_msgTypes,
	}.Build()
	File_execution_proto = out.File
	file_execution_proto_rawDesc = nil
	file_execution_proto_goTypes = nil
	file_execution_proto_depIdxs = nil
}
