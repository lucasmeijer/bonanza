// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: pkg/proto/remoteexecution/remoteexecution.proto

package remoteexecution

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Execution_Execute_FullMethodName       = "/bonanza.remoteexecution.Execution/Execute"
	Execution_WaitExecution_FullMethodName = "/bonanza.remoteexecution.Execution/WaitExecution"
)

// ExecutionClient is the client API for Execution service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ExecutionClient interface {
	Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error)
	WaitExecution(ctx context.Context, in *WaitExecutionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error)
}

type executionClient struct {
	cc grpc.ClientConnInterface
}

func NewExecutionClient(cc grpc.ClientConnInterface) ExecutionClient {
	return &executionClient{cc}
}

func (c *executionClient) Execute(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Execution_ServiceDesc.Streams[0], Execution_Execute_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ExecuteRequest, ExecuteResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Execution_ExecuteClient = grpc.ServerStreamingClient[ExecuteResponse]

func (c *executionClient) WaitExecution(ctx context.Context, in *WaitExecutionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ExecuteResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Execution_ServiceDesc.Streams[1], Execution_WaitExecution_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[WaitExecutionRequest, ExecuteResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Execution_WaitExecutionClient = grpc.ServerStreamingClient[ExecuteResponse]

// ExecutionServer is the server API for Execution service.
// All implementations should embed UnimplementedExecutionServer
// for forward compatibility.
type ExecutionServer interface {
	Execute(*ExecuteRequest, grpc.ServerStreamingServer[ExecuteResponse]) error
	WaitExecution(*WaitExecutionRequest, grpc.ServerStreamingServer[ExecuteResponse]) error
}

// UnimplementedExecutionServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedExecutionServer struct{}

func (UnimplementedExecutionServer) Execute(*ExecuteRequest, grpc.ServerStreamingServer[ExecuteResponse]) error {
	return status.Errorf(codes.Unimplemented, "method Execute not implemented")
}
func (UnimplementedExecutionServer) WaitExecution(*WaitExecutionRequest, grpc.ServerStreamingServer[ExecuteResponse]) error {
	return status.Errorf(codes.Unimplemented, "method WaitExecution not implemented")
}
func (UnimplementedExecutionServer) testEmbeddedByValue() {}

// UnsafeExecutionServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ExecutionServer will
// result in compilation errors.
type UnsafeExecutionServer interface {
	mustEmbedUnimplementedExecutionServer()
}

func RegisterExecutionServer(s grpc.ServiceRegistrar, srv ExecutionServer) {
	// If the following call pancis, it indicates UnimplementedExecutionServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Execution_ServiceDesc, srv)
}

func _Execution_Execute_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ExecuteRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ExecutionServer).Execute(m, &grpc.GenericServerStream[ExecuteRequest, ExecuteResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Execution_ExecuteServer = grpc.ServerStreamingServer[ExecuteResponse]

func _Execution_WaitExecution_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(WaitExecutionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ExecutionServer).WaitExecution(m, &grpc.GenericServerStream[WaitExecutionRequest, ExecuteResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Execution_WaitExecutionServer = grpc.ServerStreamingServer[ExecuteResponse]

// Execution_ServiceDesc is the grpc.ServiceDesc for Execution service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Execution_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "bonanza.remoteexecution.Execution",
	HandlerType: (*ExecutionServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Execute",
			Handler:       _Execution_Execute_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "WaitExecution",
			Handler:       _Execution_WaitExecution_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pkg/proto/remoteexecution/remoteexecution.proto",
}
