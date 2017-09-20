package model

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *AuditMessageGroup) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zcmr uint32
	zcmr, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zcmr > 0 {
		zcmr--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "sequence":
			z.Seq, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "timestamp":
			z.AuditTime, err = dc.ReadString()
			if err != nil {
				return
			}
		case "messages":
			var zajw uint32
			zajw, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Msgs) >= int(zajw) {
				z.Msgs = (z.Msgs)[:zajw]
			} else {
				z.Msgs = make([]AuditSubMessage, zajw)
			}
			for zxvk := range z.Msgs {
				var zwht uint32
				zwht, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				for zwht > 0 {
					zwht--
					field, err = dc.ReadMapKeyPtr()
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "type":
						z.Msgs[zxvk].Type, err = dc.ReadUint16()
						if err != nil {
							return
						}
					case "data":
						z.Msgs[zxvk].Data, err = dc.ReadString()
						if err != nil {
							return
						}
					default:
						err = dc.Skip()
						if err != nil {
							return
						}
					}
				}
			}
		case "uid_map":
			var zhct uint32
			zhct, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.UidMap == nil && zhct > 0 {
				z.UidMap = make(map[string]string, zhct)
			} else if len(z.UidMap) > 0 {
				for key, _ := range z.UidMap {
					delete(z.UidMap, key)
				}
			}
			for zhct > 0 {
				zhct--
				var zbzg string
				var zbai string
				zbzg, err = dc.ReadString()
				if err != nil {
					return
				}
				zbai, err = dc.ReadString()
				if err != nil {
					return
				}
				z.UidMap[zbzg] = zbai
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
func (z *AuditMessageGroup) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "sequence"
	err = en.Append(0x84, 0xa8, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.Seq)
	if err != nil {
		return
	}
	// write "timestamp"
	err = en.Append(0xa9, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteString(z.AuditTime)
	if err != nil {
		return
	}
	// write "messages"
	err = en.Append(0xa8, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteArrayHeader(uint32(len(z.Msgs)))
	if err != nil {
		return
	}
	for zxvk := range z.Msgs {
		// map header, size 2
		// write "type"
		err = en.Append(0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
		if err != nil {
			return err
		}
		err = en.WriteUint16(z.Msgs[zxvk].Type)
		if err != nil {
			return
		}
		// write "data"
		err = en.Append(0xa4, 0x64, 0x61, 0x74, 0x61)
		if err != nil {
			return err
		}
		err = en.WriteString(z.Msgs[zxvk].Data)
		if err != nil {
			return
		}
	}
	// write "uid_map"
	err = en.Append(0xa7, 0x75, 0x69, 0x64, 0x5f, 0x6d, 0x61, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.UidMap)))
	if err != nil {
		return
	}
	for zbzg, zbai := range z.UidMap {
		err = en.WriteString(zbzg)
		if err != nil {
			return
		}
		err = en.WriteString(zbai)
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *AuditMessageGroup) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "sequence"
	o = append(o, 0x84, 0xa8, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65)
	o = msgp.AppendInt(o, z.Seq)
	// string "timestamp"
	o = append(o, 0xa9, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70)
	o = msgp.AppendString(o, z.AuditTime)
	// string "messages"
	o = append(o, 0xa8, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Msgs)))
	for zxvk := range z.Msgs {
		// map header, size 2
		// string "type"
		o = append(o, 0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
		o = msgp.AppendUint16(o, z.Msgs[zxvk].Type)
		// string "data"
		o = append(o, 0xa4, 0x64, 0x61, 0x74, 0x61)
		o = msgp.AppendString(o, z.Msgs[zxvk].Data)
	}
	// string "uid_map"
	o = append(o, 0xa7, 0x75, 0x69, 0x64, 0x5f, 0x6d, 0x61, 0x70)
	o = msgp.AppendMapHeader(o, uint32(len(z.UidMap)))
	for zbzg, zbai := range z.UidMap {
		o = msgp.AppendString(o, zbzg)
		o = msgp.AppendString(o, zbai)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *AuditMessageGroup) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zcua uint32
	zcua, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zcua > 0 {
		zcua--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "sequence":
			z.Seq, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "timestamp":
			z.AuditTime, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "messages":
			var zxhx uint32
			zxhx, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Msgs) >= int(zxhx) {
				z.Msgs = (z.Msgs)[:zxhx]
			} else {
				z.Msgs = make([]AuditSubMessage, zxhx)
			}
			for zxvk := range z.Msgs {
				var zlqf uint32
				zlqf, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for zlqf > 0 {
					zlqf--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "type":
						z.Msgs[zxvk].Type, bts, err = msgp.ReadUint16Bytes(bts)
						if err != nil {
							return
						}
					case "data":
						z.Msgs[zxvk].Data, bts, err = msgp.ReadStringBytes(bts)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "uid_map":
			var zdaf uint32
			zdaf, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.UidMap == nil && zdaf > 0 {
				z.UidMap = make(map[string]string, zdaf)
			} else if len(z.UidMap) > 0 {
				for key, _ := range z.UidMap {
					delete(z.UidMap, key)
				}
			}
			for zdaf > 0 {
				var zbzg string
				var zbai string
				zdaf--
				zbzg, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				zbai, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				z.UidMap[zbzg] = zbai
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
func (z *AuditMessageGroup) Msgsize() (s int) {
	s = 1 + 9 + msgp.IntSize + 10 + msgp.StringPrefixSize + len(z.AuditTime) + 9 + msgp.ArrayHeaderSize
	for zxvk := range z.Msgs {
		s += 1 + 5 + msgp.Uint16Size + 5 + msgp.StringPrefixSize + len(z.Msgs[zxvk].Data)
	}
	s += 8 + msgp.MapHeaderSize
	if z.UidMap != nil {
		for zbzg, zbai := range z.UidMap {
			_ = zbai
			s += msgp.StringPrefixSize + len(zbzg) + msgp.StringPrefixSize + len(zbai)
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *AuditSubMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zpks uint32
	zpks, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zpks > 0 {
		zpks--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "type":
			z.Type, err = dc.ReadUint16()
			if err != nil {
				return
			}
		case "data":
			z.Data, err = dc.ReadString()
			if err != nil {
				return
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
func (z AuditSubMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "type"
	err = en.Append(0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteUint16(z.Type)
	if err != nil {
		return
	}
	// write "data"
	err = en.Append(0xa4, 0x64, 0x61, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Data)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z AuditSubMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "type"
	o = append(o, 0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
	o = msgp.AppendUint16(o, z.Type)
	// string "data"
	o = append(o, 0xa4, 0x64, 0x61, 0x74, 0x61)
	o = msgp.AppendString(o, z.Data)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *AuditSubMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zjfb uint32
	zjfb, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zjfb > 0 {
		zjfb--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "type":
			z.Type, bts, err = msgp.ReadUint16Bytes(bts)
			if err != nil {
				return
			}
		case "data":
			z.Data, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
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
func (z AuditSubMessage) Msgsize() (s int) {
	s = 1 + 5 + msgp.Uint16Size + 5 + msgp.StringPrefixSize + len(z.Data)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ExportedMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zcxo uint32
	zcxo, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zcxo > 0 {
		zcxo--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ParsedMessage":
			err = z.ParsedMessage.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "TimeReported":
			z.TimeReported, err = dc.ReadTime()
			if err != nil {
				return
			}
		case "TimeGenerated":
			z.TimeGenerated, err = dc.ReadTime()
			if err != nil {
				return
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
func (z *ExportedMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "ParsedMessage"
	err = en.Append(0x83, 0xad, 0x50, 0x61, 0x72, 0x73, 0x65, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65)
	if err != nil {
		return err
	}
	err = z.ParsedMessage.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "TimeReported"
	err = en.Append(0xac, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.TimeReported)
	if err != nil {
		return
	}
	// write "TimeGenerated"
	err = en.Append(0xad, 0x54, 0x69, 0x6d, 0x65, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteTime(z.TimeGenerated)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ExportedMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "ParsedMessage"
	o = append(o, 0x83, 0xad, 0x50, 0x61, 0x72, 0x73, 0x65, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65)
	o, err = z.ParsedMessage.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "TimeReported"
	o = append(o, 0xac, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	o = msgp.AppendTime(o, z.TimeReported)
	// string "TimeGenerated"
	o = append(o, 0xad, 0x54, 0x69, 0x6d, 0x65, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	o = msgp.AppendTime(o, z.TimeGenerated)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ExportedMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zeff uint32
	zeff, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zeff > 0 {
		zeff--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ParsedMessage":
			bts, err = z.ParsedMessage.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "TimeReported":
			z.TimeReported, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
			}
		case "TimeGenerated":
			z.TimeGenerated, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				return
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
func (z *ExportedMessage) Msgsize() (s int) {
	s = 1 + 14 + z.ParsedMessage.Msgsize() + 13 + msgp.TimeSize + 14 + msgp.TimeSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Facility) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zrsw int
		zrsw, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Facility(zrsw)
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Facility) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Facility) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Facility) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zxpk int
		zxpk, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Facility(zxpk)
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Facility) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ParsedMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zdnj uint32
	zdnj, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zdnj > 0 {
		zdnj--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "fields":
			err = z.Fields.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "client":
			z.Client, err = dc.ReadString()
			if err != nil {
				return
			}
		case "local_port":
			z.LocalPort, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "unix_socket_path":
			z.UnixSocketPath, err = dc.ReadString()
			if err != nil {
				return
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
func (z *ParsedMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "fields"
	err = en.Append(0x84, 0xa6, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73)
	if err != nil {
		return err
	}
	err = z.Fields.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "client"
	err = en.Append(0xa6, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Client)
	if err != nil {
		return
	}
	// write "local_port"
	err = en.Append(0xaa, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f, 0x72, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.LocalPort)
	if err != nil {
		return
	}
	// write "unix_socket_path"
	err = en.Append(0xb0, 0x75, 0x6e, 0x69, 0x78, 0x5f, 0x73, 0x6f, 0x63, 0x6b, 0x65, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68)
	if err != nil {
		return err
	}
	err = en.WriteString(z.UnixSocketPath)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ParsedMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "fields"
	o = append(o, 0x84, 0xa6, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73)
	o, err = z.Fields.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "client"
	o = append(o, 0xa6, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74)
	o = msgp.AppendString(o, z.Client)
	// string "local_port"
	o = append(o, 0xaa, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f, 0x72, 0x74)
	o = msgp.AppendInt(o, z.LocalPort)
	// string "unix_socket_path"
	o = append(o, 0xb0, 0x75, 0x6e, 0x69, 0x78, 0x5f, 0x73, 0x6f, 0x63, 0x6b, 0x65, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68)
	o = msgp.AppendString(o, z.UnixSocketPath)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ParsedMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zobc uint32
	zobc, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zobc > 0 {
		zobc--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "fields":
			bts, err = z.Fields.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "client":
			z.Client, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "local_port":
			z.LocalPort, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "unix_socket_path":
			z.UnixSocketPath, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
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
func (z *ParsedMessage) Msgsize() (s int) {
	s = 1 + 7 + z.Fields.Msgsize() + 7 + msgp.StringPrefixSize + len(z.Client) + 11 + msgp.IntSize + 17 + msgp.StringPrefixSize + len(z.UnixSocketPath)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Priority) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zsnv int
		zsnv, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Priority(zsnv)
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Priority) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Priority) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Priority) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zkgt int
		zkgt, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Priority(zkgt)
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Priority) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *RelpParsedMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zema uint32
	zema, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zema > 0 {
		zema--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "parsed":
			err = z.Parsed.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "txnr":
			z.Txnr, err = dc.ReadInt()
			if err != nil {
				return
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
func (z *RelpParsedMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "parsed"
	err = en.Append(0x82, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = z.Parsed.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "txnr"
	err = en.Append(0xa4, 0x74, 0x78, 0x6e, 0x72)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.Txnr)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *RelpParsedMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "parsed"
	o = append(o, 0x82, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
	o, err = z.Parsed.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "txnr"
	o = append(o, 0xa4, 0x74, 0x78, 0x6e, 0x72)
	o = msgp.AppendInt(o, z.Txnr)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *RelpParsedMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zpez uint32
	zpez, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zpez > 0 {
		zpez--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "parsed":
			bts, err = z.Parsed.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "txnr":
			z.Txnr, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
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
func (z *RelpParsedMessage) Msgsize() (s int) {
	s = 1 + 7 + z.Parsed.Msgsize() + 5 + msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Severity) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zqke int
		zqke, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Severity(zqke)
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Severity) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Severity) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Severity) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zqyh int
		zqyh, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Severity(zqyh)
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Severity) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *SyslogMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zgmo uint32
	zgmo, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zgmo > 0 {
		zgmo--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "priority":
			{
				var ztaf int
				ztaf, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Priority = Priority(ztaf)
			}
		case "facility":
			{
				var zeth int
				zeth, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Facility = Facility(zeth)
			}
		case "severity":
			{
				var zsbz int
				zsbz, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Severity = Severity(zsbz)
			}
		case "version":
			{
				var zrjx int
				zrjx, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Version = Version(zrjx)
			}
		case "timereported":
			z.TimeReported, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "timegenerated":
			z.TimeGenerated, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "hostname":
			z.Hostname, err = dc.ReadString()
			if err != nil {
				return
			}
		case "appname":
			z.Appname, err = dc.ReadString()
			if err != nil {
				return
			}
		case "procid":
			z.Procid, err = dc.ReadString()
			if err != nil {
				return
			}
		case "msgid":
			z.Msgid, err = dc.ReadString()
			if err != nil {
				return
			}
		case "structured":
			z.Structured, err = dc.ReadString()
			if err != nil {
				return
			}
		case "message":
			z.Message, err = dc.ReadString()
			if err != nil {
				return
			}
		case "audit":
			var zawn uint32
			zawn, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.AuditSubMessages) >= int(zawn) {
				z.AuditSubMessages = (z.AuditSubMessages)[:zawn]
			} else {
				z.AuditSubMessages = make([]AuditSubMessage, zawn)
			}
			for zyzr := range z.AuditSubMessages {
				var zwel uint32
				zwel, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				for zwel > 0 {
					zwel--
					field, err = dc.ReadMapKeyPtr()
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "type":
						z.AuditSubMessages[zyzr].Type, err = dc.ReadUint16()
						if err != nil {
							return
						}
					case "data":
						z.AuditSubMessages[zyzr].Data, err = dc.ReadString()
						if err != nil {
							return
						}
					default:
						err = dc.Skip()
						if err != nil {
							return
						}
					}
				}
			}
		case "properties":
			var zrbe uint32
			zrbe, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Properties == nil && zrbe > 0 {
				z.Properties = make(map[string]map[string]string, zrbe)
			} else if len(z.Properties) > 0 {
				for key, _ := range z.Properties {
					delete(z.Properties, key)
				}
			}
			for zrbe > 0 {
				zrbe--
				var zywj string
				var zjpj map[string]string
				zywj, err = dc.ReadString()
				if err != nil {
					return
				}
				var zmfd uint32
				zmfd, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				if zjpj == nil && zmfd > 0 {
					zjpj = make(map[string]string, zmfd)
				} else if len(zjpj) > 0 {
					for key, _ := range zjpj {
						delete(zjpj, key)
					}
				}
				for zmfd > 0 {
					zmfd--
					var zzpf string
					var zrfe string
					zzpf, err = dc.ReadString()
					if err != nil {
						return
					}
					zrfe, err = dc.ReadString()
					if err != nil {
						return
					}
					zjpj[zzpf] = zrfe
				}
				z.Properties[zywj] = zjpj
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
func (z *SyslogMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 14
	// write "priority"
	err = en.Append(0x8e, 0xa8, 0x70, 0x72, 0x69, 0x6f, 0x72, 0x69, 0x74, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.Priority))
	if err != nil {
		return
	}
	// write "facility"
	err = en.Append(0xa8, 0x66, 0x61, 0x63, 0x69, 0x6c, 0x69, 0x74, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.Facility))
	if err != nil {
		return
	}
	// write "severity"
	err = en.Append(0xa8, 0x73, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.Severity))
	if err != nil {
		return
	}
	// write "version"
	err = en.Append(0xa7, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.Version))
	if err != nil {
		return
	}
	// write "timereported"
	err = en.Append(0xac, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.TimeReported)
	if err != nil {
		return
	}
	// write "timegenerated"
	err = en.Append(0xad, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.TimeGenerated)
	if err != nil {
		return
	}
	// write "hostname"
	err = en.Append(0xa8, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Hostname)
	if err != nil {
		return
	}
	// write "appname"
	err = en.Append(0xa7, 0x61, 0x70, 0x70, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Appname)
	if err != nil {
		return
	}
	// write "procid"
	err = en.Append(0xa6, 0x70, 0x72, 0x6f, 0x63, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Procid)
	if err != nil {
		return
	}
	// write "msgid"
	err = en.Append(0xa5, 0x6d, 0x73, 0x67, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Msgid)
	if err != nil {
		return
	}
	// write "structured"
	err = en.Append(0xaa, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x75, 0x72, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Structured)
	if err != nil {
		return
	}
	// write "message"
	err = en.Append(0xa7, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Message)
	if err != nil {
		return
	}
	// write "audit"
	err = en.Append(0xa5, 0x61, 0x75, 0x64, 0x69, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteArrayHeader(uint32(len(z.AuditSubMessages)))
	if err != nil {
		return
	}
	for zyzr := range z.AuditSubMessages {
		// map header, size 2
		// write "type"
		err = en.Append(0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
		if err != nil {
			return err
		}
		err = en.WriteUint16(z.AuditSubMessages[zyzr].Type)
		if err != nil {
			return
		}
		// write "data"
		err = en.Append(0xa4, 0x64, 0x61, 0x74, 0x61)
		if err != nil {
			return err
		}
		err = en.WriteString(z.AuditSubMessages[zyzr].Data)
		if err != nil {
			return
		}
	}
	// write "properties"
	err = en.Append(0xaa, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Properties)))
	if err != nil {
		return
	}
	for zywj, zjpj := range z.Properties {
		err = en.WriteString(zywj)
		if err != nil {
			return
		}
		err = en.WriteMapHeader(uint32(len(zjpj)))
		if err != nil {
			return
		}
		for zzpf, zrfe := range zjpj {
			err = en.WriteString(zzpf)
			if err != nil {
				return
			}
			err = en.WriteString(zrfe)
			if err != nil {
				return
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *SyslogMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 14
	// string "priority"
	o = append(o, 0x8e, 0xa8, 0x70, 0x72, 0x69, 0x6f, 0x72, 0x69, 0x74, 0x79)
	o = msgp.AppendInt(o, int(z.Priority))
	// string "facility"
	o = append(o, 0xa8, 0x66, 0x61, 0x63, 0x69, 0x6c, 0x69, 0x74, 0x79)
	o = msgp.AppendInt(o, int(z.Facility))
	// string "severity"
	o = append(o, 0xa8, 0x73, 0x65, 0x76, 0x65, 0x72, 0x69, 0x74, 0x79)
	o = msgp.AppendInt(o, int(z.Severity))
	// string "version"
	o = append(o, 0xa7, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	o = msgp.AppendInt(o, int(z.Version))
	// string "timereported"
	o = append(o, 0xac, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.TimeReported)
	// string "timegenerated"
	o = append(o, 0xad, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.TimeGenerated)
	// string "hostname"
	o = append(o, 0xa8, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Hostname)
	// string "appname"
	o = append(o, 0xa7, 0x61, 0x70, 0x70, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Appname)
	// string "procid"
	o = append(o, 0xa6, 0x70, 0x72, 0x6f, 0x63, 0x69, 0x64)
	o = msgp.AppendString(o, z.Procid)
	// string "msgid"
	o = append(o, 0xa5, 0x6d, 0x73, 0x67, 0x69, 0x64)
	o = msgp.AppendString(o, z.Msgid)
	// string "structured"
	o = append(o, 0xaa, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x75, 0x72, 0x65, 0x64)
	o = msgp.AppendString(o, z.Structured)
	// string "message"
	o = append(o, 0xa7, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65)
	o = msgp.AppendString(o, z.Message)
	// string "audit"
	o = append(o, 0xa5, 0x61, 0x75, 0x64, 0x69, 0x74)
	o = msgp.AppendArrayHeader(o, uint32(len(z.AuditSubMessages)))
	for zyzr := range z.AuditSubMessages {
		// map header, size 2
		// string "type"
		o = append(o, 0x82, 0xa4, 0x74, 0x79, 0x70, 0x65)
		o = msgp.AppendUint16(o, z.AuditSubMessages[zyzr].Type)
		// string "data"
		o = append(o, 0xa4, 0x64, 0x61, 0x74, 0x61)
		o = msgp.AppendString(o, z.AuditSubMessages[zyzr].Data)
	}
	// string "properties"
	o = append(o, 0xaa, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Properties)))
	for zywj, zjpj := range z.Properties {
		o = msgp.AppendString(o, zywj)
		o = msgp.AppendMapHeader(o, uint32(len(zjpj)))
		for zzpf, zrfe := range zjpj {
			o = msgp.AppendString(o, zzpf)
			o = msgp.AppendString(o, zrfe)
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *SyslogMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zzdc uint32
	zzdc, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zzdc > 0 {
		zzdc--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "priority":
			{
				var zelx int
				zelx, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Priority = Priority(zelx)
			}
		case "facility":
			{
				var zbal int
				zbal, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Facility = Facility(zbal)
			}
		case "severity":
			{
				var zjqz int
				zjqz, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Severity = Severity(zjqz)
			}
		case "version":
			{
				var zkct int
				zkct, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Version = Version(zkct)
			}
		case "timereported":
			z.TimeReported, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "timegenerated":
			z.TimeGenerated, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "hostname":
			z.Hostname, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "appname":
			z.Appname, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "procid":
			z.Procid, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "msgid":
			z.Msgid, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "structured":
			z.Structured, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "message":
			z.Message, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "audit":
			var ztmt uint32
			ztmt, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.AuditSubMessages) >= int(ztmt) {
				z.AuditSubMessages = (z.AuditSubMessages)[:ztmt]
			} else {
				z.AuditSubMessages = make([]AuditSubMessage, ztmt)
			}
			for zyzr := range z.AuditSubMessages {
				var ztco uint32
				ztco, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for ztco > 0 {
					ztco--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "type":
						z.AuditSubMessages[zyzr].Type, bts, err = msgp.ReadUint16Bytes(bts)
						if err != nil {
							return
						}
					case "data":
						z.AuditSubMessages[zyzr].Data, bts, err = msgp.ReadStringBytes(bts)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "properties":
			var zana uint32
			zana, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Properties == nil && zana > 0 {
				z.Properties = make(map[string]map[string]string, zana)
			} else if len(z.Properties) > 0 {
				for key, _ := range z.Properties {
					delete(z.Properties, key)
				}
			}
			for zana > 0 {
				var zywj string
				var zjpj map[string]string
				zana--
				zywj, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				var ztyy uint32
				ztyy, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				if zjpj == nil && ztyy > 0 {
					zjpj = make(map[string]string, ztyy)
				} else if len(zjpj) > 0 {
					for key, _ := range zjpj {
						delete(zjpj, key)
					}
				}
				for ztyy > 0 {
					var zzpf string
					var zrfe string
					ztyy--
					zzpf, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
					zrfe, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
					zjpj[zzpf] = zrfe
				}
				z.Properties[zywj] = zjpj
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
func (z *SyslogMessage) Msgsize() (s int) {
	s = 1 + 9 + msgp.IntSize + 9 + msgp.IntSize + 9 + msgp.IntSize + 8 + msgp.IntSize + 13 + msgp.Int64Size + 14 + msgp.Int64Size + 9 + msgp.StringPrefixSize + len(z.Hostname) + 8 + msgp.StringPrefixSize + len(z.Appname) + 7 + msgp.StringPrefixSize + len(z.Procid) + 6 + msgp.StringPrefixSize + len(z.Msgid) + 11 + msgp.StringPrefixSize + len(z.Structured) + 8 + msgp.StringPrefixSize + len(z.Message) + 6 + msgp.ArrayHeaderSize
	for zyzr := range z.AuditSubMessages {
		s += 1 + 5 + msgp.Uint16Size + 5 + msgp.StringPrefixSize + len(z.AuditSubMessages[zyzr].Data)
	}
	s += 11 + msgp.MapHeaderSize
	if z.Properties != nil {
		for zywj, zjpj := range z.Properties {
			_ = zjpj
			s += msgp.StringPrefixSize + len(zywj) + msgp.MapHeaderSize
			if zjpj != nil {
				for zzpf, zrfe := range zjpj {
					_ = zrfe
					s += msgp.StringPrefixSize + len(zzpf) + msgp.StringPrefixSize + len(zrfe)
				}
			}
		}
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TcpUdpParsedMessage) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zinl uint32
	zinl, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zinl > 0 {
		zinl--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "parsed":
			err = z.Parsed.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "uid":
			z.Uid, err = dc.ReadString()
			if err != nil {
				return
			}
		case "conf_id":
			z.ConfId, err = dc.ReadString()
			if err != nil {
				return
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
func (z *TcpUdpParsedMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "parsed"
	err = en.Append(0x83, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = z.Parsed.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "uid"
	err = en.Append(0xa3, 0x75, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Uid)
	if err != nil {
		return
	}
	// write "conf_id"
	err = en.Append(0xa7, 0x63, 0x6f, 0x6e, 0x66, 0x5f, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.ConfId)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *TcpUdpParsedMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "parsed"
	o = append(o, 0x83, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
	o, err = z.Parsed.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "uid"
	o = append(o, 0xa3, 0x75, 0x69, 0x64)
	o = msgp.AppendString(o, z.Uid)
	// string "conf_id"
	o = append(o, 0xa7, 0x63, 0x6f, 0x6e, 0x66, 0x5f, 0x69, 0x64)
	o = msgp.AppendString(o, z.ConfId)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TcpUdpParsedMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zare uint32
	zare, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zare > 0 {
		zare--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "parsed":
			bts, err = z.Parsed.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "uid":
			z.Uid, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "conf_id":
			z.ConfId, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
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
func (z *TcpUdpParsedMessage) Msgsize() (s int) {
	s = 1 + 7 + z.Parsed.Msgsize() + 4 + msgp.StringPrefixSize + len(z.Uid) + 8 + msgp.StringPrefixSize + len(z.ConfId)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Version) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zljy int
		zljy, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Version(zljy)
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Version) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Version) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Version) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zixj int
		zixj, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Version(zixj)
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Version) Msgsize() (s int) {
	s = msgp.IntSize
	return
}
