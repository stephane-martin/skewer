// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: utils/logging/types.proto

/*
	Package logging is a generated protocol buffer package.

	It is generated from these files:
		utils/logging/types.proto

	It has these top-level messages:
		Record
*/
package logging

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

import strings "strings"
import reflect "reflect"
import sortkeys "github.com/gogo/protobuf/sortkeys"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type Record struct {
	Time int64             `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	Lvl  int32             `protobuf:"varint,2,opt,name=lvl,proto3" json:"lvl,omitempty"`
	Msg  string            `protobuf:"bytes,3,opt,name=msg,proto3" json:"msg,omitempty"`
	Ctx  map[string]string `protobuf:"bytes,4,rep,name=ctx" json:"ctx" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (m *Record) Reset()                    { *m = Record{} }
func (*Record) ProtoMessage()               {}
func (*Record) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{0} }

func (m *Record) GetTime() int64 {
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *Record) GetLvl() int32 {
	if m != nil {
		return m.Lvl
	}
	return 0
}

func (m *Record) GetMsg() string {
	if m != nil {
		return m.Msg
	}
	return ""
}

func (m *Record) GetCtx() map[string]string {
	if m != nil {
		return m.Ctx
	}
	return nil
}

func init() {
	proto.RegisterType((*Record)(nil), "logging.Record")
}
func (this *Record) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Record)
	if !ok {
		that2, ok := that.(Record)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.Time != that1.Time {
		return false
	}
	if this.Lvl != that1.Lvl {
		return false
	}
	if this.Msg != that1.Msg {
		return false
	}
	if len(this.Ctx) != len(that1.Ctx) {
		return false
	}
	for i := range this.Ctx {
		if this.Ctx[i] != that1.Ctx[i] {
			return false
		}
	}
	return true
}
func (this *Record) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 8)
	s = append(s, "&logging.Record{")
	s = append(s, "Time: "+fmt.Sprintf("%#v", this.Time)+",\n")
	s = append(s, "Lvl: "+fmt.Sprintf("%#v", this.Lvl)+",\n")
	s = append(s, "Msg: "+fmt.Sprintf("%#v", this.Msg)+",\n")
	keysForCtx := make([]string, 0, len(this.Ctx))
	for k, _ := range this.Ctx {
		keysForCtx = append(keysForCtx, k)
	}
	sortkeys.Strings(keysForCtx)
	mapStringForCtx := "map[string]string{"
	for _, k := range keysForCtx {
		mapStringForCtx += fmt.Sprintf("%#v: %#v,", k, this.Ctx[k])
	}
	mapStringForCtx += "}"
	if this.Ctx != nil {
		s = append(s, "Ctx: "+mapStringForCtx+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringTypes(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *Record) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Record) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Time != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintTypes(dAtA, i, uint64(m.Time))
	}
	if m.Lvl != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintTypes(dAtA, i, uint64(m.Lvl))
	}
	if len(m.Msg) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintTypes(dAtA, i, uint64(len(m.Msg)))
		i += copy(dAtA[i:], m.Msg)
	}
	if len(m.Ctx) > 0 {
		for k, _ := range m.Ctx {
			dAtA[i] = 0x22
			i++
			v := m.Ctx[k]
			mapSize := 1 + len(k) + sovTypes(uint64(len(k))) + 1 + len(v) + sovTypes(uint64(len(v)))
			i = encodeVarintTypes(dAtA, i, uint64(mapSize))
			dAtA[i] = 0xa
			i++
			i = encodeVarintTypes(dAtA, i, uint64(len(k)))
			i += copy(dAtA[i:], k)
			dAtA[i] = 0x12
			i++
			i = encodeVarintTypes(dAtA, i, uint64(len(v)))
			i += copy(dAtA[i:], v)
		}
	}
	return i, nil
}

func encodeVarintTypes(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *Record) Size() (n int) {
	var l int
	_ = l
	if m.Time != 0 {
		n += 1 + sovTypes(uint64(m.Time))
	}
	if m.Lvl != 0 {
		n += 1 + sovTypes(uint64(m.Lvl))
	}
	l = len(m.Msg)
	if l > 0 {
		n += 1 + l + sovTypes(uint64(l))
	}
	if len(m.Ctx) > 0 {
		for k, v := range m.Ctx {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovTypes(uint64(len(k))) + 1 + len(v) + sovTypes(uint64(len(v)))
			n += mapEntrySize + 1 + sovTypes(uint64(mapEntrySize))
		}
	}
	return n
}

func sovTypes(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozTypes(x uint64) (n int) {
	return sovTypes(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *Record) String() string {
	if this == nil {
		return "nil"
	}
	keysForCtx := make([]string, 0, len(this.Ctx))
	for k, _ := range this.Ctx {
		keysForCtx = append(keysForCtx, k)
	}
	sortkeys.Strings(keysForCtx)
	mapStringForCtx := "map[string]string{"
	for _, k := range keysForCtx {
		mapStringForCtx += fmt.Sprintf("%v: %v,", k, this.Ctx[k])
	}
	mapStringForCtx += "}"
	s := strings.Join([]string{`&Record{`,
		`Time:` + fmt.Sprintf("%v", this.Time) + `,`,
		`Lvl:` + fmt.Sprintf("%v", this.Lvl) + `,`,
		`Msg:` + fmt.Sprintf("%v", this.Msg) + `,`,
		`Ctx:` + mapStringForCtx + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringTypes(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *Record) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTypes
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Record: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Record: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Time", wireType)
			}
			m.Time = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Time |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Lvl", wireType)
			}
			m.Lvl = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Lvl |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Msg", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypes
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Msg = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ctx", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTypes
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Ctx == nil {
				m.Ctx = make(map[string]string)
			}
			var mapkey string
			var mapvalue string
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowTypes
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowTypes
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= (uint64(b) & 0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthTypes
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var stringLenmapvalue uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowTypes
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapvalue |= (uint64(b) & 0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapvalue := int(stringLenmapvalue)
					if intStringLenmapvalue < 0 {
						return ErrInvalidLengthTypes
					}
					postStringIndexmapvalue := iNdEx + intStringLenmapvalue
					if postStringIndexmapvalue > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = string(dAtA[iNdEx:postStringIndexmapvalue])
					iNdEx = postStringIndexmapvalue
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipTypes(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if skippy < 0 {
						return ErrInvalidLengthTypes
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.Ctx[mapkey] = mapvalue
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTypes(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTypes
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTypes(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTypes
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTypes
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthTypes
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowTypes
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipTypes(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthTypes = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTypes   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("utils/logging/types.proto", fileDescriptorTypes) }

var fileDescriptorTypes = []byte{
	// 276 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x34, 0x8f, 0x31, 0x4e, 0xfb, 0x30,
	0x18, 0xc5, 0xf3, 0x35, 0x69, 0xff, 0xff, 0x98, 0x05, 0x59, 0x0c, 0xa1, 0xc3, 0x47, 0xc4, 0x94,
	0x01, 0x25, 0x08, 0x10, 0x42, 0x8c, 0x45, 0x5c, 0xc0, 0x37, 0x20, 0xa9, 0x31, 0x11, 0x49, 0x5c,
	0x25, 0x4e, 0xd4, 0x6c, 0x1c, 0x81, 0x63, 0x20, 0x4e, 0xd2, 0xb1, 0x23, 0x13, 0x22, 0x66, 0x61,
	0xec, 0x11, 0x90, 0x9d, 0xb2, 0xfd, 0xde, 0xd3, 0xf3, 0x7b, 0xfe, 0xc8, 0x71, 0xab, 0xf2, 0xa2,
	0x49, 0x0a, 0x29, 0x44, 0x5e, 0x89, 0x44, 0xf5, 0x2b, 0xde, 0xc4, 0xab, 0x5a, 0x2a, 0x49, 0xff,
	0xed, 0xcd, 0xf9, 0x55, 0xc7, 0xab, 0xa5, 0xac, 0x13, 0x91, 0xab, 0xa7, 0x36, 0x8d, 0x33, 0x59,
	0x26, 0x42, 0x0a, 0x99, 0xd8, 0x58, 0xda, 0x3e, 0x5a, 0x65, 0x85, 0xa5, 0xf1, 0xf9, 0xe9, 0x3b,
	0x90, 0x19, 0xe3, 0x99, 0xac, 0x97, 0x94, 0x12, 0x4f, 0xe5, 0x25, 0x0f, 0x20, 0x84, 0xc8, 0x65,
	0x96, 0xe9, 0x21, 0x71, 0x8b, 0xae, 0x08, 0x26, 0x21, 0x44, 0x53, 0x66, 0xd0, 0x38, 0x65, 0x23,
	0x02, 0x37, 0x84, 0xc8, 0x67, 0x06, 0xe9, 0x39, 0x71, 0x33, 0xb5, 0x0e, 0xbc, 0xd0, 0x8d, 0x0e,
	0x2e, 0x82, 0x78, 0xff, 0x9f, 0x78, 0x6c, 0x8d, 0xef, 0xd4, 0xfa, 0xbe, 0x52, 0x75, 0xbf, 0xf0,
	0x36, 0x9f, 0x27, 0x0e, 0x33, 0xd1, 0xf9, 0x35, 0xf9, 0xff, 0x67, 0x9b, 0xbe, 0x67, 0xde, 0xdb,
	0x51, 0x9f, 0x19, 0xa4, 0x47, 0x64, 0xda, 0x3d, 0x14, 0x2d, 0xb7, 0xab, 0x3e, 0x1b, 0xc5, 0xed,
	0xe4, 0x06, 0x16, 0x67, 0xdb, 0x01, 0x9d, 0x8f, 0x01, 0x9d, 0xdd, 0x80, 0xf0, 0xa2, 0x11, 0xde,
	0x34, 0xc2, 0x46, 0x23, 0x6c, 0x35, 0xc2, 0x97, 0x46, 0xf8, 0xd1, 0xe8, 0xec, 0x34, 0xc2, 0xeb,
	0x37, 0x3a, 0xe9, 0xcc, 0x5e, 0x78, 0xf9, 0x1b, 0x00, 0x00, 0xff, 0xff, 0x4a, 0x52, 0x09, 0x26,
	0x3d, 0x01, 0x00, 0x00,
}