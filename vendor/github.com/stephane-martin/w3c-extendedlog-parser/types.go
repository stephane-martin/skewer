package parser

import (
	"net"
	"strconv"
	"strings"
	"time"
)

type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Ptr
	Slice
	String
	Struct
	UnsafePointer
	MyDate
	MyTime
	MyIP
	MyTimestamp
	MyURI
)

func GuessType(fieldName string) Kind {
	switch fieldName {
	case "date", "x-cookie-date", "x-http-date":
		return MyDate
	case "time":
		return MyTime
	case "time-taken", "rs-time-taken", "sc-time-taken", "rs-service-time-taken", "rs-download-time-taken", "cs-categorization-time-dynamic", "duration":
		return Float64
	case "bytes":
		return Int64
	case "cached":
		return Bool
	case "x-client-address", "x-bluecoat-appliance-primary-address", "x-bluecoat-proxy-primary-address", "cs-uri-address", "c-uri-address":
		return MyIP
	case "sr-uri-address", "s-uri-address", "x-cs-user-login-address":
		return MyIP
	case "connect-time", "dnslookup-time":
		return Int64
	case "gmttime", "localtime", "timestamp", "x-timestamp-unix", "x-timestamp-unix-utc":
		return MyTimestamp
	default:
	}
	if strings.IndexByte(fieldName, '(') != -1 {
		return String
	}
	if strings.HasSuffix(fieldName, "-ip") {
		return MyIP
	}
	if strings.HasSuffix(fieldName, "-dns") {
		return String
	}
	if strings.HasSuffix(fieldName, "-status") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-comment") {
		return String
	}
	if strings.HasSuffix(fieldName, "-method") {
		return String
	}
	if strings.HasSuffix(fieldName, "-uri") {
		return MyURI
	}
	if strings.HasSuffix(fieldName, "-uri-stem") {
		return MyURI
	}
	if strings.HasSuffix(fieldName, "-uri-query") {
		return MyURI
	}
	if strings.HasSuffix(fieldName, "-length") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-headerlength") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-bytes") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-written") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-read") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-operations") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-size") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-port") {
		return Int64
	}
	if strings.HasSuffix(fieldName, "-count") {
		return Int64
	}
	return String
}

func ConvertValue(fieldName string, value string) interface{} {
	value = strings.TrimSpace(value)
	switch fieldName {
	case "date", "x-cookie-date", "x-http-date":
		if value == "" {
			return nil
		}
		d, err := ParseDate(value)
		if err != nil {
			return nil
		}
		return d
	case "time":
		if value == "" {
			return nil
		}
		t, err := ParseTime(value)
		if err != nil {
			return nil
		}
		return t
	case "time-taken", "rs-time-taken", "sc-time-taken", "rs-service-time-taken", "rs-download-time-taken", "cs-categorization-time-dynamic", "duration":
		if value == "" {
			return nil
		}
		ttaken, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil
		}
		return ttaken
	case "bytes":
		if value == "" {
			return nil
		}
		return makeInt(value)
	case "cached":
		return value == "1"
	case "x-client-address", "x-bluecoat-appliance-primary-address", "x-bluecoat-proxy-primary-address", "cs-uri-address", "c-uri-address":
		if value == "" {
			return nil
		}
		return net.ParseIP(value)
	case "sr-uri-address", "s-uri-address", "x-cs-user-login-address":
		if value == "" {
			return nil
		}
		return net.ParseIP(value)
	case "connect-time", "dnslookup-time":
		if value == "" {
			return nil
		}
		return makeInt(value)
	case "gmttime":
		if value == "" {
			return nil
		}
		t, err := time.Parse("02/01/2006:15:04:05", value)
		if err != nil {
			return nil
		}
		return t.UTC()
	case "localtime":
		if value == "" {
			return nil
		}
		var t time.Time
		var err error
		pluspos := strings.Index(value, "+")
		if pluspos == -1 {
			t, err = time.Parse("02/Jan/2006:15:04:05", value)
		} else {
			t, err = time.Parse("02/Jan/2006:15:04:05 -0700", value)
		}
		if err != nil {
			return nil
		}
		return t
	case "timestamp", "x-timestamp-unix", "x-timestamp-unix-utc":
		if value == "" {
			return nil
		}
		i := makeInt(value)
		if i != nil {
			return time.Unix(i.(int64), 0).UTC()
		}
		return nil
	default:
	}
	if strings.IndexByte(fieldName, '(') != -1 {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-ip") {
		return makeIP(value)
	}
	if strings.HasSuffix(fieldName, "-dns") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-status") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-comment") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-method") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-uri") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-uri-stem") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-uri-query") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-length") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-headerlength") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-bytes") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-written") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-read") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-operations") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-size") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-port") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-count") {
		return makeInt(value)
	}
	return makeStr(value)

}
