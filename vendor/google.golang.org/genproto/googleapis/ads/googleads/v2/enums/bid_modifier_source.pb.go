// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/ads/googleads/v2/enums/bid_modifier_source.proto

package enums

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
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

// Enum describing possible bid modifier sources.
type BidModifierSourceEnum_BidModifierSource int32

const (
	// Not specified.
	BidModifierSourceEnum_UNSPECIFIED BidModifierSourceEnum_BidModifierSource = 0
	// Used for return value only. Represents value unknown in this version.
	BidModifierSourceEnum_UNKNOWN BidModifierSourceEnum_BidModifierSource = 1
	// The bid modifier is specified at the campaign level, on the campaign
	// level criterion.
	BidModifierSourceEnum_CAMPAIGN BidModifierSourceEnum_BidModifierSource = 2
	// The bid modifier is specified (overridden) at the ad group level.
	BidModifierSourceEnum_AD_GROUP BidModifierSourceEnum_BidModifierSource = 3
)

var BidModifierSourceEnum_BidModifierSource_name = map[int32]string{
	0: "UNSPECIFIED",
	1: "UNKNOWN",
	2: "CAMPAIGN",
	3: "AD_GROUP",
}

var BidModifierSourceEnum_BidModifierSource_value = map[string]int32{
	"UNSPECIFIED": 0,
	"UNKNOWN":     1,
	"CAMPAIGN":    2,
	"AD_GROUP":    3,
}

func (x BidModifierSourceEnum_BidModifierSource) String() string {
	return proto.EnumName(BidModifierSourceEnum_BidModifierSource_name, int32(x))
}

func (BidModifierSourceEnum_BidModifierSource) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_93ffe131b49c1099, []int{0, 0}
}

// Container for enum describing possible bid modifier sources.
type BidModifierSourceEnum struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BidModifierSourceEnum) Reset()         { *m = BidModifierSourceEnum{} }
func (m *BidModifierSourceEnum) String() string { return proto.CompactTextString(m) }
func (*BidModifierSourceEnum) ProtoMessage()    {}
func (*BidModifierSourceEnum) Descriptor() ([]byte, []int) {
	return fileDescriptor_93ffe131b49c1099, []int{0}
}

func (m *BidModifierSourceEnum) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BidModifierSourceEnum.Unmarshal(m, b)
}
func (m *BidModifierSourceEnum) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BidModifierSourceEnum.Marshal(b, m, deterministic)
}
func (m *BidModifierSourceEnum) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BidModifierSourceEnum.Merge(m, src)
}
func (m *BidModifierSourceEnum) XXX_Size() int {
	return xxx_messageInfo_BidModifierSourceEnum.Size(m)
}
func (m *BidModifierSourceEnum) XXX_DiscardUnknown() {
	xxx_messageInfo_BidModifierSourceEnum.DiscardUnknown(m)
}

var xxx_messageInfo_BidModifierSourceEnum proto.InternalMessageInfo

func init() {
	proto.RegisterEnum("google.ads.googleads.v2.enums.BidModifierSourceEnum_BidModifierSource", BidModifierSourceEnum_BidModifierSource_name, BidModifierSourceEnum_BidModifierSource_value)
	proto.RegisterType((*BidModifierSourceEnum)(nil), "google.ads.googleads.v2.enums.BidModifierSourceEnum")
}

func init() {
	proto.RegisterFile("google/ads/googleads/v2/enums/bid_modifier_source.proto", fileDescriptor_93ffe131b49c1099)
}

var fileDescriptor_93ffe131b49c1099 = []byte{
	// 309 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x50, 0xdf, 0x4a, 0xf3, 0x30,
	0x1c, 0xfd, 0xd6, 0xc1, 0xa7, 0x64, 0x82, 0xb5, 0xa0, 0x17, 0xe2, 0x2e, 0xb6, 0x07, 0x48, 0xa0,
	0x5e, 0x08, 0xf1, 0x2a, 0xdd, 0x66, 0x19, 0xb2, 0xae, 0x38, 0x56, 0x41, 0x0a, 0xa5, 0x5b, 0xba,
	0x10, 0x58, 0x93, 0xd2, 0xb4, 0x7b, 0x20, 0x2f, 0x7d, 0x14, 0x1f, 0x45, 0x5f, 0x42, 0x9a, 0xac,
	0xbd, 0x19, 0x7a, 0x13, 0x4e, 0x72, 0xfe, 0xe4, 0xfc, 0x7e, 0xe0, 0x81, 0x49, 0xc9, 0xf6, 0x19,
	0x4a, 0xa9, 0x42, 0x06, 0x36, 0xe8, 0xe0, 0xa2, 0x4c, 0xd4, 0xb9, 0x42, 0x1b, 0x4e, 0x93, 0x5c,
	0x52, 0xbe, 0xe3, 0x59, 0x99, 0x28, 0x59, 0x97, 0xdb, 0x0c, 0x16, 0xa5, 0xac, 0xa4, 0x33, 0x34,
	0x6a, 0x98, 0x52, 0x05, 0x3b, 0x23, 0x3c, 0xb8, 0x50, 0x1b, 0x6f, 0xef, 0xda, 0xdc, 0x82, 0xa3,
	0x54, 0x08, 0x59, 0xa5, 0x15, 0x97, 0x42, 0x19, 0xf3, 0x78, 0x07, 0xae, 0x3d, 0x4e, 0x17, 0xc7,
	0xe0, 0x95, 0xce, 0x9d, 0x89, 0x3a, 0x1f, 0x2f, 0xc0, 0xd5, 0x09, 0xe1, 0x5c, 0x82, 0xc1, 0x3a,
	0x58, 0x85, 0xb3, 0xc9, 0xfc, 0x69, 0x3e, 0x9b, 0xda, 0xff, 0x9c, 0x01, 0x38, 0x5b, 0x07, 0xcf,
	0xc1, 0xf2, 0x35, 0xb0, 0x7b, 0xce, 0x05, 0x38, 0x9f, 0x90, 0x45, 0x48, 0xe6, 0x7e, 0x60, 0x5b,
	0xcd, 0x8d, 0x4c, 0x13, 0xff, 0x65, 0xb9, 0x0e, 0xed, 0xbe, 0xf7, 0xdd, 0x03, 0xa3, 0xad, 0xcc,
	0xe1, 0x9f, 0x5d, 0xbd, 0x9b, 0x93, 0x2f, 0xc3, 0xa6, 0x65, 0xd8, 0x7b, 0xf3, 0x8e, 0x46, 0x26,
	0xf7, 0xa9, 0x60, 0x50, 0x96, 0x0c, 0xb1, 0x4c, 0xe8, 0x19, 0xda, 0x6d, 0x15, 0x5c, 0xfd, 0xb2,
	0xbc, 0x47, 0x7d, 0xbe, 0x5b, 0x7d, 0x9f, 0x90, 0x0f, 0x6b, 0xe8, 0x9b, 0x28, 0x42, 0x15, 0x34,
	0xb0, 0x41, 0x91, 0x0b, 0x9b, 0xb9, 0xd5, 0x67, 0xcb, 0xc7, 0x84, 0xaa, 0xb8, 0xe3, 0xe3, 0xc8,
	0x8d, 0x35, 0xff, 0x65, 0x8d, 0xcc, 0x23, 0xc6, 0x84, 0x2a, 0x8c, 0x3b, 0x05, 0xc6, 0x91, 0x8b,
	0xb1, 0xd6, 0x6c, 0xfe, 0xeb, 0x62, 0xf7, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x55, 0x2e, 0x2f,
	0x11, 0xd4, 0x01, 0x00, 0x00,
}
