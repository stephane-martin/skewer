package es

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"
)

type Opts struct {
	S Settings `json:"settings"`
	M Mappings `json:"mappings"`
}

func NewOpts(shards uint, replicas uint, checkStartup bool, refreshInterval time.Duration) Opts {
	return Opts{
		S: NewSettings(shards, replicas, checkStartup, refreshInterval),
		M: NewMappings(),
	}
}

func (o Opts) Marshal() string {
	b, err := json.Marshal(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
	return string(b)
}

type Settings struct {
	Shards   uint          `json:"number_of_shards"`
	Replicas uint          `json:"number_of_replicas"`
	Shard    ShardSettings `json:"shard"`
	Refresh  string        `json:"refresh_interval"`
}

type ShardSettings struct {
	Check bool `json:"check_on_startup"`
}

func NewSettings(s uint, r uint, c bool, refr time.Duration) Settings {
	return Settings{
		Shards:   s,
		Replicas: r,
		Shard:    ShardSettings{Check: c},
		Refresh:  strconv.FormatInt(int64(refr.Seconds()), 10) + "s",
	}
}

type Mappings struct {
	Mtyp MessageType `json:"syslogmsg"`
}

func NewMappings() Mappings {
	return Mappings{
		Mtyp: MessageType{
			Properties: NewMessageFields(),
			DynamicTemplates: [](map[string]DynamicTemplate){
				map[string]DynamicTemplate{"detect_dates": DynamicDatetime()},
				map[string]DynamicTemplate{"detect_ns": DynamicNanoSeconds()},
				map[string]DynamicTemplate{"detect_bytes": DynamicBytes()},
				map[string]DynamicTemplate{"detect_pid": DynamicPID()},
				map[string]DynamicTemplate{"strings_as_keywords": StringsAsKeywords()},
			},
		},
	}
}

type MessageType struct {
	Properties       MessageFields                  `json:"properties"`
	DynamicTemplates [](map[string]DynamicTemplate) `json:"dynamic_templates"`
}

type MessageFields struct {
	Facility   StrField  `json:"facility"`
	Severity   StrField  `json:"severity"`
	Hostname   StrField  `json:"keyword"`
	Appname    StrField  `json:"appname"`
	Procid     StrField  `json:"procid"`
	Msgid      StrField  `json:"msgid"`
	Message    StrField  `json:"message"`
	Treported  DateField `json:"timereported"`
	Tgenerated DateField `json:"timegenerated"`
}

func NewMessageFields() MessageFields {
	return MessageFields{
		Facility:   NewKeyword(),
		Severity:   NewKeyword(),
		Hostname:   NewKeyword(),
		Appname:    NewKeyword(),
		Procid:     NewKeyword(),
		Msgid:      NewKeyword(),
		Message:    NewTextField(),
		Treported:  NewDateField(),
		Tgenerated: NewDateField(),
	}
}

type AnyField interface{}

type StrField struct {
	Typ   string `json:"type"`
	Store bool   `json:"store"`
}

func NewKeyword() StrField {
	return StrField{
		Typ:   "keyword",
		Store: true,
	}
}

func NewTextField() StrField {
	return StrField{
		Typ:   "text",
		Store: true,
	}
}

type DateField struct {
	Typ    string `json:"type"`
	Format string `json:"format"`
	Store  bool   `json:"store"`
}

func NewDateField() DateField {
	return DateField{
		Typ:    "date",
		Format: "strict_date_time_no_millis||strict_date_time",
		Store:  true,
	}
}

type LongField struct {
	Typ   string `json:"type"`
	Store bool   `json:"store"`
}

func NewLongField() LongField {
	return LongField{
		Typ:   "long",
		Store: true,
	}
}

type DynamicTemplate struct {
	MatchMappingType string   `json:"match_mapping_type,omitempty"`
	Match            string   `json:"match,omitempty"`
	Mapping          AnyField `json:"mapping"`
}

func StringsAsKeywords() DynamicTemplate {
	return DynamicTemplate{
		MatchMappingType: "string",
		Mapping:          NewKeyword(),
	}
}

func DynamicDatetime() DynamicTemplate {
	return DynamicTemplate{
		Match:   "*_datetime",
		Mapping: NewDateField(),
	}
}

func DynamicNanoSeconds() DynamicTemplate {
	return DynamicTemplate{
		Match:   "*_ns",
		Mapping: NewLongField(),
	}
}

func DynamicBytes() DynamicTemplate {
	return DynamicTemplate{
		Match:   "*_bytes",
		Mapping: NewLongField(),
	}
}

func DynamicPID() DynamicTemplate {
	return DynamicTemplate{
		Match:   "*_pid",
		Mapping: NewLongField(),
	}
}

type ESLogger struct {
	Logger log15.Logger
}

func (l *ESLogger) Printf(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}
