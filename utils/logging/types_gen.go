package logging

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Record) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Time":
			z.Time, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "Lvl":
			z.Lvl, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "Msg":
			z.Msg, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Ctx":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Ctx == nil && zb0002 > 0 {
				z.Ctx = make(map[string]string, zb0002)
			} else if len(z.Ctx) > 0 {
				for key, _ := range z.Ctx {
					delete(z.Ctx, key)
				}
			}
			for zb0002 > 0 {
				zb0002--
				var za0001 string
				var za0002 string
				za0001, err = dc.ReadString()
				if err != nil {
					return
				}
				za0002, err = dc.ReadString()
				if err != nil {
					return
				}
				z.Ctx[za0001] = za0002
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Record) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "Time"
	err = en.Append(0x84, 0xa4, 0x54, 0x69, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.Time)
	if err != nil {
		return
	}
	// write "Lvl"
	err = en.Append(0xa3, 0x4c, 0x76, 0x6c)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.Lvl)
	if err != nil {
		return
	}
	// write "Msg"
	err = en.Append(0xa3, 0x4d, 0x73, 0x67)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Msg)
	if err != nil {
		return
	}
	// write "Ctx"
	err = en.Append(0xa3, 0x43, 0x74, 0x78)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Ctx)))
	if err != nil {
		return
	}
	for za0001, za0002 := range z.Ctx {
		err = en.WriteString(za0001)
		if err != nil {
			return
		}
		err = en.WriteString(za0002)
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Record) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "Time"
	o = append(o, 0x84, 0xa4, 0x54, 0x69, 0x6d, 0x65)
	o = msgp.AppendTime(o, z.Time)
	// string "Lvl"
	o = append(o, 0xa3, 0x4c, 0x76, 0x6c)
	o = msgp.AppendInt(o, z.Lvl)
	// string "Msg"
	o = append(o, 0xa3, 0x4d, 0x73, 0x67)
	o = msgp.AppendString(o, z.Msg)
	// string "Ctx"
	o = append(o, 0xa3, 0x43, 0x74, 0x78)
	o = msgp.AppendMapHeader(o, uint32(len(z.Ctx)))
	for za0001, za0002 := range z.Ctx {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendString(o, za0002)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Record) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Time":
			z.Time, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "Lvl":
			z.Lvl, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Msg":
			z.Msg, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Ctx":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Ctx == nil && zb0002 > 0 {
				z.Ctx = make(map[string]string, zb0002)
			} else if len(z.Ctx) > 0 {
				for key, _ := range z.Ctx {
					delete(z.Ctx, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 string
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				za0002, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				z.Ctx[za0001] = za0002
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Record) Msgsize() (s int) {
	s = 1 + 5 + msgp.TimeSize + 4 + msgp.IntSize + 4 + msgp.StringPrefixSize + len(z.Msg) + 4 + msgp.MapHeaderSize
	if z.Ctx != nil {
		for za0001, za0002 := range z.Ctx {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.StringPrefixSize + len(za0002)
		}
	}
	return
}
