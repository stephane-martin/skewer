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
	var zbai uint32
	zbai, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zbai > 0 {
		zbai--
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
			var zcmr uint32
			zcmr, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Ctx == nil && zcmr > 0 {
				z.Ctx = make(map[string]string, zcmr)
			} else if len(z.Ctx) > 0 {
				for key, _ := range z.Ctx {
					delete(z.Ctx, key)
				}
			}
			for zcmr > 0 {
				zcmr--
				var zxvk string
				var zbzg string
				zxvk, err = dc.ReadString()
				if err != nil {
					return
				}
				zbzg, err = dc.ReadString()
				if err != nil {
					return
				}
				z.Ctx[zxvk] = zbzg
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
	for zxvk, zbzg := range z.Ctx {
		err = en.WriteString(zxvk)
		if err != nil {
			return
		}
		err = en.WriteString(zbzg)
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
	for zxvk, zbzg := range z.Ctx {
		o = msgp.AppendString(o, zxvk)
		o = msgp.AppendString(o, zbzg)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Record) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zajw uint32
	zajw, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
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
			var zwht uint32
			zwht, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Ctx == nil && zwht > 0 {
				z.Ctx = make(map[string]string, zwht)
			} else if len(z.Ctx) > 0 {
				for key, _ := range z.Ctx {
					delete(z.Ctx, key)
				}
			}
			for zwht > 0 {
				var zxvk string
				var zbzg string
				zwht--
				zxvk, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				zbzg, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				z.Ctx[zxvk] = zbzg
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
		for zxvk, zbzg := range z.Ctx {
			_ = zbzg
			s += msgp.StringPrefixSize + len(zxvk) + msgp.StringPrefixSize + len(zbzg)
		}
	}
	return
}
