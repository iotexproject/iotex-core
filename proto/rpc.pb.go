// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

package iproto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type CreateRawTransferRequest struct {
	Sender    string `protobuf:"bytes,1,opt,name=sender" json:"sender,omitempty"`
	Recipient string `protobuf:"bytes,2,opt,name=recipient" json:"recipient,omitempty"`
	Amount    []byte `protobuf:"bytes,3,opt,name=amount,proto3" json:"amount,omitempty"`
	Nonce     uint64 `protobuf:"varint,4,opt,name=nonce" json:"nonce,omitempty"`
	Data      []byte `protobuf:"bytes,5,opt,name=data,proto3" json:"data,omitempty"`
}

func (m *CreateRawTransferRequest) Reset()                    { *m = CreateRawTransferRequest{} }
func (m *CreateRawTransferRequest) String() string            { return proto.CompactTextString(m) }
func (*CreateRawTransferRequest) ProtoMessage()               {}
func (*CreateRawTransferRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

func (m *CreateRawTransferRequest) GetSender() string {
	if m != nil {
		return m.Sender
	}
	return ""
}

func (m *CreateRawTransferRequest) GetRecipient() string {
	if m != nil {
		return m.Recipient
	}
	return ""
}

func (m *CreateRawTransferRequest) GetAmount() []byte {
	if m != nil {
		return m.Amount
	}
	return nil
}

func (m *CreateRawTransferRequest) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *CreateRawTransferRequest) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type CreateRawTransferResponse struct {
	SerializedTransfer []byte `protobuf:"bytes,1,opt,name=serialized_transfer,json=serializedTransfer,proto3" json:"serialized_transfer,omitempty"`
}

func (m *CreateRawTransferResponse) Reset()                    { *m = CreateRawTransferResponse{} }
func (m *CreateRawTransferResponse) String() string            { return proto.CompactTextString(m) }
func (*CreateRawTransferResponse) ProtoMessage()               {}
func (*CreateRawTransferResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{1} }

func (m *CreateRawTransferResponse) GetSerializedTransfer() []byte {
	if m != nil {
		return m.SerializedTransfer
	}
	return nil
}

type SendTransferRequest struct {
	SerializedTransfer []byte `protobuf:"bytes,1,opt,name=serialized_transfer,json=serializedTransfer,proto3" json:"serialized_transfer,omitempty"`
}

func (m *SendTransferRequest) Reset()                    { *m = SendTransferRequest{} }
func (m *SendTransferRequest) String() string            { return proto.CompactTextString(m) }
func (*SendTransferRequest) ProtoMessage()               {}
func (*SendTransferRequest) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{2} }

func (m *SendTransferRequest) GetSerializedTransfer() []byte {
	if m != nil {
		return m.SerializedTransfer
	}
	return nil
}

type SendTransferResponse struct {
}

func (m *SendTransferResponse) Reset()                    { *m = SendTransferResponse{} }
func (m *SendTransferResponse) String() string            { return proto.CompactTextString(m) }
func (*SendTransferResponse) ProtoMessage()               {}
func (*SendTransferResponse) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{3} }

func init() {
	proto.RegisterType((*CreateRawTransferRequest)(nil), "iproto.CreateRawTransferRequest")
	proto.RegisterType((*CreateRawTransferResponse)(nil), "iproto.CreateRawTransferResponse")
	proto.RegisterType((*SendTransferRequest)(nil), "iproto.SendTransferRequest")
	proto.RegisterType((*SendTransferResponse)(nil), "iproto.SendTransferResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for ChainService service

type ChainServiceClient interface {
	CreateRawTx(ctx context.Context, in *CreateRawTransferRequest, opts ...grpc.CallOption) (*CreateRawTransferResponse, error)
	SendTx(ctx context.Context, in *SendTransferRequest, opts ...grpc.CallOption) (*SendTransferResponse, error)
}

type chainServiceClient struct {
	cc *grpc.ClientConn
}

func NewChainServiceClient(cc *grpc.ClientConn) ChainServiceClient {
	return &chainServiceClient{cc}
}

func (c *chainServiceClient) CreateRawTx(ctx context.Context, in *CreateRawTransferRequest, opts ...grpc.CallOption) (*CreateRawTransferResponse, error) {
	out := new(CreateRawTransferResponse)
	err := grpc.Invoke(ctx, "/iproto.ChainService/CreateRawTx", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chainServiceClient) SendTx(ctx context.Context, in *SendTransferRequest, opts ...grpc.CallOption) (*SendTransferResponse, error) {
	out := new(SendTransferResponse)
	err := grpc.Invoke(ctx, "/iproto.ChainService/SendTx", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for ChainService service

type ChainServiceServer interface {
	CreateRawTx(context.Context, *CreateRawTransferRequest) (*CreateRawTransferResponse, error)
	SendTx(context.Context, *SendTransferRequest) (*SendTransferResponse, error)
}

func RegisterChainServiceServer(s *grpc.Server, srv ChainServiceServer) {
	s.RegisterService(&_ChainService_serviceDesc, srv)
}

func _ChainService_CreateRawTx_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateRawTransferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChainServiceServer).CreateRawTx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/iproto.ChainService/CreateRawTx",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChainServiceServer).CreateRawTx(ctx, req.(*CreateRawTransferRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ChainService_SendTx_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendTransferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChainServiceServer).SendTx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/iproto.ChainService/SendTx",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChainServiceServer).SendTx(ctx, req.(*SendTransferRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _ChainService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "iproto.ChainService",
	HandlerType: (*ChainServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateRawTx",
			Handler:    _ChainService_CreateRawTx_Handler,
		},
		{
			MethodName: "SendTx",
			Handler:    _ChainService_SendTx_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "rpc.proto",
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor1) }

var fileDescriptor1 = []byte{
	// 269 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x50, 0x41, 0x4e, 0xc3, 0x30,
	0x10, 0xc4, 0x90, 0x46, 0xca, 0x92, 0xd3, 0xb6, 0xaa, 0x4c, 0xe9, 0x21, 0xe4, 0x94, 0x53, 0x90,
	0xe0, 0x09, 0x15, 0x9c, 0x38, 0xb9, 0xbd, 0x23, 0x93, 0x2c, 0xc2, 0x12, 0xd8, 0xc1, 0x76, 0x21,
	0xe2, 0x19, 0x7c, 0x83, 0x4f, 0x22, 0x9c, 0x44, 0x11, 0x28, 0x20, 0x71, 0xb2, 0x67, 0x67, 0x76,
	0x76, 0x76, 0x21, 0xb1, 0x4d, 0x55, 0x36, 0xd6, 0x78, 0x83, 0xb1, 0x0a, 0x6f, 0xfe, 0xce, 0x80,
	0x6f, 0x2c, 0x49, 0x4f, 0x42, 0xbe, 0xee, 0xac, 0xd4, 0xee, 0x9e, 0xac, 0xa0, 0xe7, 0x3d, 0x39,
	0x8f, 0x4b, 0x88, 0x1d, 0xe9, 0x9a, 0x2c, 0x67, 0x19, 0x2b, 0x12, 0xd1, 0x23, 0x5c, 0x43, 0x62,
	0xa9, 0x52, 0x8d, 0x22, 0xed, 0xf9, 0x61, 0xa0, 0xc6, 0xc2, 0x57, 0x97, 0x7c, 0x32, 0x7b, 0xed,
	0xf9, 0x51, 0xc6, 0x8a, 0x54, 0xf4, 0x08, 0x17, 0x30, 0xd3, 0x46, 0x57, 0xc4, 0xa3, 0x8c, 0x15,
	0x91, 0xe8, 0x00, 0x22, 0x44, 0xb5, 0xf4, 0x92, 0xcf, 0x82, 0x36, 0xfc, 0xf3, 0x1b, 0x38, 0x99,
	0xc8, 0xe4, 0x1a, 0xa3, 0x1d, 0xe1, 0x39, 0xcc, 0x1d, 0x59, 0x25, 0x1f, 0xd5, 0x1b, 0xd5, 0xb7,
	0xbe, 0xa7, 0x43, 0xc2, 0x54, 0xe0, 0x48, 0x0d, 0x8d, 0xf9, 0x35, 0xcc, 0xb7, 0xa4, 0xeb, 0x9f,
	0xcb, 0xfd, 0xdb, 0x67, 0x09, 0x8b, 0xef, 0x3e, 0x5d, 0xa0, 0x8b, 0x0f, 0x06, 0xe9, 0xe6, 0x41,
	0x2a, 0xbd, 0x25, 0xfb, 0xa2, 0x2a, 0xc2, 0x1d, 0x1c, 0x8f, 0xf1, 0x5b, 0xcc, 0xca, 0xee, 0xd6,
	0xe5, 0x6f, 0x77, 0x5e, 0x9d, 0xfd, 0xa1, 0xe8, 0x86, 0xe4, 0x07, 0x78, 0x05, 0x71, 0x18, 0xdf,
	0xe2, 0xe9, 0x20, 0x9f, 0x58, 0x6b, 0xb5, 0x9e, 0x26, 0x07, 0x9b, 0xbb, 0x38, 0xb0, 0x97, 0x9f,
	0x01, 0x00, 0x00, 0xff, 0xff, 0xfa, 0x22, 0xea, 0x5a, 0x0c, 0x02, 0x00, 0x00,
}
