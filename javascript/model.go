package javascript

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/dop251/goja"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/model"
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

type ISyslogMessage struct {
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
	Properties    map[string]interface{}
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

type Parser struct {
	e    *Environment
	Name string
}

type JSParsingError struct {
	Message    string
	ParserName string
}

func (e *JSParsingError) Error() string {
	return fmt.Sprintf("The provided JS parser could not parse the raw message: %s", e.Message)
}

func (p *Parser) Parse(rawMessage string, dont_parse_sd bool) (*model.SyslogMessage, error) {
	jsParser, ok := p.e.jsParsers[p.Name]
	if !ok {
		return nil, fmt.Errorf("Parser was not found: '%s'", p.Name)
	}
	rawMessage = strings.Trim(rawMessage, "\r\n ")
	if len(rawMessage) == 0 {
		return nil, nil
	}
	jsRawMessage := p.e.runtime.ToValue(rawMessage)
	jsParsedMessage, err := jsParser(nil, jsRawMessage)
	if err != nil {
		if jserr, ok := err.(*goja.Exception); ok {
			message, ok := jserr.Value().Export().(string)
			if ok {
				return nil, &JSParsingError{ParserName: p.Name, Message: message}
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	parsedMessage, err := p.e.FromJsSyslogMessage(jsParsedMessage)
	if err != nil {
		return nil, err
	}
	return parsedMessage, nil
}

func New(
	filterFunc string,
	topicFunc string,
	topicTmpl *template.Template,
	partitionKeyFunc string,
	partitionKeyTmpl *template.Template,
	logger log15.Logger,
) *Environment {

	e := Environment{topicTmpl: topicTmpl, partitionKeyTmpl: partitionKeyTmpl}
	e.logger = logger.New("class", "Environment")
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
		err := e.SetTopicFunc(topicFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS Topic() func", "error", err)
		}
	}
	if len(partitionKeyFunc) > 0 {
		err := e.SetPartitionKeyFunc(partitionKeyFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS PartitionKey() func", "error", err)
		}
	}
	if len(filterFunc) > 0 {
		err := e.SetFilterMessagesFunc(filterFunc)
		if err != nil {
			e.logger.Warn("Error setting the JS Filter() func", "error", err)
		}
	}
	return &e
}

func (e *Environment) GetParser(name string) *Parser {
	_, ok := e.jsParsers[name]
	if !ok {
		return nil
	}
	return &Parser{e: e, Name: name}
}

func (e *Environment) AddParser(name string, parserFunc string) error {
	name = strings.TrimSpace(name)
	parserFunc = strings.TrimSpace(parserFunc)
	if len(name) == 0 {
		return fmt.Errorf("Empty name")
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
		return fmt.Errorf("Parser func was not found")
	}
	parser, b := goja.AssertFunction(v)
	if !b {
		return fmt.Errorf("Parser is not a JS function")
	}
	e.jsParsers[name] = parser
	return nil
}

func (e *Environment) SetTopicFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("Topic")
	if v == nil {
		return fmt.Errorf("Topic function was not found")
	}
	jsTopic, b := goja.AssertFunction(v)
	if !b {
		return fmt.Errorf("Topic is not a JS function")
	}
	e.jsTopic = jsTopic
	return nil
}

func (e *Environment) SetPartitionKeyFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("PartitionKey")
	if v == nil {
		return fmt.Errorf("PartitionKey function was not found")
	}
	jsPartitionKey, b := goja.AssertFunction(v)
	if !b {
		return fmt.Errorf("PartitionKey is not a JS function")
	}
	e.jsPartitionKey = jsPartitionKey
	return nil
}

func (e *Environment) SetFilterMessagesFunc(f string) error {
	_, err := e.runtime.RunString(f)
	if err != nil {
		return err
	}
	v := e.runtime.Get("FilterMessages")
	if v == nil {
		return fmt.Errorf("FilterMessages function was not found")
	}
	jsFilter, b := goja.AssertFunction(v)
	if !b {
		return fmt.Errorf("FilterMessages is not a JS function")
	}
	e.jsFilterMessages = jsFilter
	return nil
}

func (e *Environment) Topic(m *model.SyslogMessage) string {
	topic := ""
	if e.jsTopic != nil && m != nil {
		jsMessage, err := e.ToJsSyslogMessage(m)
		if err == nil {
			jsTopic, err := e.jsTopic(nil, jsMessage)
			if err == nil {
				topic = jsTopic.String()
			} else {
				e.logger.Warn("Error calculating Topic()", "error", err)
			}
		} else {
			e.logger.Warn("Error converting the syslog message to a JS object", "error", err)
		}
	}
	if len(topic) == 0 && e.topicTmpl != nil {
		topicBuf := bytes.Buffer{}
		err := e.topicTmpl.Execute(&topicBuf, m)
		if err == nil {
			topic = topicBuf.String()
		} else {
			e.logger.Warn("Error executing the topic template", "error", err)
		}
	}
	if len(topic) > 0 {
		if !model.TopicNameIsValid(topic) {
			e.logger.Warn("The calculated topic is invalid", "topic", topic)
			topic = ""
		}
	}
	return topic
}

func (e *Environment) PartitionKey(m *model.SyslogMessage) string {
	partitionKey := ""
	if e.jsPartitionKey != nil && m != nil {
		jsMessage, err := e.ToJsSyslogMessage(m)
		if err == nil {
			jsPartitionKey, err := e.jsPartitionKey(nil, jsMessage)
			if err == nil {
				partitionKey = jsPartitionKey.String()
			} else {
				e.logger.Warn("Error calculating PartitionKey()", "error", err)
			}
		} else {
			e.logger.Warn("Error converting the syslog message to a JS object", "error", err)
		}
	}
	if len(partitionKey) == 0 && e.partitionKeyTmpl != nil {
		partitionKeyBuf := bytes.Buffer{}
		err := e.partitionKeyTmpl.Execute(&partitionKeyBuf, m)
		if err == nil {
			partitionKey = partitionKeyBuf.String()
		} else {
			e.logger.Warn("Error executing the PartitionKey template", "error", err)
		}
	}
	return partitionKey
}

func (e *Environment) FilterMessage(m *model.SyslogMessage) (result *model.SyslogMessage, filterResult FilterResult) {
	var jsMessage goja.Value
	var resJsMessage goja.Value
	var err error

	if e.jsFilterMessages == nil {
		return m, PASS
	}
	if m == nil {
		return nil, DROPPED
	}
	jsMessage, err = e.ToJsSyslogMessage(m)
	if err != nil {
		e.logger.Warn("Error converting the syslog message to a JS object", "error", err)
		return nil, FILTER_ERROR
	}
	resJsMessage, err = e.jsFilterMessages(nil, jsMessage)
	_, err = e.jsFilterMessages(nil, jsMessage)
	if err != nil {
		e.logger.Warn("Error filtering the syslog message", "error", err)
		return nil, FILTER_ERROR
	}

	filterResult = FilterResult(resJsMessage.ToInteger())
	switch filterResult {
	case DROPPED:
		return nil, DROPPED
	case REJECTED:
		return nil, REJECTED
	case FILTER_ERROR:
		return nil, FILTER_ERROR
	case PASS:
		result, err = e.FromJsSyslogMessage(jsMessage)
		if err != nil {
			e.logger.Warn("Error converting back the syslog message from JS", "error", err)
			return nil, FILTER_ERROR
		}
		return result, PASS

	default:
		e.logger.Warn("JS filter function returned an invalid result", "result", int64(filterResult))
		return nil, FILTER_ERROR
	}

}

func (e *Environment) ToJsSyslogMessage(m *model.SyslogMessage) (sm goja.Value, err error) {
	p := e.runtime.ToValue(int(m.Priority))
	f := e.runtime.ToValue(int(m.Facility))
	s := e.runtime.ToValue(int(m.Severity))
	v := e.runtime.ToValue(int(m.Version))
	timeg := e.runtime.ToValue(m.TimeGenerated.UnixNano() / 1000000)
	timer := e.runtime.ToValue(m.TimeReported.UnixNano() / 1000000)
	host := e.runtime.ToValue(m.Hostname)
	app := e.runtime.ToValue(m.Appname)
	proc := e.runtime.ToValue(m.Procid)
	msgid := e.runtime.ToValue(m.Msgid)
	structured := e.runtime.ToValue(m.Structured)
	msg := e.runtime.ToValue(m.Message)
	props := e.runtime.ToValue(m.Properties)

	sm, err = e.jsNewSyslogMessage(nil, p, f, s, v, timer, timeg, host, app, proc, msgid, structured, msg, props)
	if err != nil {
		return nil, err
	}
	return sm, nil
}

func (e *Environment) FromJsSyslogMessage(sm goja.Value) (m *model.SyslogMessage, err error) {
	if goja.IsUndefined(sm) {
		return nil, fmt.Errorf("Undefined goja value")
	}
	var smToGo goja.Value
	smToGo, err = e.jsSyslogMessageToGo(nil, sm)
	if err != nil {
		return nil, err
	}
	imsg := ISyslogMessage{}
	err = e.runtime.ExportTo(smToGo, &imsg)
	if err != nil {
		return nil, err
	}
	msg := model.SyslogMessage{
		Priority:      model.Priority(imsg.Priority),
		Facility:      model.Facility(imsg.Facility),
		Severity:      model.Severity(imsg.Severity),
		Version:       model.Version(imsg.Version),
		TimeGenerated: time.Unix(0, imsg.TimeGenerated*1000000),
		TimeReported:  time.Unix(0, imsg.TimeReported*1000000),
		Hostname:      imsg.Hostname,
		Appname:       imsg.Appname,
		Procid:        imsg.Procid,
		Msgid:         imsg.Msgid,
		Structured:    imsg.Structured,
		Message:       imsg.Message,
		Properties:    imsg.Properties,
	}
	return &msg, nil
}
