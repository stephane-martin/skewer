package model

//go:generate ffjson $GOFILE

import (
	"fmt"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gogo/protobuf/proto"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/stephane-martin/skewer/model/avro"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

const (
	Fkern Facility = iota
	Fuser
	Fmail
	Fdaemon
	Fauth
	Fsyslog
	Flpr
	Fnews
	Fuucp
	Fclock
	Fauthpriv
	Fftp
	Fntp
	Flogaudit
	Flogalert
	Fcron
	Flocal0
	Flocal1
	Flocal2
	Flocal3
	Flocal4
	Flocal5
	Flocal6
	Flocal7
)

var Facilities map[Facility]string = map[Facility]string{
	0:  "kern",
	1:  "user",
	2:  "mail",
	3:  "daemon",
	4:  "auth",
	5:  "syslog",
	6:  "lpr",
	7:  "news",
	8:  "uucp",
	9:  "clock",
	10: "authpriv",
	11: "ftp",
	12: "ntp",
	13: "logaudit",
	14: "logalert",
	15: "cron",
	16: "local0",
	17: "local1",
	18: "local2",
	19: "local3",
	20: "local4",
	21: "local5",
	22: "local6",
	23: "local7",
}

var Severities map[Severity]string = map[Severity]string{
	0: "emerg",
	1: "alert",
	2: "crit",
	3: "err",
	4: "warning",
	5: "notice",
	6: "info",
	7: "debug",
}

const (
	Semerg Severity = iota
	Salert
	Scrit
	Serr
	SWarning
	Snotice
	Sinfo
	Sdebug
)

var RFacilities map[string]Facility
var RSeverities map[string]Severity

var syslogMsgPool *sync.Pool
var fullMsgPool *sync.Pool

func init() {
	RFacilities = map[string]Facility{}
	RSeverities = map[string]Severity{}
	for k, v := range Facilities {
		RFacilities[v] = k
	}
	for k, v := range Severities {
		RSeverities[v] = k
	}
	syslogMsgPool = &sync.Pool{
		New: func() interface{} {
			return &SyslogMessage{}
		},
	}
	fullMsgPool = &sync.Pool{
		New: func() interface{} {
			return &FullMessage{}
		},
	}
}

func CleanFactory() (msg *SyslogMessage) {
	msg = Factory()
	msg.Clear()
	return
}

func Factory() (msg *SyslogMessage) {
	return syslogMsgPool.Get().(*SyslogMessage)
}

func FullFactory() (msg *FullMessage) {
	msg = fullMsgPool.Get().(*FullMessage)
	msg.Fields = Factory()
	return msg
}

func FromBuf(buf []byte) (msg *FullMessage, err error) {
	msg = FullFactory()
	err = proto.Unmarshal(buf, msg)
	if err != nil {
		FullFree(msg)
		return nil, err
	}
	return msg, nil
}

func FullFactoryFrom(smsg *SyslogMessage) (msg *FullMessage) {
	msg = fullMsgPool.Get().(*FullMessage)
	msg.Fields = smsg
	msg.Txnr = 0
	msg.ConnId = utils.ZeroUid
	msg.ConfId = utils.ZeroUid
	msg.Uid = utils.ZeroUid
	return msg
}

func FullCleanFactory() (msg *FullMessage) {
	msg = fullMsgPool.Get().(*FullMessage)
	msg.Fields = CleanFactory()
	msg.Txnr = 0
	msg.ConnId = utils.ZeroUid
	msg.ConfId = utils.ZeroUid
	msg.Uid = utils.ZeroUid
	return msg
}

type OutputMsg struct {
	Message         *FullMessage
	PartitionKey    string
	PartitionNumber int32
	Topic           string
}

func FullFree(msg *FullMessage) {
	if msg == nil {
		return
	}
	Free(msg.Fields)
	fullMsgPool.Put(msg)
}

func Free(msg *SyslogMessage) {
	if msg == nil {
		return
	}
	syslogMsgPool.Put(msg)
}

type Priority int32
type Facility int32
type Severity int32
type Version int32

func (f Facility) String() string {
	if s, ok := Facilities[f]; ok {
		return s
	}
	return Facilities[Fuser]
}

func FacilityFromString(s string) Facility {
	if f, ok := RFacilities[s]; ok {
		return f
	}
	return Fuser
}

func (s Severity) String() string {
	if st, ok := Severities[s]; ok {
		return st
	}
	return Severities[Sinfo]
}

func SeverityFromString(st string) Severity {
	if s, ok := RSeverities[st]; ok {
		return s
	}
	return Sinfo
}

type RegularSyslog struct {
	Facility      string                       `json:"facility"`
	Severity      string                       `json:"severity"`
	TimeReported  time.Time                    `json:"timereported"`
	TimeGenerated time.Time                    `json:"timegenerated"`
	HostName      string                       `json:"hostname,omitempty"`
	AppName       string                       `json:"appname,omitempty"`
	ProcID        string                       `json:"procid,omitempty"`
	MsgID         string                       `json:"msgid,omitempty"`
	Message       string                       `json:"message,omitempty"`
	Properties    map[string]map[string]string `json:"properties,omitempty"`
}

func (m *RegularSyslog) Internal() (res *SyslogMessage) {
	if m == nil {
		return nil
	}
	res = Factory()
	res.Facility = FacilityFromString(m.Facility)
	res.Severity = SeverityFromString(m.Severity)
	res.Version = 1
	res.TimeReportedNum = m.TimeReported.UnixNano()
	res.TimeGeneratedNum = m.TimeGenerated.UnixNano()
	res.HostName = m.HostName
	res.AppName = m.AppName
	res.ProcId = m.ProcID
	res.MsgId = m.MsgID
	res.Structured = ""
	res.Message = m.Message
	res.SetAllProperties(m.Properties)
	res.SetPriority()
	return res
}

func (m *SyslogMessage) Regular() (reg *RegularSyslog) {
	if m == nil {
		return nil
	}
	return &RegularSyslog{
		Facility:      m.Facility.String(),
		Severity:      m.Severity.String(),
		TimeReported:  time.Unix(0, m.TimeReportedNum),
		TimeGenerated: time.Unix(0, m.TimeGeneratedNum),
		HostName:      m.HostName,
		AppName:       m.AppName,
		ProcID:        m.ProcId,
		MsgID:         m.MsgId,
		Message:       m.Message,
		Properties:    m.GetAllProperties(),
	}
}

func (m *SyslogMessage) Avro() *avro.SyslogMessage {
	if m == nil {
		return nil
	}
	return &avro.SyslogMessage{
		Facility:      m.Facility.String(),
		Severity:      m.Severity.String(),
		TimeReported:  time.Unix(0, m.TimeReportedNum).UTC().Format(time.RFC3339Nano),
		TimeGenerated: time.Unix(0, m.TimeGeneratedNum).UTC().Format(time.RFC3339Nano),
		Hostname:      m.HostName,
		Appname:       m.AppName,
		Procid:        m.ProcId,
		Msgid:         m.MsgId,
		Message:       m.Message,
		Properties:    m.GetAllProperties(),
	}
}

func (m *SyslogMessage) NativeAvro() map[string]interface{} {
	if m == nil {
		return nil
	}
	props := m.GetAllProperties()
	nprops := make(map[string]interface{}, len(props))
	for k, v := range props {
		nprops[k] = v
	}
	return map[string]interface{}{
		"Facility":      m.Facility.String(),
		"Severity":      m.Severity.String(),
		"TimeReported":  time.Unix(0, m.TimeReportedNum).UTC().Format(time.RFC3339Nano),
		"TimeGenerated": time.Unix(0, m.TimeGeneratedNum).UTC().Format(time.RFC3339Nano),
		"Hostname":      m.HostName,
		"Appname":       m.AppName,
		"Procid":        m.ProcId,
		"Msgid":         m.MsgId,
		"Message":       m.Message,
		"Properties":    nprops,
	}
}

func (m *SyslogMessage) RegularJSON() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return ffjson.Marshal(m.Regular())
}

type RegularFullMessage struct {
	ClientAddr string         `json:"client_addr,omitempty"`
	SourceType string         `json:"source_type,omitempty"`
	SourcePath string         `json:"source_path,omitempty"`
	SourcePort int32          `json:"source_port"`
	Uid        string         `json:"uid,omitempty"`
	Fields     *RegularSyslog `json:"fields"`
}

func (m *RegularFullMessage) Internal() (res *FullMessage, err error) {
	if m == nil || m.Fields == nil {
		return nil, nil
	}
	var uid utils.MyULID
	uid, err = utils.ParseMyULID(m.Uid)
	if err != nil {
		return nil, err
	}
	res = FullFactoryFrom(m.Fields.Internal())
	res.ClientAddr = m.ClientAddr
	res.SourceType = m.SourceType
	res.SourcePath = m.SourcePath
	res.SourcePort = m.SourcePort
	res.Uid = uid
	return res, nil
}

func (m *FullMessage) Regular() *RegularFullMessage {
	if m == nil || m.Fields == nil {
		return nil
	}
	return &RegularFullMessage{
		ClientAddr: m.ClientAddr,
		SourceType: m.SourceType,
		SourcePath: m.SourcePath,
		SourcePort: m.SourcePort,
		Uid:        m.Uid.String(),
		Fields:     m.Fields.Regular(),
	}
}

func (m *FullMessage) Avro() *avro.FullMessage {
	if m == nil || m.Fields == nil {
		return nil
	}
	return &avro.FullMessage{
		ClientAddr: m.ClientAddr,
		SourceType: m.SourceType,
		SourcePath: m.SourcePath,
		SourcePort: m.SourcePort,
		Uid:        m.Uid.String(),
		Fields:     m.Fields.Avro(),
	}
}

func (m *FullMessage) NativeAvro() map[string]interface{} {
	if m == nil || m.Fields == nil {
		return nil
	}
	return map[string]interface{}{
		"Fields":     m.Fields.NativeAvro(),
		"ClientAddr": m.ClientAddr,
		"SourceType": m.SourceType,
		"SourcePath": m.SourcePath,
		"SourcePort": m.SourcePort,
		"Uid":        m.Uid.String(),
	}
}

/*
{
  "category" : "policy",
  "processImageUUID" : "12DB7DE7-29C0-329E-9FD7-555E7838BB7F",
  "processUniqueID" : 288,
  "threadID" : 6996578,
  "timestamp" : "2018-02-14 21:33:12.356682+0100",
  "traceID" : 1158816569950212,
  "messageType" : "Default",
  "activityID" : 7384009,
  "processID" : 288,
  "machTimestamp" : 1211974753480388,
  "timezoneName" : "",
  "subsystem" : "com.apple.securityd",
  "senderProgramCounter" : 105038,
  "eventMessage" : "cert[2]: AnchorTrusted =(leaf)[force]> 0",
  "senderImageUUID" : "12DB7DE7-29C0-329E-9FD7-555E7838BB7F",
  "processImagePath" : "\/usr\/libexec\/trustd",
  "senderImagePath" : "\/usr\/libexec\/trustd"
}
*/

// ffjson: noencoder
type MacOSLogMessage struct {
	Category             string `json:"category"`
	ProcessImageUUID     string `json:"processImageUUID"`
	ProcessUniqueID      uint64 `json:"processUniqueID"`
	ThreadID             uint64 `json:"threadID"`
	Timestamp            string `json:"timestamp"`
	TraceID              uint64 `json:"traceID"`
	MessageType          string `json:"messageType"`
	ActivityID           uint64 `json:"activityID"`
	ProcessID            uint64 `json:"processID"`
	MachTimestamp        uint64 `json:"machTimestamp"`
	TimezoneName         string `json:"timezoneName"`
	Subsystem            string `json:"subsystem"`
	SenderProgramCounter uint64 `json:"senderProgramCounter"`
	EventMessage         string `json:"eventMessage"`
	SenderImageUUID      string `json:"senderImageUUID"`
	ProcessImagePath     string `json:"processImagePath"`
	SenderImagePath      string `json:"senderImagePath"`
}

// ffjson: noencoder
type JsonRsyslogMessage struct {
	// used to parsed JSON input from rsyslog
	Message       string `json:"msg"`
	TimeReported  string `json:"timereported"`
	TimeGenerated string `json:"timegenerated"`
	Hostname      string `json:"hostname"`
	Priority      string `json:"pri"`
	Appname       string `json:"app-name"`
	Procid        string `json:"procid"`
	Msgid         string `json:"msgid"`
	Uuid          string `json:"uuid"`
	Structured    string `json:"structured-data"`

	Properties map[string]interface{} `json:"$!"`
}

func (m *SyslogMessage) SetPriority() {
	m.Priority = Priority(int(m.Facility)*8 + int(m.Severity))
}

func (m *SyslogMessage) GetTimeReported() time.Time {
	return time.Unix(0, m.TimeReportedNum).UTC()
}

func (m *SyslogMessage) GetTimeGenerated() time.Time {
	return time.Unix(0, m.TimeGeneratedNum).UTC()
}

func (m *SyslogMessage) Date() string {
	return m.GetTimeReported().Format("2006-01-02")
}

func (m *SyslogMessage) Clear() {
	if m == nil {
		return
	}
	m.ClearProperties()
	m.Priority = 0
	m.Severity = 0
	m.Facility = 0
	m.Version = 0
	m.TimeGeneratedNum = 0
	m.TimeReportedNum = 0
	m.HostName = ""
	m.AppName = ""
	m.ProcId = ""
	m.MsgId = ""
	m.Structured = ""
	m.Message = ""
}

func (m *SyslogMessage) ClearProperties() {
	if m == nil {
		return
	}
	// only allocate a new map if needed
	if m.Properties.Map == nil {
		m.Properties.Map = make(map[string]*InnerProperties)
	}
	if len(m.Properties.Map) == 0 {
		return
	}
	var k string
	for k = range m.Properties.Map {
		delete(m.Properties.Map, k)
	}
}

func (m *SyslogMessage) ClearDomain(domain string) {
	if m.Properties.Map == nil {
		m.Properties.Map = map[string]*InnerProperties{}
	}
	m.Properties.Map[domain] = &InnerProperties{
		Map: map[string]string{},
	}
}

func (m *SyslogMessage) GetProperty(domain, key string) string {
	if len(m.Properties.Map) == 0 {
		return ""
	}
	kv := m.Properties.Map[domain]
	if kv == nil {
		return ""
	}
	if len(kv.Map) == 0 {
		return ""
	}
	return kv.Map[key]
}

func (m *SyslogMessage) SetProperty(domain, key, value string) {
	if m.Properties.Map == nil {
		m.Properties.Map = map[string]*InnerProperties{}
	}
	kv := m.Properties.Map[domain]
	if kv == nil {
		m.Properties.Map[domain] = &InnerProperties{
			Map: map[string]string{},
		}
		kv = m.Properties.Map[domain]
	}
	if kv.Map == nil {
		kv.Map = map[string]string{}
	}
	kv.Map[key] = value
}

func (m *SyslogMessage) SetAllProperties(all map[string](map[string]string)) {
	m.ClearProperties()
	for domain, kv := range all {
		for k, v := range kv {
			m.SetProperty(domain, k, v)
		}
	}
}

func (m *SyslogMessage) GetAllProperties() (res map[string](map[string]string)) {
	res = map[string](map[string]string){}
	if len(m.Properties.Map) == 0 {
		return res
	}
	for domain, inner := range m.Properties.Map {
		if inner == nil {
			continue
		}
		if len(inner.Map) == 0 {
			continue
		}
		res[domain] = map[string]string{}
		for k, v := range inner.Map {
			res[domain][k] = v
		}
	}
	return res
}

func (m *FullMessage) Decrypt(secret *memguard.LockedBuffer, enc []byte) (err error) {
	if len(enc) == 0 {
		return fmt.Errorf("Empty message")
	}
	var dec []byte
	if secret != nil {
		dec, err = sbox.Decrypt(enc, secret)
		if err != nil {
			return err
		}
	} else {
		dec = enc
	}
	err = proto.Unmarshal(dec, m)
	return err
}
