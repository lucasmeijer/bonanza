// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: pkg/proto/configuration/model/parser/parser.proto

package parser

import (
	eviction "github.com/buildbarn/bb-storage/pkg/proto/configuration/eviction"
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

type ParsedObjectPool struct {
	state                  protoimpl.MessageState          `protogen:"open.v1"`
	CacheReplacementPolicy eviction.CacheReplacementPolicy `protobuf:"varint,1,opt,name=cache_replacement_policy,json=cacheReplacementPolicy,proto3,enum=buildbarn.configuration.eviction.CacheReplacementPolicy" json:"cache_replacement_policy,omitempty"`
	Count                  int64                           `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	SizeBytes              int64                           `protobuf:"varint,3,opt,name=size_bytes,json=sizeBytes,proto3" json:"size_bytes,omitempty"`
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *ParsedObjectPool) Reset() {
	*x = ParsedObjectPool{}
	mi := &file_pkg_proto_configuration_model_parser_parser_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ParsedObjectPool) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ParsedObjectPool) ProtoMessage() {}

func (x *ParsedObjectPool) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_configuration_model_parser_parser_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ParsedObjectPool.ProtoReflect.Descriptor instead.
func (*ParsedObjectPool) Descriptor() ([]byte, []int) {
	return file_pkg_proto_configuration_model_parser_parser_proto_rawDescGZIP(), []int{0}
}

func (x *ParsedObjectPool) GetCacheReplacementPolicy() eviction.CacheReplacementPolicy {
	if x != nil {
		return x.CacheReplacementPolicy
	}
	return eviction.CacheReplacementPolicy(0)
}

func (x *ParsedObjectPool) GetCount() int64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ParsedObjectPool) GetSizeBytes() int64 {
	if x != nil {
		return x.SizeBytes
	}
	return 0
}

var File_pkg_proto_configuration_model_parser_parser_proto protoreflect.FileDescriptor

var file_pkg_proto_configuration_model_parser_parser_proto_rawDesc = string([]byte{
	0x0a, 0x31, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2f,
	0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x22, 0x62, 0x6f, 0x6e, 0x61, 0x6e, 0x7a, 0x61, 0x2e, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x6d, 0x6f, 0x64, 0x65, 0x6c,
	0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x1a, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2f, 0x65, 0x76, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x65, 0x76, 0x69, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xbb, 0x01, 0x0a, 0x10, 0x50, 0x61, 0x72,
	0x73, 0x65, 0x64, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x50, 0x6f, 0x6f, 0x6c, 0x12, 0x72, 0x0a,
	0x18, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x72, 0x65, 0x70, 0x6c, 0x61, 0x63, 0x65, 0x6d, 0x65,
	0x6e, 0x74, 0x5f, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x38, 0x2e, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x62, 0x61, 0x72, 0x6e, 0x2e, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x65, 0x76, 0x69, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x43, 0x61, 0x63, 0x68, 0x65, 0x52, 0x65, 0x70, 0x6c, 0x61, 0x63, 0x65, 0x6d,
	0x65, 0x6e, 0x74, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x52, 0x16, 0x63, 0x61, 0x63, 0x68, 0x65,
	0x52, 0x65, 0x70, 0x6c, 0x61, 0x63, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x50, 0x6f, 0x6c, 0x69, 0x63,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x69, 0x7a, 0x65, 0x5f,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x73, 0x69, 0x7a,
	0x65, 0x42, 0x79, 0x74, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_pkg_proto_configuration_model_parser_parser_proto_rawDescOnce sync.Once
	file_pkg_proto_configuration_model_parser_parser_proto_rawDescData []byte
)

func file_pkg_proto_configuration_model_parser_parser_proto_rawDescGZIP() []byte {
	file_pkg_proto_configuration_model_parser_parser_proto_rawDescOnce.Do(func() {
		file_pkg_proto_configuration_model_parser_parser_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_pkg_proto_configuration_model_parser_parser_proto_rawDesc), len(file_pkg_proto_configuration_model_parser_parser_proto_rawDesc)))
	})
	return file_pkg_proto_configuration_model_parser_parser_proto_rawDescData
}

var file_pkg_proto_configuration_model_parser_parser_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_configuration_model_parser_parser_proto_goTypes = []any{
	(*ParsedObjectPool)(nil),             // 0: bonanza.configuration.model.parser.ParsedObjectPool
	(eviction.CacheReplacementPolicy)(0), // 1: buildbarn.configuration.eviction.CacheReplacementPolicy
}
var file_pkg_proto_configuration_model_parser_parser_proto_depIdxs = []int32{
	1, // 0: bonanza.configuration.model.parser.ParsedObjectPool.cache_replacement_policy:type_name -> buildbarn.configuration.eviction.CacheReplacementPolicy
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_pkg_proto_configuration_model_parser_parser_proto_init() }
func file_pkg_proto_configuration_model_parser_parser_proto_init() {
	if File_pkg_proto_configuration_model_parser_parser_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_pkg_proto_configuration_model_parser_parser_proto_rawDesc), len(file_pkg_proto_configuration_model_parser_parser_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_configuration_model_parser_parser_proto_goTypes,
		DependencyIndexes: file_pkg_proto_configuration_model_parser_parser_proto_depIdxs,
		MessageInfos:      file_pkg_proto_configuration_model_parser_parser_proto_msgTypes,
	}.Build()
	File_pkg_proto_configuration_model_parser_parser_proto = out.File
	file_pkg_proto_configuration_model_parser_parser_proto_goTypes = nil
	file_pkg_proto_configuration_model_parser_parser_proto_depIdxs = nil
}
