package javascript

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/dop251/goja"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

var jsSyslogMessage string = `function SyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, subs, props) {
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
	this.SubMessages = subs;
	this.Properties = props;
}

function NewSyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, subs, props) {
	return new SyslogMessage(p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, subs, props);
}

function NewEmptySyslogMessage() {
	var n = Date.now()
	return new SyslogMessage(0, 0, 0, 1, n, n, "", "", "", "", "", "", [], {});
}

function SyslogMessageToGo(m) {
	return new SyslogMessage(m.Priority, m.Facility, m.Severity, m.Version, m.TimeReported.getTime(), m.TimeGenerated.getTime(), m.Hostname, m.Appname, m.Procid, m.Msgid, m.Structured, m.Message, m.SubMessages, m.Properties);
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

type Parser interface {
	Parse(rawMessage []byte, decoder *encoding.Decoder, dont_parse_sd bool) (model.SyslogMessage, error)
}

type ParsersEnvironment interface {
	GetParser(name string) *ConcreteParser
	AddParser(name string, parserFunc string) error
}

func NewParsersEnvironment(logger log15.Logger) ParsersEnvironment {
	return newEnv("", "", "", "", "", logger)
}

type FilterEnvironment interface {
	FilterMessage(m *model.SyslogMessage) (result *model.SyslogMessage, filterResult FilterResult, err error)
	PartitionKey(m *model.SyslogMessage) (partitionKey string, errs []error)
	Topic(m *model.SyslogMessage) (topic string, errs []error)
}

func NewFilterEnvironment(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl string, logger log15.Logger) *Environment {
	return newEnv(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl, logger)
}

type Environment struct {
	runtime             *goja.Runtime
	logger              log15.Logger
	jsNewSyslogMessage  goja.Callable
	jsSyslogMessageToGo goja.Callable
	jsFilterMessages    goja.Callable
	jsTopic             goja.Callable
	jsPartitionKey      goja.Callable
	jsParsers           map[string]goja.Callable
	topicTmpl           *template.Template
	partitionKeyTmpl    *template.Template
}

type ConcreteParser struct {
	env  *Environment
	name string
}

func (p *ConcreteParser) Parse(rawMessage []byte, decoder *encoding.Decoder, dont_parse_sd bool) (parsedMessage model.SyslogMessage, err error) {
	jsParser, ok := p.env.jsParsers[p.name]
	if !ok {
		return parsedMessage, &model.UnknownFormatError{Format: p.name}
	}

	if decoder == nil {
		decoder = unicode.UTF8.NewDecoder()
	}
	rawMessage, err = decoder.Bytes(rawMessage)
	if err != nil {
		return parsedMessage, &model.InvalidEncodingError{Err: err}
	}

	rawMessage = bytes.Trim(rawMessage, "\r\n ")
	if len(rawMessage) == 0 {
		return parsedMessage, nil
	}
	jsRawMessage := p.env.runtime.ToValue(string(rawMessage))
	jsParsedMessage, err := jsParser(nil, jsRawMessage)
	if err != nil {
		if jserr, ok := err.(*goja.Exception); ok {
			message, ok := jserr.Value().Export().(string)
			if ok {
				return parsedMessage, &model.JSParsingError{ParserName: p.name, Message: message}
			} else {
				return parsedMessage, err
			}
		} else {
			return parsedMessage, err
		}
	}
	parsedMessage, err = p.env.fromJsMessage(jsParsedMessage)
	if err != nil {
		return parsedMessage, err
	}
	return parsedMessage, nil
}

func newEnv(filterFunc, topicFunc, topicTmpl, partitionKeyFunc, partitionKeyTmpl string, logger log15.Logger) *Environment {

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
	e.runtime.RunString(jsSyslogMessage)
	v := e.runtime.Get("NewSyslogMessage")
	e.jsNewSyslogMessage, _ = goja.AssertFunction(v)
	v = e.runtime.Get("SyslogMessageToGo")
	e.jsSyslogMessageToGo, _ = goja.AssertFunction(v)

	topicFunc = strings.TrimSpace(topicFunc)
	partitionKeyFunc = strings.TrimSpace(partitionKeyFunc)
	filterFunc = strings.TrimSpace(filterFunc)

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
	return &e
}

func (e *Environment) GetParser(name string) *ConcreteParser {
	_, ok := e.jsParsers[name]
	if !ok {
		return nil
	}
	return &ConcreteParser{env: e, name: name}
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
		return &ObjectNotFoundError{name}
	}
	parser, b := goja.AssertFunction(v)
	if !b {
		return &NotAFunctionError{name}
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
		return &ObjectNotFoundError{"Topic"}
	}
	jsTopic, b := goja.AssertFunction(v)
	if !b {
		return &NotAFunctionError{"Topic"}
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
		return &ObjectNotFoundError{"PartitionKey"}
	}
	jsPartitionKey, b := goja.AssertFunction(v)
	if !b {
		return &NotAFunctionError{"PartitionKey"}
	}
	e.jsPartitionKey = jsPartitionKey
	return nil
}

func (e *Environment) setFilterMessagesFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("FilterMessages")
	if v == nil {
		return &ObjectNotFoundError{"FilterMessages"}
	}
	jsFilter, b := goja.AssertFunction(v)
	if !b {
		return &NotAFunctionError{"FilterMessages"}
	}
	e.jsFilterMessages = jsFilter
	return nil
}

func (e *Environment) Topic(m model.SyslogMessage) (topic string, errs []error) {
	var err error
	errs = []error{}

	if e.jsTopic != nil {
		var jsMessage goja.Value
		var jsTopic goja.Value
		jsMessage, err = e.toJsMessage(m)
		if err == nil {
			jsTopic, err = e.jsTopic(nil, jsMessage)
			if err == nil {
				topic = jsTopic.String()
			} else {
				errs = append(errs, ExecutingJSErrorFactory(err, "Topic"))
			}
		} else {
			errs = append(errs, &ConversionGoJsError{ExecutingJSErrorFactory(err, "NewSyslogMessage")})
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
		if !model.TopicNameIsValid(topic) {
			errs = append(errs, &model.InvalidTopic{Topic: topic})
			return "", errs
		}
	}
	return topic, errs
}

func (e *Environment) PartitionKey(m model.SyslogMessage) (partitionKey string, errs []error) {
	errs = []error{}
	var err error
	var jsMessage goja.Value
	var jsPartitionKey goja.Value

	if e.jsPartitionKey != nil {
		jsMessage, err = e.toJsMessage(m)
		if err == nil {
			jsPartitionKey, err = e.jsPartitionKey(nil, jsMessage)
			if err == nil {
				partitionKey = jsPartitionKey.String()
			} else {
				errs = append(errs, ExecutingJSErrorFactory(err, "PartitionKey"))
			}
		} else {
			errs = append(errs, &ConversionGoJsError{ExecutingJSErrorFactory(err, "NewSyslogMessage")})
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
	return partitionKey, errs
}

func (e *Environment) FilterMessage(m *model.SyslogMessage) (filterResult FilterResult, err error) {
	var jsMessage goja.Value
	var resJsMessage goja.Value
	var result model.SyslogMessage

	if e.jsFilterMessages == nil {
		return PASS, nil
	}
	if m == nil {
		return DROPPED, nil
	}
	jsMessage, err = e.toJsMessage(*m)
	if err != nil {
		return FILTER_ERROR, &ConversionGoJsError{ExecutingJSErrorFactory(err, "NewSyslogMessage")}
	}
	resJsMessage, err = e.jsFilterMessages(nil, jsMessage)
	if err != nil {
		return FILTER_ERROR, ExecutingJSErrorFactory(err, "FilterMessages")
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
			return FILTER_ERROR, &ConversionJsGoError{Err: err}
		}
		*m = result
		return PASS, nil

	default:
		return FILTER_ERROR, fmt.Errorf("JS filter function returned an invalid result: %d", int64(filterResult))
	}

}

func (e *Environment) toJsMessage(m model.SyslogMessage) (sm goja.Value, err error) {
	p := e.runtime.ToValue(int(m.Priority))
	f := e.runtime.ToValue(int(m.Facility))
	s := e.runtime.ToValue(int(m.Severity))
	v := e.runtime.ToValue(int(m.Version))
	timeg := e.runtime.ToValue(m.TimeGenerated / 1000000)
	timer := e.runtime.ToValue(m.TimeReported / 1000000)
	host := e.runtime.ToValue(m.Hostname)
	app := e.runtime.ToValue(m.Appname)
	proc := e.runtime.ToValue(m.Procid)
	msgid := e.runtime.ToValue(m.Msgid)
	structured := e.runtime.ToValue(m.Structured)
	msg := e.runtime.ToValue(m.Message)
	props := e.runtime.ToValue(m.Properties)

	subs := make([]string, 0, len(m.AuditSubMessages))
	for _, sub := range m.AuditSubMessages {
		subs = append(subs, fmt.Sprintf("%d:%s", sub.Type, sub.Data))
	}
	jsSubs := e.runtime.ToValue(&subs)

	sm, err = e.jsNewSyslogMessage(nil, p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, jsSubs, props)
	if err != nil {
		return nil, err
	}
	return sm, nil
}

func (e *Environment) fromJsMessage(sm goja.Value) (m model.SyslogMessage, err error) {
	if goja.IsUndefined(sm) {
		return m, fmt.Errorf("The JS syslog message is 'undefined'")
	}
	var smToGo goja.Value
	smToGo, err = e.jsSyslogMessageToGo(nil, sm)
	if err != nil {
		return m, ExecutingJSErrorFactory(err, "SyslogMessageToGo")
	}
	imsg := iSyslogMessage{}
	err = e.runtime.ExportTo(smToGo, &imsg)
	if err != nil {
		return m, err
	}

	var parts []string
	var t int
	var sub string
	subMessages := make([]model.AuditSubMessage, 0, len(imsg.SubMessages))

	for _, sub = range imsg.SubMessages {
		parts = strings.SplitN(sub, ":", 2)
		if len(parts) == 2 {
			t, err = strconv.Atoi(parts[0])
			if err == nil {
				subMessages = append(subMessages, model.AuditSubMessage{Data: parts[1], Type: uint16(t)})
			}
		}
	}

	return model.SyslogMessage{
		Priority:         model.Priority(imsg.Priority),
		Facility:         model.Facility(imsg.Facility),
		Severity:         model.Severity(imsg.Severity),
		Version:          model.Version(imsg.Version),
		TimeGenerated:    imsg.TimeGenerated * 1000000,
		TimeReported:     imsg.TimeReported * 1000000,
		Hostname:         imsg.Hostname,
		Appname:          imsg.Appname,
		Procid:           imsg.Procid,
		Msgid:            imsg.Msgid,
		Structured:       imsg.Structured,
		Message:          imsg.Message,
		AuditSubMessages: subMessages,
		Properties:       imsg.Properties,
	}, nil
}
