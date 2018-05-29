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
	InternalMsgType      uint32   `protobuf:"varint,2,opt,name=internalMsgType" json:"internalMsgType,omitempty"`
	Value                string   `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Request) Reset()         { *m = Request{} }
func (m *Request) String() string { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()    {}
func (*Request) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_ef80f7e42aada45b, []int{0}
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

func (m *Request) GetInternalMsgType() uint32 {
	if m != nil {
		return m.InternalMsgType
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
	return fileDescriptor_simulator_ef80f7e42aada45b, []int{1}
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
	InternalMsgType      uint32   `protobuf:"varint,2,opt,name=internalMsgType" json:"internalMsgType,omitempty"`
	Value                string   `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Reply) Reset()         { *m = Reply{} }
func (m *Reply) String() string { return proto.CompactTextString(m) }
func (*Reply) ProtoMessage()    {}
func (*Reply) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_ef80f7e42aada45b, []int{2}
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

func (m *Reply) GetInternalMsgType() uint32 {
	if m != nil {
		return m.InternalMsgType
	}
	return 0
}

func (m *Reply) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

type Proposal struct {
	PlayerID             int32    `protobuf:"varint,1,opt,name=playerID" json:"playerID,omitempty"`
	InternalMsgType      uint32   `protobuf:"varint,2,opt,name=internalMsgType" json:"internalMsgType,omitempty"`
	Value                string   `protobuf:"bytes,3,opt,name=value" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Proposal) Reset()         { *m = Proposal{} }
func (m *Proposal) String() string { return proto.CompactTextString(m) }
func (*Proposal) ProtoMessage()    {}
func (*Proposal) Descriptor() ([]byte, []int) {
	return fileDescriptor_simulator_ef80f7e42aada45b, []int{3}
}
func (m *Proposal) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Proposal.Unmarshal(m, b)
}
func (m *Proposal) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Proposal.Marshal(b, m, deterministic)
}
func (dst *Proposal) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Proposal.Merge(dst, src)
}
func (m *Proposal) XXX_Size() int {
	return xxx_messageInfo_Proposal.Size(m)
}
func (m *Proposal) XXX_DiscardUnknown() {
	xxx_messageInfo_Proposal.DiscardUnknown(m)
}

var xxx_messageInfo_Proposal proto.InternalMessageInfo

func (m *Proposal) GetPlayerID() int32 {
	if m != nil {
		return m.PlayerID
	}
	return 0
}

func (m *Proposal) GetInternalMsgType() uint32 {
	if m != nil {
		return m.InternalMsgType
	}
	return 0
}

func (m *Proposal) GetValue() string {
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
	return fileDescriptor_simulator_ef80f7e42aada45b, []int{4}
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
	proto.RegisterType((*Proposal)(nil), "simulator.Proposal")
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
	Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (Simulator_InitClient, error)
	Exit(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
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

func (c *simulatorClient) Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (Simulator_InitClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Simulator_serviceDesc.Streams[1], "/simulator.Simulator/Init", opts...)
	if err != nil {
		return nil, err
	}
	x := &simulatorInitClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Simulator_InitClient interface {
	Recv() (*Proposal, error)
	grpc.ClientStream
}

type simulatorInitClient struct {
	grpc.ClientStream
}

func (x *simulatorInitClient) Recv() (*Proposal, error) {
	m := new(Proposal)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *simulatorClient) Exit(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/simulator.Simulator/Exit", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SimulatorServer is the server API for Simulator service.
type SimulatorServer interface {
	Ping(*Request, Simulator_PingServer) error
	Init(*InitRequest, Simulator_InitServer) error
	Exit(context.Context, *Empty) (*Empty, error)
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

func _Simulator_Init_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(InitRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SimulatorServer).Init(m, &simulatorInitServer{stream})
}

type Simulator_InitServer interface {
	Send(*Proposal) error
	grpc.ServerStream
}

type simulatorInitServer struct {
	grpc.ServerStream
}

func (x *simulatorInitServer) Send(m *Proposal) error {
	return x.ServerStream.SendMsg(m)
}

func _Simulator_Exit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SimulatorServer).Exit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/simulator.Simulator/Exit",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SimulatorServer).Exit(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _Simulator_serviceDesc = grpc.ServiceDesc{
	ServiceName: "simulator.Simulator",
	HandlerType: (*SimulatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Exit",
			Handler:    _Simulator_Exit_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Ping",
			Handler:       _Simulator_Ping_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Init",
			Handler:       _Simulator_Init_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "simulator.proto",
}

func init() { proto.RegisterFile("simulator.proto", fileDescriptor_simulator_ef80f7e42aada45b) }

var fileDescriptor_simulator_ef80f7e42aada45b = []byte{
	// 266 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x92, 0xc1, 0x4a, 0xc4, 0x30,
	0x10, 0x86, 0x37, 0xba, 0x75, 0xb7, 0xb3, 0xc8, 0xca, 0x28, 0x52, 0x7a, 0x2a, 0x3d, 0x55, 0x90,
	0x65, 0xd1, 0x83, 0x2f, 0xe0, 0x1e, 0xf6, 0x20, 0x94, 0xea, 0x0b, 0x44, 0x18, 0x4b, 0x20, 0x6d,
	0x62, 0x92, 0x8a, 0x79, 0x1f, 0x1f, 0x54, 0xcc, 0xda, 0x35, 0xea, 0x51, 0x3c, 0xfe, 0x5f, 0x92,
	0xf9, 0x67, 0xfe, 0x0c, 0x2c, 0xad, 0xe8, 0x06, 0xc9, 0x9d, 0x32, 0x2b, 0x6d, 0x94, 0x53, 0x98,
	0xee, 0x41, 0x49, 0x30, 0x6b, 0xe8, 0x79, 0x20, 0xeb, 0x30, 0x87, 0xb9, 0x96, 0xdc, 0x93, 0xd9,
	0xde, 0x66, 0xac, 0x60, 0x55, 0xd2, 0xec, 0x35, 0x56, 0xb0, 0x14, 0xbd, 0x23, 0xd3, 0x73, 0x79,
	0x67, 0xdb, 0x07, 0xaf, 0x29, 0x3b, 0x28, 0x58, 0x75, 0xdc, 0xfc, 0xc4, 0x78, 0x06, 0xc9, 0x0b,
	0x97, 0x03, 0x65, 0x87, 0x05, 0xab, 0xd2, 0x66, 0x27, 0xca, 0x0b, 0x58, 0x6c, 0x7b, 0xe1, 0x22,
	0xab, 0xbe, 0x0e, 0xb5, 0xed, 0x68, 0x35, 0xea, 0x52, 0x40, 0xd2, 0x90, 0x96, 0x1e, 0x0b, 0x58,
	0x74, 0x64, 0x2d, 0x6f, 0x29, 0xf8, 0xed, 0xee, 0xc5, 0xe8, 0xcf, 0x5d, 0x3d, 0xc1, 0xbc, 0x36,
	0x4a, 0x2b, 0xcb, 0xe5, 0xbf, 0x4e, 0x3f, 0x83, 0x64, 0xd3, 0x69, 0xe7, 0xaf, 0xde, 0x18, 0xa4,
	0xf7, 0x63, 0xf6, 0xb8, 0x86, 0x69, 0x2d, 0xfa, 0x16, 0x71, 0xf5, 0xf5, 0x41, 0x9f, 0x09, 0xe5,
	0x27, 0xdf, 0x98, 0x96, 0xbe, 0x9c, 0xac, 0x19, 0xde, 0xc0, 0xf4, 0x23, 0x46, 0x3c, 0x8f, 0x4e,
	0xa3, 0x5c, 0xf3, 0xd3, 0x88, 0x8f, 0x93, 0x85, 0x87, 0x97, 0x30, 0xdd, 0xbc, 0x0a, 0x87, 0x71,
	0xd9, 0xd0, 0x52, 0xfe, 0x8b, 0x94, 0x93, 0xc7, 0xa3, 0xb0, 0x26, 0xd7, 0xef, 0x01, 0x00, 0x00,
	0xff, 0xff, 0xeb, 0x77, 0xbf, 0xfd, 0x39, 0x02, 0x00, 0x00,
}
