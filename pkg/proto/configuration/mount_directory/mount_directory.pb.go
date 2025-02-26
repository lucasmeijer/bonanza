// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: pkg/proto/configuration/mount_directory/mount_directory.proto

package mount_directory

import (
	virtual "github.com/buildbarn/bb-remote-execution/pkg/proto/configuration/filesystem/virtual"
	eviction "github.com/buildbarn/bb-storage/pkg/proto/configuration/eviction"
	global "github.com/buildbarn/bb-storage/pkg/proto/configuration/global"
	grpc "github.com/buildbarn/bb-storage/pkg/proto/configuration/grpc"
	encoding "github.com/buildbarn/bonanza/pkg/proto/model/encoding"
	object "github.com/buildbarn/bonanza/pkg/proto/storage/object"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ApplicationConfiguration struct {
	state                              protoimpl.MessageState          `protogen:"open.v1"`
	Global                             *global.Configuration           `protobuf:"bytes,1,opt,name=global,proto3" json:"global,omitempty"`
	Mount                              *virtual.MountConfiguration     `protobuf:"bytes,2,opt,name=mount,proto3" json:"mount,omitempty"`
	ParsedObjectCacheReplacementPolicy eviction.CacheReplacementPolicy `protobuf:"varint,3,opt,name=parsed_object_cache_replacement_policy,json=parsedObjectCacheReplacementPolicy,proto3,enum=buildbarn.configuration.eviction.CacheReplacementPolicy" json:"parsed_object_cache_replacement_policy,omitempty"`
	ParsedObjectCacheCount             int64                           `protobuf:"varint,4,opt,name=parsed_object_cache_count,json=parsedObjectCacheCount,proto3" json:"parsed_object_cache_count,omitempty"`
	ParsedObjectCacheTotalSizeBytes    int64                           `protobuf:"varint,5,opt,name=parsed_object_cache_total_size_bytes,json=parsedObjectCacheTotalSizeBytes,proto3" json:"parsed_object_cache_total_size_bytes,omitempty"`
	GrpcClient                         *grpc.ClientConfiguration       `protobuf:"bytes,6,opt,name=grpc_client,json=grpcClient,proto3" json:"grpc_client,omitempty"`
	Namespace                          *object.Namespace               `protobuf:"bytes,7,opt,name=namespace,proto3" json:"namespace,omitempty"`
	RootDirectoryReference             []byte                          `protobuf:"bytes,8,opt,name=root_directory_reference,json=rootDirectoryReference,proto3" json:"root_directory_reference,omitempty"`
	DirectoryEncoders                  []*encoding.BinaryEncoder       `protobuf:"bytes,9,rep,name=directory_encoders,json=directoryEncoders,proto3" json:"directory_encoders,omitempty"`
	SmallFileEncoders                  []*encoding.BinaryEncoder       `protobuf:"bytes,10,rep,name=small_file_encoders,json=smallFileEncoders,proto3" json:"small_file_encoders,omitempty"`
	ConcatenatedFileEncoders           []*encoding.BinaryEncoder       `protobuf:"bytes,11,rep,name=concatenated_file_encoders,json=concatenatedFileEncoders,proto3" json:"concatenated_file_encoders,omitempty"`
	unknownFields                      protoimpl.UnknownFields
	sizeCache                          protoimpl.SizeCache
}

func (x *ApplicationConfiguration) Reset() {
	*x = ApplicationConfiguration{}
	mi := &file_pkg_proto_configuration_mount_directory_mount_directory_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ApplicationConfiguration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplicationConfiguration) ProtoMessage() {}

func (x *ApplicationConfiguration) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_configuration_mount_directory_mount_directory_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplicationConfiguration.ProtoReflect.Descriptor instead.
func (*ApplicationConfiguration) Descriptor() ([]byte, []int) {
	return file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescGZIP(), []int{0}
}

func (x *ApplicationConfiguration) GetGlobal() *global.Configuration {
	if x != nil {
		return x.Global
	}
	return nil
}

func (x *ApplicationConfiguration) GetMount() *virtual.MountConfiguration {
	if x != nil {
		return x.Mount
	}
	return nil
}

func (x *ApplicationConfiguration) GetParsedObjectCacheReplacementPolicy() eviction.CacheReplacementPolicy {
	if x != nil {
		return x.ParsedObjectCacheReplacementPolicy
	}
	return eviction.CacheReplacementPolicy(0)
}

func (x *ApplicationConfiguration) GetParsedObjectCacheCount() int64 {
	if x != nil {
		return x.ParsedObjectCacheCount
	}
	return 0
}

func (x *ApplicationConfiguration) GetParsedObjectCacheTotalSizeBytes() int64 {
	if x != nil {
		return x.ParsedObjectCacheTotalSizeBytes
	}
	return 0
}

func (x *ApplicationConfiguration) GetGrpcClient() *grpc.ClientConfiguration {
	if x != nil {
		return x.GrpcClient
	}
	return nil
}

func (x *ApplicationConfiguration) GetNamespace() *object.Namespace {
	if x != nil {
		return x.Namespace
	}
	return nil
}

func (x *ApplicationConfiguration) GetRootDirectoryReference() []byte {
	if x != nil {
		return x.RootDirectoryReference
	}
	return nil
}

func (x *ApplicationConfiguration) GetDirectoryEncoders() []*encoding.BinaryEncoder {
	if x != nil {
		return x.DirectoryEncoders
	}
	return nil
}

func (x *ApplicationConfiguration) GetSmallFileEncoders() []*encoding.BinaryEncoder {
	if x != nil {
		return x.SmallFileEncoders
	}
	return nil
}

func (x *ApplicationConfiguration) GetConcatenatedFileEncoders() []*encoding.BinaryEncoder {
	if x != nil {
		return x.ConcatenatedFileEncoders
	}
	return nil
}

var File_pkg_proto_configuration_mount_directory_mount_directory_proto protoreflect.FileDescriptor

var file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDesc = string([]byte{
	0x0a, 0x3d, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f,
	0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f,
	0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x25, 0x62, 0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f, 0x64, 0x69, 0x72,
	0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x1a, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f,
	0x65, 0x76, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x65, 0x76, 0x69, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x38, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x2f, 0x76, 0x69, 0x72, 0x74,
	0x75, 0x61, 0x6c, 0x2f, 0x76, 0x69, 0x72, 0x74, 0x75, 0x61, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x2b, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x67, 0x6c, 0x6f, 0x62, 0x61,
	0x6c, 0x2f, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x27,
	0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x67, 0x72, 0x70, 0x63, 0x2f, 0x67, 0x72, 0x70,
	0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x27, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2f, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e,
	0x67, 0x2f, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x25, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x74, 0x6f, 0x72,
	0x61, 0x67, 0x65, 0x2f, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x2f, 0x6f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xb1, 0x07, 0x0a, 0x18, 0x41, 0x70, 0x70, 0x6c,
	0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x45, 0x0a, 0x06, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e,
	0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x67,
	0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x06, 0x67, 0x6c, 0x6f, 0x62, 0x61, 0x6c, 0x12, 0x54, 0x0a, 0x05, 0x6d,
	0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x3e, 0x2e, 0x62, 0x75, 0x69,
	0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x2e,
	0x76, 0x69, 0x72, 0x74, 0x75, 0x61, 0x6c, 0x2e, 0x4d, 0x6f, 0x75, 0x6e, 0x74, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x05, 0x6d, 0x6f, 0x75, 0x6e,
	0x74, 0x12, 0x8c, 0x01, 0x0a, 0x26, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64, 0x5f, 0x6f, 0x62, 0x6a,
	0x65, 0x63, 0x74, 0x5f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x72, 0x65, 0x70, 0x6c, 0x61, 0x63,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x38, 0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x65, 0x76, 0x69,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x43, 0x61, 0x63, 0x68, 0x65, 0x52, 0x65, 0x70, 0x6c, 0x61,
	0x63, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x52, 0x22, 0x70, 0x61,
	0x72, 0x73, 0x65, 0x64, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x43, 0x61, 0x63, 0x68, 0x65, 0x52,
	0x65, 0x70, 0x6c, 0x61, 0x63, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79,
	0x12, 0x39, 0x0a, 0x19, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64, 0x5f, 0x6f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x5f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x16, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64, 0x4f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x43, 0x61, 0x63, 0x68, 0x65, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x4d, 0x0a, 0x24, 0x70,
	0x61, 0x72, 0x73, 0x65, 0x64, 0x5f, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x5f, 0x63, 0x61, 0x63,
	0x68, 0x65, 0x5f, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x5f, 0x62, 0x79,
	0x74, 0x65, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x1f, 0x70, 0x61, 0x72, 0x73, 0x65,
	0x64, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x43, 0x61, 0x63, 0x68, 0x65, 0x54, 0x6f, 0x74, 0x61,
	0x6c, 0x53, 0x69, 0x7a, 0x65, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12, 0x52, 0x0a, 0x0b, 0x67, 0x72,
	0x70, 0x63, 0x5f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x31, 0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x67, 0x72, 0x70, 0x63, 0x2e, 0x43,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x0a, 0x67, 0x72, 0x70, 0x63, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x3f,
	0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x21, 0x2e, 0x62, 0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2e, 0x73, 0x74, 0x6f, 0x72,
	0x61, 0x67, 0x65, 0x2e, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x73,
	0x70, 0x61, 0x63, 0x65, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12,
	0x38, 0x0a, 0x18, 0x72, 0x6f, 0x6f, 0x74, 0x5f, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x6f, 0x72,
	0x79, 0x5f, 0x72, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x16, 0x72, 0x6f, 0x6f, 0x74, 0x44, 0x69, 0x72, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x79,
	0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x54, 0x0a, 0x12, 0x64, 0x69, 0x72,
	0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x5f, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x72, 0x73, 0x18,
	0x09, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x62, 0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2e,
	0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x2e, 0x42,
	0x69, 0x6e, 0x61, 0x72, 0x79, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x72, 0x52, 0x11, 0x64, 0x69,
	0x72, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x79, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x72, 0x73, 0x12,
	0x55, 0x0a, 0x13, 0x73, 0x6d, 0x61, 0x6c, 0x6c, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x65, 0x6e,
	0x63, 0x6f, 0x64, 0x65, 0x72, 0x73, 0x18, 0x0a, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x62,
	0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2e, 0x65, 0x6e, 0x63,
	0x6f, 0x64, 0x69, 0x6e, 0x67, 0x2e, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x45, 0x6e, 0x63, 0x6f,
	0x64, 0x65, 0x72, 0x52, 0x11, 0x73, 0x6d, 0x61, 0x6c, 0x6c, 0x46, 0x69, 0x6c, 0x65, 0x45, 0x6e,
	0x63, 0x6f, 0x64, 0x65, 0x72, 0x73, 0x12, 0x63, 0x0a, 0x1a, 0x63, 0x6f, 0x6e, 0x63, 0x61, 0x74,
	0x65, 0x6e, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x65, 0x6e, 0x63, 0x6f,
	0x64, 0x65, 0x72, 0x73, 0x18, 0x0b, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x62, 0x6f, 0x6e,
	0x61, 0x6e, 0x7a, 0x61, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x64,
	0x69, 0x6e, 0x67, 0x2e, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65,
	0x72, 0x52, 0x18, 0x63, 0x6f, 0x6e, 0x63, 0x61, 0x74, 0x65, 0x6e, 0x61, 0x74, 0x65, 0x64, 0x46,
	0x69, 0x6c, 0x65, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x72, 0x73, 0x42, 0x46, 0x5a, 0x44, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62,
	0x61, 0x72, 0x6e, 0x2f, 0x62, 0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2f, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74,
	0x6f, 0x72, 0x79, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescOnce sync.Once
	file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescData []byte
)

func file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescGZIP() []byte {
	file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescOnce.Do(func() {
		file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDesc), len(file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDesc)))
	})
	return file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDescData
}

var file_pkg_proto_configuration_mount_directory_mount_directory_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_configuration_mount_directory_mount_directory_proto_goTypes = []any{
	(*ApplicationConfiguration)(nil),     // 0: bonanza.configuration.mount_directory.ApplicationConfiguration
	(*global.Configuration)(nil),         // 1: buildbarn.configuration.global.Configuration
	(*virtual.MountConfiguration)(nil),   // 2: buildbarn.configuration.filesystem.virtual.MountConfiguration
	(eviction.CacheReplacementPolicy)(0), // 3: buildbarn.configuration.eviction.CacheReplacementPolicy
	(*grpc.ClientConfiguration)(nil),     // 4: buildbarn.configuration.grpc.ClientConfiguration
	(*object.Namespace)(nil),             // 5: bonanza.storage.object.Namespace
	(*encoding.BinaryEncoder)(nil),       // 6: bonanza.model.encoding.BinaryEncoder
}
var file_pkg_proto_configuration_mount_directory_mount_directory_proto_depIdxs = []int32{
	1, // 0: bonanza.configuration.mount_directory.ApplicationConfiguration.global:type_name -> buildbarn.configuration.global.Configuration
	2, // 1: bonanza.configuration.mount_directory.ApplicationConfiguration.mount:type_name -> buildbarn.configuration.filesystem.virtual.MountConfiguration
	3, // 2: bonanza.configuration.mount_directory.ApplicationConfiguration.parsed_object_cache_replacement_policy:type_name -> buildbarn.configuration.eviction.CacheReplacementPolicy
	4, // 3: bonanza.configuration.mount_directory.ApplicationConfiguration.grpc_client:type_name -> buildbarn.configuration.grpc.ClientConfiguration
	5, // 4: bonanza.configuration.mount_directory.ApplicationConfiguration.namespace:type_name -> bonanza.storage.object.Namespace
	6, // 5: bonanza.configuration.mount_directory.ApplicationConfiguration.directory_encoders:type_name -> bonanza.model.encoding.BinaryEncoder
	6, // 6: bonanza.configuration.mount_directory.ApplicationConfiguration.small_file_encoders:type_name -> bonanza.model.encoding.BinaryEncoder
	6, // 7: bonanza.configuration.mount_directory.ApplicationConfiguration.concatenated_file_encoders:type_name -> bonanza.model.encoding.BinaryEncoder
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_pkg_proto_configuration_mount_directory_mount_directory_proto_init() }
func file_pkg_proto_configuration_mount_directory_mount_directory_proto_init() {
	if File_pkg_proto_configuration_mount_directory_mount_directory_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDesc), len(file_pkg_proto_configuration_mount_directory_mount_directory_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_configuration_mount_directory_mount_directory_proto_goTypes,
		DependencyIndexes: file_pkg_proto_configuration_mount_directory_mount_directory_proto_depIdxs,
		MessageInfos:      file_pkg_proto_configuration_mount_directory_mount_directory_proto_msgTypes,
	}.Build()
	File_pkg_proto_configuration_mount_directory_mount_directory_proto = out.File
	file_pkg_proto_configuration_mount_directory_mount_directory_proto_goTypes = nil
	file_pkg_proto_configuration_mount_directory_mount_directory_proto_depIdxs = nil
}
