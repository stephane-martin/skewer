package baseenc

import "strings"

func ParseFormat(format string) Format {
	format = strings.ToLower(strings.TrimSpace(format))
	if f, ok := Formats[format]; ok {
		return f
	}
	return -1
}

type Format int

const (
	RFC5424 Format = 1 + iota
	RFC3164
	JSON
	FullJSON
	AVRO
	FullAVRO
	JSONAVRO
	FullJSONAVRO
	File
	GELF
	Protobuf
)

var Formats = map[string]Format{
	"rfc5424":      RFC5424,
	"rfc3164":      RFC3164,
	"json":         JSON,
	"fulljson":     FullJSON,
	"avro":         AVRO,
	"fullavro":     FullAVRO,
	"jsonavro":     JSONAVRO,
	"fulljsonavro": FullJSONAVRO,
	"file":         File,
	"gelf":         GELF,
	"protobuf":     Protobuf,
	"":             JSON,
}
