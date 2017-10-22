package model

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *ErrInvalid5424) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zxvk uint32
	zxvk, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zxvk > 0 {
		zxvk--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Property":
			z.Property, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Value":
			z.Value, err = dc.ReadIntf()
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
func (z ErrInvalid5424) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Property"
	err = en.Append(0x82, 0xa8, 0x50, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Property)
	if err != nil {
		return
	}
	// write "Value"
	err = en.Append(0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteIntf(z.Value)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ErrInvalid5424) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Property"
	o = append(o, 0x82, 0xa8, 0x50, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79)
	o = msgp.AppendString(o, z.Property)
	// string "Value"
	o = append(o, 0xa5, 0x56, 0x61, 0x6c, 0x75, 0x65)
	o, err = msgp.AppendIntf(o, z.Value)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ErrInvalid5424) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zbzg uint32
	zbzg, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zbzg > 0 {
		zbzg--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Property":
			z.Property, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Value":
			z.Value, bts, err = msgp.ReadIntfBytes(bts)
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
func (z ErrInvalid5424) Msgsize() (s int) {
	s = 1 + 9 + msgp.StringPrefixSize + len(z.Property) + 6 + msgp.GuessSize(z.Value)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Facility) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zbai int
		zbai, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Facility(zbai)
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
		var zcmr int
		zcmr, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Facility(zcmr)
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
	var zajw uint32
	zajw, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
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
	var zwht uint32
	zwht, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zwht > 0 {
		zwht--
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
		var zhct int
		zhct, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Priority(zhct)
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
		var zcua int
		zcua, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Priority(zcua)
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
func (z *Severity) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zxhx int
		zxhx, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Severity(zxhx)
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
		var zlqf int
		zlqf, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Severity(zlqf)
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
	var zeff uint32
	zeff, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zeff > 0 {
		zeff--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "priority":
			{
				var zrsw int
				zrsw, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Priority = Priority(zrsw)
			}
		case "facility":
			{
				var zxpk int
				zxpk, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Facility = Facility(zxpk)
			}
		case "severity":
			{
				var zdnj int
				zdnj, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Severity = Severity(zdnj)
			}
		case "version":
			{
				var zobc int
				zobc, err = dc.ReadInt()
				if err != nil {
					return
				}
				z.Version = Version(zobc)
			}
		case "timereportednum":
			z.TimeReportedNum, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "timegeneratednum":
			z.TimeGeneratedNum, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "timereported":
			z.TimeReported, err = dc.ReadString()
			if err != nil {
				return
			}
		case "timegenerated":
			z.TimeGenerated, err = dc.ReadString()
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
		case "properties":
			var zsnv uint32
			zsnv, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.Properties == nil && zsnv > 0 {
				z.Properties = make(map[string]map[string]string, zsnv)
			} else if len(z.Properties) > 0 {
				for key, _ := range z.Properties {
					delete(z.Properties, key)
				}
			}
			for zsnv > 0 {
				zsnv--
				var zdaf string
				var zpks map[string]string
				zdaf, err = dc.ReadString()
				if err != nil {
					return
				}
				var zkgt uint32
				zkgt, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				if zpks == nil && zkgt > 0 {
					zpks = make(map[string]string, zkgt)
				} else if len(zpks) > 0 {
					for key, _ := range zpks {
						delete(zpks, key)
					}
				}
				for zkgt > 0 {
					zkgt--
					var zjfb string
					var zcxo string
					zjfb, err = dc.ReadString()
					if err != nil {
						return
					}
					zcxo, err = dc.ReadString()
					if err != nil {
						return
					}
					zpks[zjfb] = zcxo
				}
				z.Properties[zdaf] = zpks
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
	// map header, size 15
	// write "priority"
	err = en.Append(0x8f, 0xa8, 0x70, 0x72, 0x69, 0x6f, 0x72, 0x69, 0x74, 0x79)
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
	// write "timereportednum"
	err = en.Append(0xaf, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x6e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.TimeReportedNum)
	if err != nil {
		return
	}
	// write "timegeneratednum"
	err = en.Append(0xb0, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x6e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.TimeGeneratedNum)
	if err != nil {
		return
	}
	// write "timereported"
	err = en.Append(0xac, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.TimeReported)
	if err != nil {
		return
	}
	// write "timegenerated"
	err = en.Append(0xad, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteString(z.TimeGenerated)
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
	// write "properties"
	err = en.Append(0xaa, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.Properties)))
	if err != nil {
		return
	}
	for zdaf, zpks := range z.Properties {
		err = en.WriteString(zdaf)
		if err != nil {
			return
		}
		err = en.WriteMapHeader(uint32(len(zpks)))
		if err != nil {
			return
		}
		for zjfb, zcxo := range zpks {
			err = en.WriteString(zjfb)
			if err != nil {
				return
			}
			err = en.WriteString(zcxo)
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
	// map header, size 15
	// string "priority"
	o = append(o, 0x8f, 0xa8, 0x70, 0x72, 0x69, 0x6f, 0x72, 0x69, 0x74, 0x79)
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
	// string "timereportednum"
	o = append(o, 0xaf, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x6e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, z.TimeReportedNum)
	// string "timegeneratednum"
	o = append(o, 0xb0, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x6e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, z.TimeGeneratedNum)
	// string "timereported"
	o = append(o, 0xac, 0x74, 0x69, 0x6d, 0x65, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64)
	o = msgp.AppendString(o, z.TimeReported)
	// string "timegenerated"
	o = append(o, 0xad, 0x74, 0x69, 0x6d, 0x65, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64)
	o = msgp.AppendString(o, z.TimeGenerated)
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
	// string "properties"
	o = append(o, 0xaa, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Properties)))
	for zdaf, zpks := range z.Properties {
		o = msgp.AppendString(o, zdaf)
		o = msgp.AppendMapHeader(o, uint32(len(zpks)))
		for zjfb, zcxo := range zpks {
			o = msgp.AppendString(o, zjfb)
			o = msgp.AppendString(o, zcxo)
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *SyslogMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zema uint32
	zema, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zema > 0 {
		zema--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "priority":
			{
				var zpez int
				zpez, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Priority = Priority(zpez)
			}
		case "facility":
			{
				var zqke int
				zqke, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Facility = Facility(zqke)
			}
		case "severity":
			{
				var zqyh int
				zqyh, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Severity = Severity(zqyh)
			}
		case "version":
			{
				var zyzr int
				zyzr, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					return
				}
				z.Version = Version(zyzr)
			}
		case "timereportednum":
			z.TimeReportedNum, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "timegeneratednum":
			z.TimeGeneratedNum, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "timereported":
			z.TimeReported, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "timegenerated":
			z.TimeGenerated, bts, err = msgp.ReadStringBytes(bts)
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
		case "properties":
			var zywj uint32
			zywj, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.Properties == nil && zywj > 0 {
				z.Properties = make(map[string]map[string]string, zywj)
			} else if len(z.Properties) > 0 {
				for key, _ := range z.Properties {
					delete(z.Properties, key)
				}
			}
			for zywj > 0 {
				var zdaf string
				var zpks map[string]string
				zywj--
				zdaf, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				var zjpj uint32
				zjpj, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				if zpks == nil && zjpj > 0 {
					zpks = make(map[string]string, zjpj)
				} else if len(zpks) > 0 {
					for key, _ := range zpks {
						delete(zpks, key)
					}
				}
				for zjpj > 0 {
					var zjfb string
					var zcxo string
					zjpj--
					zjfb, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
					zcxo, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						return
					}
					zpks[zjfb] = zcxo
				}
				z.Properties[zdaf] = zpks
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
	s = 1 + 9 + msgp.IntSize + 9 + msgp.IntSize + 9 + msgp.IntSize + 8 + msgp.IntSize + 16 + msgp.Int64Size + 17 + msgp.Int64Size + 13 + msgp.StringPrefixSize + len(z.TimeReported) + 14 + msgp.StringPrefixSize + len(z.TimeGenerated) + 9 + msgp.StringPrefixSize + len(z.Hostname) + 8 + msgp.StringPrefixSize + len(z.Appname) + 7 + msgp.StringPrefixSize + len(z.Procid) + 6 + msgp.StringPrefixSize + len(z.Msgid) + 11 + msgp.StringPrefixSize + len(z.Structured) + 8 + msgp.StringPrefixSize + len(z.Message) + 11 + msgp.MapHeaderSize
	if z.Properties != nil {
		for zdaf, zpks := range z.Properties {
			_ = zpks
			s += msgp.StringPrefixSize + len(zdaf) + msgp.MapHeaderSize
			if zpks != nil {
				for zjfb, zcxo := range zpks {
					_ = zcxo
					s += msgp.StringPrefixSize + len(zjfb) + msgp.StringPrefixSize + len(zcxo)
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
		case "parsed":
			err = z.Parsed.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "uid":
			err = dc.ReadExactBytes((z.Uid)[:])
			if err != nil {
				return
			}
		case "conf_id":
			err = dc.ReadExactBytes((z.ConfId)[:])
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
func (z *TcpUdpParsedMessage) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "parsed"
	err = en.Append(0x84, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
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
	err = en.WriteBytes((z.Uid)[:])
	if err != nil {
		return
	}
	// write "conf_id"
	err = en.Append(0xa7, 0x63, 0x6f, 0x6e, 0x66, 0x5f, 0x69, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteBytes((z.ConfId)[:])
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
func (z *TcpUdpParsedMessage) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "parsed"
	o = append(o, 0x84, 0xa6, 0x70, 0x61, 0x72, 0x73, 0x65, 0x64)
	o, err = z.Parsed.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "uid"
	o = append(o, 0xa3, 0x75, 0x69, 0x64)
	o = msgp.AppendBytes(o, (z.Uid)[:])
	// string "conf_id"
	o = append(o, 0xa7, 0x63, 0x6f, 0x6e, 0x66, 0x5f, 0x69, 0x64)
	o = msgp.AppendBytes(o, (z.ConfId)[:])
	// string "txnr"
	o = append(o, 0xa4, 0x74, 0x78, 0x6e, 0x72)
	o = msgp.AppendInt(o, z.Txnr)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TcpUdpParsedMessage) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var ztaf uint32
	ztaf, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for ztaf > 0 {
		ztaf--
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
			bts, err = msgp.ReadExactBytes(bts, (z.Uid)[:])
			if err != nil {
				return
			}
		case "conf_id":
			bts, err = msgp.ReadExactBytes(bts, (z.ConfId)[:])
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
func (z *TcpUdpParsedMessage) Msgsize() (s int) {
	s = 1 + 7 + z.Parsed.Msgsize() + 4 + msgp.ArrayHeaderSize + (16 * (msgp.ByteSize)) + 8 + msgp.ArrayHeaderSize + (16 * (msgp.ByteSize)) + 5 + msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Version) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zeth int
		zeth, err = dc.ReadInt()
		if err != nil {
			return
		}
		(*z) = Version(zeth)
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
		var zsbz int
		zsbz, bts, err = msgp.ReadIntBytes(bts)
		if err != nil {
			return
		}
		(*z) = Version(zsbz)
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Version) Msgsize() (s int) {
	s = msgp.IntSize
	return
}
