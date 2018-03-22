package base

import (
	"strings"
)

type Format int

const (
	RFC5424 Format = iota
	RFC3164
	JSON
	RsyslogJSON
	GELF
	InfluxDB
	Protobuf
	Collectd
	W3C
	LTSV
)

var Formats = map[string]Format{
	"rfc5424":     RFC5424,
	"rfc3164":     RFC3164,
	"json":        JSON,
	"rsyslogjson": RsyslogJSON,
	"gelf":        GELF,
	"influxdb":    InfluxDB,
	"protobuf":    Protobuf,
	"collectd":    Collectd,
	"w3c":         W3C,
	"ltsv":        LTSV,
}

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

//type Parser func(m []byte) ([]*model.SyslogMessage, error)
