// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/ads/googleads/v2/common/explorer_auto_optimizer_setting.proto

package common

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	_ "google.golang.org/genproto/googleapis/api/annotations"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Settings for the Display Campaign Optimizer, initially named "Explorer".
// Learn more about
// [automatic targeting](https://support.google.com/google-ads/answer/190596).
type ExplorerAutoOptimizerSetting struct {
	// Indicates whether the optimizer is turned on.
	OptIn                *wrappers.BoolValue `protobuf:"bytes,1,opt,name=opt_in,json=optIn,proto3" json:"opt_in,omitempty"`
	XXX_NoUnkeyedLiteral struct{}            `json:"-"`
	XXX_unrecognized     []byte              `json:"-"`
	XXX_sizecache        int32               `json:"-"`
}

func (m *ExplorerAutoOptimizerSetting) Reset()         { *m = ExplorerAutoOptimizerSetting{} }
func (m *ExplorerAutoOptimizerSetting) String() string { return proto.CompactTextString(m) }
func (*ExplorerAutoOptimizerSetting) ProtoMessage()    {}
func (*ExplorerAutoOptimizerSetting) Descriptor() ([]byte, []int) {
	return fileDescriptor_596617ac6a64e0fd, []int{0}
}

func (m *ExplorerAutoOptimizerSetting) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExplorerAutoOptimizerSetting.Unmarshal(m, b)
}
func (m *ExplorerAutoOptimizerSetting) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExplorerAutoOptimizerSetting.Marshal(b, m, deterministic)
}
func (m *ExplorerAutoOptimizerSetting) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExplorerAutoOptimizerSetting.Merge(m, src)
}
func (m *ExplorerAutoOptimizerSetting) XXX_Size() int {
	return xxx_messageInfo_ExplorerAutoOptimizerSetting.Size(m)
}
func (m *ExplorerAutoOptimizerSetting) XXX_DiscardUnknown() {
	xxx_messageInfo_ExplorerAutoOptimizerSetting.DiscardUnknown(m)
}

var xxx_messageInfo_ExplorerAutoOptimizerSetting proto.InternalMessageInfo

func (m *ExplorerAutoOptimizerSetting) GetOptIn() *wrappers.BoolValue {
	if m != nil {
		return m.OptIn
	}
	return nil
}

func init() {
	proto.RegisterType((*ExplorerAutoOptimizerSetting)(nil), "google.ads.googleads.v2.common.ExplorerAutoOptimizerSetting")
}

func init() {
	proto.RegisterFile("google/ads/googleads/v2/common/explorer_auto_optimizer_setting.proto", fileDescriptor_596617ac6a64e0fd)
}

var fileDescriptor_596617ac6a64e0fd = []byte{
	// 306 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x90, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x86, 0x95, 0x22, 0x3a, 0x84, 0xad, 0x13, 0xaa, 0xaa, 0x0a, 0x3a, 0x31, 0x9d, 0x85, 0xd9,
	0xcc, 0xe4, 0x02, 0xaa, 0x98, 0x28, 0x20, 0x65, 0x40, 0x91, 0x22, 0xb7, 0x31, 0x96, 0xa5, 0xc4,
	0x67, 0xd9, 0x4e, 0x41, 0x3c, 0x0e, 0x23, 0x8f, 0xc2, 0xa3, 0xf0, 0x0c, 0x0c, 0xa8, 0x71, 0x92,
	0x8d, 0x4e, 0xf9, 0x15, 0x7f, 0xf7, 0xdf, 0xa7, 0x4b, 0x6f, 0x15, 0xa2, 0xaa, 0x24, 0x11, 0xa5,
	0x27, 0x31, 0xee, 0xd3, 0x8e, 0x92, 0x2d, 0xd6, 0x35, 0x1a, 0x22, 0xdf, 0x6d, 0x85, 0x4e, 0xba,
	0x42, 0x34, 0x01, 0x0b, 0xb4, 0x41, 0xd7, 0xfa, 0x43, 0xba, 0xc2, 0xcb, 0x10, 0xb4, 0x51, 0x60,
	0x1d, 0x06, 0x9c, 0xcc, 0xe3, 0x28, 0x88, 0xd2, 0xc3, 0xd0, 0x02, 0x3b, 0x0a, 0xb1, 0x65, 0xda,
	0xbd, 0x93, 0x96, 0xde, 0x34, 0xaf, 0xe4, 0xcd, 0x09, 0x6b, 0xa5, 0xf3, 0x71, 0x7e, 0x3a, 0xeb,
	0x2d, 0xac, 0x26, 0xc2, 0x18, 0x0c, 0x22, 0x68, 0x34, 0xdd, 0xeb, 0xe2, 0x31, 0x9d, 0xdd, 0x75,
	0x1a, 0xbc, 0x09, 0xf8, 0xd0, 0x4b, 0x3c, 0x47, 0x87, 0xc9, 0x65, 0x3a, 0x46, 0x1b, 0x0a, 0x6d,
	0x4e, 0x93, 0xb3, 0xe4, 0xe2, 0x84, 0x4e, 0x3b, 0x07, 0xe8, 0xd7, 0xc1, 0x12, 0xb1, 0xca, 0x44,
	0xd5, 0xc8, 0xa7, 0x63, 0xb4, 0xe1, 0xde, 0x2c, 0x7f, 0x93, 0x74, 0xb1, 0xc5, 0x1a, 0x0e, 0x7b,
	0x2f, 0xcf, 0x0f, 0xed, 0x5d, 0xef, 0xdb, 0xd7, 0xc9, 0x4b, 0x77, 0x42, 0x50, 0x58, 0x09, 0xa3,
	0x00, 0x9d, 0x22, 0x4a, 0x9a, 0x76, 0x77, 0x7f, 0x52, 0xab, 0xfd, 0x7f, 0x17, 0xbe, 0x8e, 0x9f,
	0xcf, 0xd1, 0xd1, 0x8a, 0xf3, 0xaf, 0xd1, 0x7c, 0x15, 0xcb, 0x78, 0xe9, 0x21, 0xc6, 0x7d, 0xca,
	0x28, 0xdc, 0xb4, 0xd8, 0x77, 0x0f, 0xe4, 0xbc, 0xf4, 0xf9, 0x00, 0xe4, 0x19, 0xcd, 0x23, 0xf0,
	0x33, 0x5a, 0xc4, 0xbf, 0x8c, 0xf1, 0xd2, 0x33, 0x36, 0x20, 0x8c, 0x65, 0x94, 0xb1, 0x08, 0x6d,
	0xc6, 0xad, 0xdd, 0xd5, 0x5f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x04, 0x77, 0x20, 0x5c, 0xfe, 0x01,
	0x00, 0x00,
}
