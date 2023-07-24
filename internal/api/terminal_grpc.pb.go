// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: terminal.proto

package api

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	GuestService_TerminalChannel_FullMethodName = "/GuestService/TerminalChannel"
)

// GuestServiceClient is the client API for GuestService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GuestServiceClient interface {
	TerminalChannel(ctx context.Context, opts ...grpc.CallOption) (GuestService_TerminalChannelClient, error)
}

type guestServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGuestServiceClient(cc grpc.ClientConnInterface) GuestServiceClient {
	return &guestServiceClient{cc}
}

func (c *guestServiceClient) TerminalChannel(ctx context.Context, opts ...grpc.CallOption) (GuestService_TerminalChannelClient, error) {
	stream, err := c.cc.NewStream(ctx, &GuestService_ServiceDesc.Streams[0], GuestService_TerminalChannel_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &guestServiceTerminalChannelClient{stream}
	return x, nil
}

type GuestService_TerminalChannelClient interface {
	Send(*GuestTerminalRequest) error
	Recv() (*GuestTerminalResponse, error)
	grpc.ClientStream
}

type guestServiceTerminalChannelClient struct {
	grpc.ClientStream
}

func (x *guestServiceTerminalChannelClient) Send(m *GuestTerminalRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *guestServiceTerminalChannelClient) Recv() (*GuestTerminalResponse, error) {
	m := new(GuestTerminalResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GuestServiceServer is the server API for GuestService service.
// All implementations must embed UnimplementedGuestServiceServer
// for forward compatibility
type GuestServiceServer interface {
	TerminalChannel(GuestService_TerminalChannelServer) error
	mustEmbedUnimplementedGuestServiceServer()
}

// UnimplementedGuestServiceServer must be embedded to have forward compatible implementations.
type UnimplementedGuestServiceServer struct {
}

func (UnimplementedGuestServiceServer) TerminalChannel(GuestService_TerminalChannelServer) error {
	return status.Errorf(codes.Unimplemented, "method TerminalChannel not implemented")
}
func (UnimplementedGuestServiceServer) mustEmbedUnimplementedGuestServiceServer() {}

// UnsafeGuestServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GuestServiceServer will
// result in compilation errors.
type UnsafeGuestServiceServer interface {
	mustEmbedUnimplementedGuestServiceServer()
}

func RegisterGuestServiceServer(s grpc.ServiceRegistrar, srv GuestServiceServer) {
	s.RegisterService(&GuestService_ServiceDesc, srv)
}

func _GuestService_TerminalChannel_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GuestServiceServer).TerminalChannel(&guestServiceTerminalChannelServer{stream})
}

type GuestService_TerminalChannelServer interface {
	Send(*GuestTerminalResponse) error
	Recv() (*GuestTerminalRequest, error)
	grpc.ServerStream
}

type guestServiceTerminalChannelServer struct {
	grpc.ServerStream
}

func (x *guestServiceTerminalChannelServer) Send(m *GuestTerminalResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *guestServiceTerminalChannelServer) Recv() (*GuestTerminalRequest, error) {
	m := new(GuestTerminalRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GuestService_ServiceDesc is the grpc.ServiceDesc for GuestService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var GuestService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "GuestService",
	HandlerType: (*GuestServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "TerminalChannel",
			Handler:       _GuestService_TerminalChannel_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "terminal.proto",
}

const (
	HostService_ControlChannel_FullMethodName = "/HostService/ControlChannel"
	HostService_DataChannel_FullMethodName    = "/HostService/DataChannel"
)

// HostServiceClient is the client API for HostService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type HostServiceClient interface {
	ControlChannel(ctx context.Context, opts ...grpc.CallOption) (HostService_ControlChannelClient, error)
	DataChannel(ctx context.Context, opts ...grpc.CallOption) (HostService_DataChannelClient, error)
}

type hostServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewHostServiceClient(cc grpc.ClientConnInterface) HostServiceClient {
	return &hostServiceClient{cc}
}

func (c *hostServiceClient) ControlChannel(ctx context.Context, opts ...grpc.CallOption) (HostService_ControlChannelClient, error) {
	stream, err := c.cc.NewStream(ctx, &HostService_ServiceDesc.Streams[0], HostService_ControlChannel_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &hostServiceControlChannelClient{stream}
	return x, nil
}

type HostService_ControlChannelClient interface {
	Send(*HostControlRequest) error
	Recv() (*HostControlResponse, error)
	grpc.ClientStream
}

type hostServiceControlChannelClient struct {
	grpc.ClientStream
}

func (x *hostServiceControlChannelClient) Send(m *HostControlRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hostServiceControlChannelClient) Recv() (*HostControlResponse, error) {
	m := new(HostControlResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *hostServiceClient) DataChannel(ctx context.Context, opts ...grpc.CallOption) (HostService_DataChannelClient, error) {
	stream, err := c.cc.NewStream(ctx, &HostService_ServiceDesc.Streams[1], HostService_DataChannel_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &hostServiceDataChannelClient{stream}
	return x, nil
}

type HostService_DataChannelClient interface {
	Send(*HostDataRequest) error
	Recv() (*HostDataResponse, error)
	grpc.ClientStream
}

type hostServiceDataChannelClient struct {
	grpc.ClientStream
}

func (x *hostServiceDataChannelClient) Send(m *HostDataRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hostServiceDataChannelClient) Recv() (*HostDataResponse, error) {
	m := new(HostDataResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// HostServiceServer is the server API for HostService service.
// All implementations must embed UnimplementedHostServiceServer
// for forward compatibility
type HostServiceServer interface {
	ControlChannel(HostService_ControlChannelServer) error
	DataChannel(HostService_DataChannelServer) error
	mustEmbedUnimplementedHostServiceServer()
}

// UnimplementedHostServiceServer must be embedded to have forward compatible implementations.
type UnimplementedHostServiceServer struct {
}

func (UnimplementedHostServiceServer) ControlChannel(HostService_ControlChannelServer) error {
	return status.Errorf(codes.Unimplemented, "method ControlChannel not implemented")
}
func (UnimplementedHostServiceServer) DataChannel(HostService_DataChannelServer) error {
	return status.Errorf(codes.Unimplemented, "method DataChannel not implemented")
}
func (UnimplementedHostServiceServer) mustEmbedUnimplementedHostServiceServer() {}

// UnsafeHostServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to HostServiceServer will
// result in compilation errors.
type UnsafeHostServiceServer interface {
	mustEmbedUnimplementedHostServiceServer()
}

func RegisterHostServiceServer(s grpc.ServiceRegistrar, srv HostServiceServer) {
	s.RegisterService(&HostService_ServiceDesc, srv)
}

func _HostService_ControlChannel_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HostServiceServer).ControlChannel(&hostServiceControlChannelServer{stream})
}

type HostService_ControlChannelServer interface {
	Send(*HostControlResponse) error
	Recv() (*HostControlRequest, error)
	grpc.ServerStream
}

type hostServiceControlChannelServer struct {
	grpc.ServerStream
}

func (x *hostServiceControlChannelServer) Send(m *HostControlResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hostServiceControlChannelServer) Recv() (*HostControlRequest, error) {
	m := new(HostControlRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _HostService_DataChannel_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HostServiceServer).DataChannel(&hostServiceDataChannelServer{stream})
}

type HostService_DataChannelServer interface {
	Send(*HostDataResponse) error
	Recv() (*HostDataRequest, error)
	grpc.ServerStream
}

type hostServiceDataChannelServer struct {
	grpc.ServerStream
}

func (x *hostServiceDataChannelServer) Send(m *HostDataResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hostServiceDataChannelServer) Recv() (*HostDataRequest, error) {
	m := new(HostDataRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// HostService_ServiceDesc is the grpc.ServiceDesc for HostService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var HostService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "HostService",
	HandlerType: (*HostServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ControlChannel",
			Handler:       _HostService_ControlChannel_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "DataChannel",
			Handler:       _HostService_DataChannel_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "terminal.proto",
}
