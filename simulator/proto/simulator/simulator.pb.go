// Code generated by protoc-gen-go. DO NOT EDIT.
// source: simulator.proto

package simulator

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

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// The request message containing the playerID and the value of the message being fed into the consensus engine
type Request struct {
	PlayerID             int32    `protobuf:"varint,1,opt,name=playerID" json:"playerID,omitempty"`
	Value                string   `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Request) Reset()         { *m = Request{} }
func (m *Request) String() string { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()    {}
func (*Request) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_1f083e41507ba06d, []int{0}
}
func (m *Request) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Request.Unmarshal(m, b)
}
func (m *Request) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Request.Marshal(b, m, deterministic)
}
func (dst *Request) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Request.Merge(dst, src)
}
func (m *Request) XXX_Size() int {
	return xxx_messageInfo_Request.Size(m)
}
func (m *Request) XXX_DiscardUnknown() {
	xxx_messageInfo_Request.DiscardUnknown(m)
}

var xxx_messageInfo_Request proto.InternalMessageInfo

func (m *Request) GetPlayerID() int32 {
	if m != nil {
		return m.PlayerID
	}
	return 0
}

func (m *Request) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

// The request message telling the server to initialize the necessary parameters
type InitRequest struct {
	NPlayers             int32    `protobuf:"varint,1,opt,name=nPlayers" json:"nPlayers,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *InitRequest) Reset()         { *m = InitRequest{} }
func (m *InitRequest) String() string { return proto.CompactTextString(m) }
func (*InitRequest) ProtoMessage()    {}
func (*InitRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_1f083e41507ba06d, []int{1}
}
func (m *InitRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_InitRequest.Unmarshal(m, b)
}
func (m *InitRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_InitRequest.Marshal(b, m, deterministic)
}
func (dst *InitRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InitRequest.Merge(dst, src)
}
func (m *InitRequest) XXX_Size() int {
	return xxx_messageInfo_InitRequest.Size(m)
}
func (m *InitRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_InitRequest.DiscardUnknown(m)
}

var xxx_messageInfo_InitRequest proto.InternalMessageInfo

func (m *InitRequest) GetNPlayers() int32 {
	if m != nil {
		return m.NPlayers
	}
	return 0
}

// The response message returning the output of the consensus engine
type Reply struct {
	MessageType          int32    `protobuf:"varint,1,opt,name=messageType" json:"messageType,omitempty"`
	Value                string   `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Reply) Reset()         { *m = Reply{} }
func (m *Reply) String() string { return proto.CompactTextString(m) }
func (*Reply) ProtoMessage()    {}
func (*Reply) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_1f083e41507ba06d, []int{2}
}
func (m *Reply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Reply.Unmarshal(m, b)
}
func (m *Reply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Reply.Marshal(b, m, deterministic)
}
func (dst *Reply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Reply.Merge(dst, src)
}
func (m *Reply) XXX_Size() int {
	return xxx_messageInfo_Reply.Size(m)
}
func (m *Reply) XXX_DiscardUnknown() {
	xxx_messageInfo_Reply.DiscardUnknown(m)
}

var xxx_messageInfo_Reply proto.InternalMessageInfo

func (m *Reply) GetMessageType() int32 {
	if m != nil {
		return m.MessageType
	}
	return 0
}

func (m *Reply) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

// an empty message
type Empty struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Empty) Reset()         { *m = Empty{} }
func (m *Empty) String() string { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()    {}
func (*Empty) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_1f083e41507ba06d, []int{3}
}
func (m *Empty) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Empty.Unmarshal(m, b)
}
func (m *Empty) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Empty.Marshal(b, m, deterministic)
}
func (dst *Empty) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Empty.Merge(dst, src)
}
func (m *Empty) XXX_Size() int {
	return xxx_messageInfo_Empty.Size(m)
}
func (m *Empty) XXX_DiscardUnknown() {
	xxx_messageInfo_Empty.DiscardUnknown(m)
}

var xxx_messageInfo_Empty proto.InternalMessageInfo

func init() {
	proto.RegisterType((*Request)(nil), "simulator.Request")
	proto.RegisterType((*InitRequest)(nil), "simulator.InitRequest")
	proto.RegisterType((*Reply)(nil), "simulator.Reply")
	proto.RegisterType((*Empty)(nil), "simulator.Empty")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// SimulatorClient is the client API for Simulator service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type SimulatorClient interface {
	Ping(ctx context.Context, in *Request, opts ...grpc.CallOption) (Simulator_PingClient, error)
	Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*Empty, error)
}

type simulatorClient struct {
	cc *grpc.ClientConn
}

func NewSimulatorClient(cc *grpc.ClientConn) SimulatorClient {
	return &simulatorClient{cc}
}

func (c *simulatorClient) Ping(ctx context.Context, in *Request, opts ...grpc.CallOption) (Simulator_PingClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Simulator_serviceDesc.Streams[0], "/simulator.Simulator/Ping", opts...)
	if err != nil {
		return nil, err
	}
	x := &simulatorPingClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Simulator_PingClient interface {
	Recv() (*Reply, error)
	grpc.ClientStream
}

type simulatorPingClient struct {
	grpc.ClientStream
}

func (x *simulatorPingClient) Recv() (*Reply, error) {
	m := new(Reply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *simulatorClient) Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/simulator.Simulator/Init", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SimulatorServer is the server API for Simulator service.
type SimulatorServer interface {
	Ping(*Request, Simulator_PingServer) error
	Init(context.Context, *InitRequest) (*Empty, error)
}

func RegisterSimulatorServer(s *grpc.Server, srv SimulatorServer) {
	s.RegisterService(&_Simulator_serviceDesc, srv)
}

func _Simulator_Ping_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Request)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SimulatorServer).Ping(m, &simulatorPingServer{stream})
}

type Simulator_PingServer interface {
	Send(*Reply) error
	grpc.ServerStream
}

type simulatorPingServer struct {
	grpc.ServerStream
}

func (x *simulatorPingServer) Send(m *Reply) error {
	return x.ServerStream.SendMsg(m)
}

func _Simulator_Init_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InitRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SimulatorServer).Init(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/simulator.Simulator/Init",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SimulatorServer).Init(ctx, req.(*InitRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Simulator_serviceDesc = grpc.ServiceDesc{
	ServiceName: "simulator.Simulator",
	HandlerType: (*SimulatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Init",
			Handler:    _Simulator_Init_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Ping",
			Handler:       _Simulator_Ping_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "simulator.proto",
}

func init() { proto.RegisterFile("simulator.proto", fileDescriptor_simulator_1f083e41507ba06d) }

var fileDescriptor_simulator_1f083e41507ba06d = []byte{
	// 213 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x50, 0x4d, 0x4f, 0x84, 0x30,
	0x10, 0xdd, 0xea, 0xd6, 0x95, 0xd9, 0x83, 0x66, 0x62, 0xcc, 0x66, 0x4f, 0xa4, 0xa7, 0xf5, 0x42,
	0x08, 0x1e, 0x3d, 0x78, 0xd1, 0x03, 0x37, 0x52, 0xfd, 0x03, 0x35, 0x99, 0x10, 0x92, 0x02, 0x85,
	0x16, 0x93, 0xfe, 0x7b, 0x23, 0xcb, 0x47, 0x39, 0xbe, 0xd7, 0xbe, 0xaf, 0x81, 0x07, 0x5b, 0xd5,
	0x83, 0x56, 0xae, 0xed, 0x13, 0xd3, 0xb7, 0xae, 0xc5, 0x68, 0x21, 0xc4, 0x1b, 0x1c, 0x24, 0x75,
	0x03, 0x59, 0x87, 0x67, 0xb8, 0x37, 0x5a, 0x79, 0xea, 0xf3, 0x8f, 0x13, 0x8b, 0xd9, 0x85, 0xcb,
	0x05, 0xe3, 0x13, 0xf0, 0x5f, 0xa5, 0x07, 0x3a, 0xdd, 0xc6, 0xec, 0x12, 0xc9, 0x2b, 0x10, 0x2f,
	0x70, 0xcc, 0x9b, 0xca, 0x05, 0x06, 0x4d, 0x31, 0x2a, 0xec, 0x6c, 0x30, 0x63, 0xf1, 0x0e, 0x5c,
	0x92, 0xd1, 0x1e, 0x63, 0x38, 0xd6, 0x64, 0xad, 0x2a, 0xe9, 0xdb, 0x1b, 0x9a, 0xfe, 0x85, 0xd4,
	0x9a, 0x75, 0x13, 0x66, 0x1d, 0x80, 0x7f, 0xd6, 0xc6, 0xf9, 0xac, 0x83, 0xe8, 0x6b, 0xae, 0x8f,
	0x29, 0xec, 0x8b, 0xaa, 0x29, 0x11, 0x93, 0x75, 0xe3, 0x54, 0xe7, 0xfc, 0xb8, 0xe1, 0x8c, 0xf6,
	0x62, 0x97, 0x32, 0xcc, 0x60, 0xff, 0xdf, 0x19, 0x9f, 0x83, 0xd7, 0x60, 0xc4, 0x46, 0x35, 0x06,
	0x8a, 0xdd, 0xcf, 0xdd, 0x78, 0xb6, 0xd7, 0xbf, 0x00, 0x00, 0x00, 0xff, 0xff, 0xd5, 0xe2, 0x06,
	0xc7, 0x49, 0x01, 0x00, 0x00,
}
