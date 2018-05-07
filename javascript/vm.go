package javascript

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/dop251/goja"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

var jsSyslogMessage string = `function SyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, props) {
	this.Priority = p;
	this.Facility = f;
	this.Severity = s;
	this.Version = v;
	this.TimeReported = new Date(timer);
	this.TimeGenerated = new Date(timeg);
	this.Hostname = host;
	this.Appname = app;
	this.Procid = proc;
	this.Msgid = msgid;
	this.Structured = structured;
	this.Message = msg;
	this.Properties = props;
}

function NewSyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, props) {
	return new SyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, props);
}

function NewEmptySyslogMessage() {
	var n = Date.now()
	return new SyslogMessage(0, 0, 0, 1, n, n, "", "", "", "", "", "", {});
}

function SyslogMessageToGo(m) {
	return new SyslogMessage(m.Priority, m.Facility, m.Severity, m.Version, m.TimeReported.getTime(), m.TimeGenerated.getTime(), m.Hostname, m.Appname, m.Procid, m.Msgid, m.Structured, m.Message, m.Properties);
}

var FILTER = {
	PASS: 0,
	DROPPED: 1,
	REJECTED: 2,
	ERROR: 3,
}
`

type FilterResult int64

const (
	PASS         FilterResult = 0
	DROPPED                   = 1
	REJECTED                  = 2
	FILTER_ERROR              = 3
)

type iSyslogMessage struct {
	Priority      int
	Facility      int
	Severity      int
	Version       int
	TimeReported  int64
	TimeGenerated int64
	Hostname      string
	Appname       string
	Procid        string
	Msgid         string
	Structured    string
	Message       string
	SubMessages   []string
	Properties    map[string]map[string]string
}

type ParsersEnvironment interface {
	GetParser(name string) (func(m []byte) ([]*model.SyslogMessage, error), error)
	AddParser(name string, parserFunc string) error
}

func NewParsersEnvironment(logger log15.Logger) *Environment {
	return newEnv("", "", "", "", "", "", logger)
}

type FilterEnvironment interface {
	FilterMessage(m *model.SyslogMessage) (result *model.SyslogMessage, filterResult FilterResult, err error)
	PartitionKey(m model.SyslogMessage) (partitionKey string, errs []error)
	PartitionNumber(m model.SyslogMessage) (partitionNumber int32, errs []error)
	Topic(m *model.SyslogMessage) (topic string, errs []error)
}

func NewFilterEnvironment(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl, partitionNumberFunc string, logger log15.Logger) *Environment {
	return newEnv(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl, partitionNumberFunc, logger)
}

type Environment struct {
	runtime             *goja.Runtime
	logger              log15.Logger
	jsNewSyslogMessage  goja.Callable
	jsSyslogMessageToGo goja.Callable
	jsFilterMessages    goja.Callable
	jsTopic             goja.Callable
	jsPartitionKey      goja.Callable
	jsPartitionNumber   goja.Callable
	jsParsers           map[string]goja.Callable
	topicTmpl           *template.Template
	partitionKeyTmpl    *template.Template
}

type ConcreteParser struct {
	env  *Environment
	name string
}

func (p *ConcreteParser) Parse(rawMessage []byte) ([]*model.SyslogMessage, error) {
	// we don't need a lock to ensure that Parse is thread-safe, as the
	// parser environment is retrieved through a sync.Pool
	var err error
	jsParser, ok := p.env.jsParsers[p.name]
	if !ok {
		return nil, ErrorUnknownFormat(p.name)
	}

	rawMessage = bytes.Trim(rawMessage, "\r\n ")
	if len(rawMessage) == 0 {
		return nil, nil
	}
	jsRawMessage := p.env.runtime.ToValue(string(rawMessage))
	jsParsedMessage, err := jsParser(nil, jsRawMessage)
	if err != nil {
		if jserr, ok := err.(*goja.Exception); ok {
			message, ok := jserr.Value().Export().(string)
			if ok {
				return nil, jsDecodingError(message, p.name)
			}
			return nil, err
		}
		return nil, err
	}
	parsedMessage, err := p.env.fromJsMessage(jsParsedMessage)
	if err != nil {
		return nil, err
	}
	return []*model.SyslogMessage{parsedMessage}, nil
}

func newEnv(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl, partitionNumberFunc string, logger log15.Logger) *Environment {

	e := Environment{}
	e.logger = logger.New("class", "Environment")

	if len(topicTmpl) > 0 {
		t, err := template.New("topic").Parse(topicTmpl)
		if err == nil {
			e.topicTmpl = t
		}
	}
	if len(partitionKeyTmpl) > 0 {
		t, err := template.New("pkey").Parse(partitionKeyTmpl)
		if err == nil {
			e.partitionKeyTmpl = t
		}
	}

	e.jsParsers = map[string]goja.Callable{}

	e.runtime = goja.New()
	_, _ = e.runtime.RunString(jsSyslogMessage)
	v := e.runtime.Get("NewSyslogMessage")
	e.jsNewSyslogMessage, _ = goja.AssertFunction(v)
	v = e.runtime.Get("SyslogMessageToGo")
	e.jsSyslogMessageToGo, _ = goja.AssertFunction(v)

	topicFunc = strings.TrimSpace(topicFunc)
	partitionKeyFunc = strings.TrimSpace(partitionKeyFunc)
	filterFunc = strings.TrimSpace(filterFunc)
	partitionNumberFunc = strings.TrimSpace(partitionNumberFunc)

	if len(topicFunc) > 0 {
		err := e.setTopicFunc(topicFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS Topic() func", "error", err)
		}
	}
	if len(partitionKeyFunc) > 0 {
		err := e.setPartitionKeyFunc(partitionKeyFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS PartitionKey() func", "error", err)
		}
	}
	if len(filterFunc) > 0 {
		err := e.setFilterMessagesFunc(filterFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS Filter() func", "error", err)
		}
	}
	if len(partitionNumberFunc) > 0 {
		err := e.setPartitionNumberFunc(partitionNumberFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS PartitionNumber() func", "error", err)
		}
	}

	return &e
}

func (e *Environment) GetParser(name string) (func(m []byte) ([]*model.SyslogMessage, error), error) {
	_, ok := e.jsParsers[name]
	if !ok {
		return nil, fmt.Errorf("Unknown javascript parser: %s", name)
	}
	c := &ConcreteParser{env: e, name: name}
	return c.Parse, nil
}

func (e *Environment) AddParser(name string, parserFunc string) error {
	name = strings.TrimSpace(name)
	parserFunc = strings.TrimSpace(parserFunc)
	if len(name) == 0 {
		return fmt.Errorf("Empty parser name")
	}
	if len(parserFunc) == 0 {
		return fmt.Errorf("Empty parser function")
	}
	_, err := e.runtime.RunString(parserFunc)
	if err != nil {
		return err
	}
	v := e.runtime.Get(name)
	if v == nil {
		return objectNotFoundError(name)
	}
	parser, b := goja.AssertFunction(v)
	if !b {
		return notAFunctionError(name)
	}
	e.jsParsers[name] = parser
	return nil
}

func (e *Environment) setTopicFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("Topic")
	if v == nil {
		return objectNotFoundError("Topic")
	}
	jsTopic, b := goja.AssertFunction(v)
	if !b {
		return notAFunctionError("Topic")
	}
	e.jsTopic = jsTopic
	return nil
}

func (e *Environment) setPartitionKeyFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("PartitionKey")
	if v == nil {
		return objectNotFoundError("PartitionKey")
	}
	jsPartitionKey, b := goja.AssertFunction(v)
	if !b {
		return notAFunctionError("PartitionKey")
	}
	e.jsPartitionKey = jsPartitionKey
	return nil
}

func (e *Environment) setPartitionNumberFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("PartitionNumber")
	if v == nil {
		return objectNotFoundError("PartitionNumber")
	}
	jsPartitionNumber, b := goja.AssertFunction(v)
	if !b {
		return notAFunctionError("PartitionNumber")
	}
	e.jsPartitionNumber = jsPartitionNumber
	return nil
}

func (e *Environment) setFilterMessagesFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("FilterMessages")
	if v == nil {
		return objectNotFoundError("FilterMessages")
	}
	jsFilter, b := goja.AssertFunction(v)
	if !b {
		return notAFunctionError("FilterMessages")
	}
	e.jsFilterMessages = jsFilter
	return nil
}

func (e *Environment) Topic(m *model.SyslogMessage) (topic string, err error) {
	errs := make([]error, 0, 0)

	if e.jsTopic != nil {
		var jsMessage goja.Value
		var jsTopic goja.Value
		jsMessage, err = e.toJsMessage(m)
		if err == nil {
			jsTopic, err = e.jsTopic(nil, jsMessage)
			if err == nil {
				topic = jsTopic.String()
			} else {
				errs = append(errs, executingJSErrorFactory(err, "Topic"))
			}
		} else {
			errs = append(errs, go2jsError(executingJSErrorFactory(err, "NewSyslogMessage")))
		}
	}
	if len(topic) == 0 && e.topicTmpl != nil {
		topicBuf := bytes.Buffer{}
		err = e.topicTmpl.Execute(&topicBuf, m)
		if err == nil {
			topic = topicBuf.String()
		} else {
			errs = append(errs, err)
		}
	}
	if len(topic) > 0 {
		if !TopicNameIsValid(topic) {
			errs = append(errs, InvalidTopicError(topic))
			return "", eerrors.Combine(errs...)
		}
	}
	return topic, eerrors.Combine(errs...)
}

func (e *Environment) PartitionKey(m *model.SyslogMessage) (partitionKey string, err error) {
	errs := make([]error, 0, 0)
	var jsMessage goja.Value
	var jsPartitionKey goja.Value

	if e.jsPartitionKey != nil {
		jsMessage, err = e.toJsMessage(m)
		if err == nil {
			jsPartitionKey, err = e.jsPartitionKey(nil, jsMessage)
			if err == nil {
				partitionKey = jsPartitionKey.String()
			} else {
				errs = append(errs, executingJSErrorFactory(err, "PartitionKey"))
			}
		} else {
			errs = append(errs, go2jsError(executingJSErrorFactory(err, "NewSyslogMessage")))
		}
	}
	if len(partitionKey) == 0 && e.partitionKeyTmpl != nil {
		partitionKeyBuf := bytes.Buffer{}
		err = e.partitionKeyTmpl.Execute(&partitionKeyBuf, m)
		if err == nil {
			partitionKey = partitionKeyBuf.String()
		} else {
			errs = append(errs, err)
		}
	}
	return partitionKey, eerrors.Combine(errs...)
}

func (e *Environment) PartitionNumber(m *model.SyslogMessage) (partitionNumber int32, err error) {
	errs := make([]error, 0, 0)
	var jsMessage goja.Value
	var jsPartitionNumber goja.Value

	if e.jsPartitionNumber != nil {
		jsMessage, err = e.toJsMessage(m)
		if err == nil {
			jsPartitionNumber, err = e.jsPartitionNumber(nil, jsMessage)
			if err == nil {
				partitionNumber = int32(jsPartitionNumber.ToInteger())
			} else {
				errs = append(errs, executingJSErrorFactory(err, "PartitionNumber"))
			}
		} else {
			errs = append(errs, go2jsError(executingJSErrorFactory(err, "NewSyslogMessage")))
		}
	}
	return partitionNumber, eerrors.Combine(errs...)
}

func (e *Environment) FilterMessage(m *model.SyslogMessage) (filterResult FilterResult, err error) {
	var jsMessage goja.Value
	var resJsMessage goja.Value
	var result *model.SyslogMessage

	if e.jsFilterMessages == nil {
		return PASS, nil
	}
	if m == nil {
		return DROPPED, nil
	}
	jsMessage, err = e.toJsMessage(m)
	if err != nil {
		return FILTER_ERROR, go2jsError(executingJSErrorFactory(err, "NewSyslogMessage"))
	}
	resJsMessage, err = e.jsFilterMessages(nil, jsMessage)
	if err != nil {
		return FILTER_ERROR, executingJSErrorFactory(err, "FilterMessages")
	}

	filterResult = FilterResult(resJsMessage.ToInteger())
	switch filterResult {
	case DROPPED:
		return DROPPED, nil
	case REJECTED:
		return REJECTED, nil
	case FILTER_ERROR:
		return FILTER_ERROR, nil
	case PASS:
		result, err = e.fromJsMessage(jsMessage)
		if err != nil {
			return FILTER_ERROR, js2goError(err)
		}
		if result != nil {
			*m = *result
			model.Free(result)
		}
		return PASS, nil

	default:
		return FILTER_ERROR, jsvmError(eerrors.Errorf("JS filter function returned an invalid result: %d", int64(filterResult)))
	}

}

func (e *Environment) toJsMessage(m *model.SyslogMessage) (sm goja.Value, err error) {
	p := e.runtime.ToValue(int(m.Priority))
	f := e.runtime.ToValue(int(m.Facility))
	s := e.runtime.ToValue(int(m.Severity))
	v := e.runtime.ToValue(int(m.Version))
	timeg := e.runtime.ToValue(m.TimeGeneratedNum / 1000000)
	timer := e.runtime.ToValue(m.TimeReportedNum / 1000000)
	host := e.runtime.ToValue(m.HostName)
	app := e.runtime.ToValue(m.AppName)
	proc := e.runtime.ToValue(m.ProcId)
	msgid := e.runtime.ToValue(m.MsgId)
	structured := e.runtime.ToValue(m.Structured)
	msg := e.runtime.ToValue(m.Message)
	props := e.runtime.ToValue(m.GetAllProperties())

	sm, err = e.jsNewSyslogMessage(nil, p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, props)
	if err != nil {
		return nil, err
	}
	return sm, nil
}

func (e *Environment) fromJsMessage(sm goja.Value) (m *model.SyslogMessage, err error) {
	if goja.IsUndefined(sm) {
		return m, fmt.Errorf("The JS syslog message is 'undefined'")
	}
	var smToGo goja.Value
	smToGo, err = e.jsSyslogMessageToGo(nil, sm)
	if err != nil {
		return m, executingJSErrorFactory(err, "SyslogMessageToGo")
	}
	imsg := iSyslogMessage{}
	err = e.runtime.ExportTo(smToGo, &imsg)
	if err != nil {
		return m, err
	}
	m = model.Factory()
	m.Priority = model.Priority(imsg.Priority)
	m.Facility = model.Facility(imsg.Facility)
	m.Severity = model.Severity(imsg.Severity)
	m.Version = model.Version(imsg.Version)
	m.TimeGeneratedNum = imsg.TimeGenerated * 1000000
	m.TimeReportedNum = imsg.TimeReported * 1000000
	m.HostName = imsg.Hostname
	m.AppName = imsg.Appname
	m.ProcId = imsg.Procid
	m.MsgId = imsg.Msgid
	m.Structured = imsg.Structured
	m.Message = imsg.Message
	m.SetAllProperties(imsg.Properties)
	return m, nil
}

func TopicNameIsValid(name string) bool {
	if len(name) == 0 {
		return false
	}
	if len(name) > 249 {
		return false
	}
	if !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if !validRune(r) {
			return false
		}
	}
	return true
}

func validRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '.' {
		return true
	}
	if r == '_' {
		return true
	}
	if r == '-' {
		return true
	}
	return false
}
