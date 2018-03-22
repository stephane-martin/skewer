package decoders

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/grammars/rfc5424"
	"github.com/stephane-martin/skewer/model"
)

var parser5424Pool *sync.Pool

func init() {
	parser5424Pool = &sync.Pool{
		New: func() interface{} {
			return rfc5424.NewRFC5424Parser(nil)
		},
	}
}

func p5424(m []byte) ([]*model.SyslogMessage, error) {
	// TODO: multiple messages ?
	parser := parser5424Pool.Get().(*rfc5424.RFC5424Parser)
	defer parser5424Pool.Put(parser)

	lexer := rfc5424.NewRFC5424Lexer(antlr.NewInputStream(string(m)))
	parser.SetInputStream(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))

	parser.RemoveErrorListeners()
	errListner := newErrorListener()
	parser.AddErrorListener(errListner)
	//parser.AddErrorListener(antlr.NewDiagnosticErrorListener(false))
	parser.SetErrorHandler(newErrorStrategy())
	parser.BuildParseTrees = true
	parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)
	listnr := newListener()
	antlr.ParseTreeWalkerDefault.Walk(listnr, parser.Full())

	err := errListner.Err()
	if err == nil {
		err = listnr.Err()
	}
	if err != nil {
		return nil, err
	}
	return []*model.SyslogMessage{listnr.GetMessage()}, nil
}

type errorStrategy struct {
	*antlr.DefaultErrorStrategy
}

func newErrorStrategy() *errorStrategy {
	return &errorStrategy{
		DefaultErrorStrategy: antlr.NewDefaultErrorStrategy(),
	}
}

func (s *errorStrategy) Sync(antlr.Parser) {

}

type errorListener struct {
	*antlr.DefaultErrorListener
	err    error
	line   int
	column int
}

func newErrorListener() *errorListener {
	return &errorListener{
		DefaultErrorListener: antlr.NewDefaultErrorListener(),
	}
}

func (el *errorListener) SyntaxError(rec antlr.Recognizer, offendingSymbol interface{}, line, column int, errmsg string, e antlr.RecognitionException) {
	if len(errmsg) > 0 && el.err == nil {
		el.err = fmt.Errorf(errmsg)
		el.line = line
		el.column = column
	}
}

func (el *errorListener) Err() error {
	return el.err
}

type listener struct {
	*rfc5424.BaseRFC5424Listener
	msg        *model.SyslogMessage
	err        error
	currentSID string
}

func newListener() *listener {
	return &listener{
		BaseRFC5424Listener: &rfc5424.BaseRFC5424Listener{},
		msg:                 model.CleanFactory(),
	}
}

func (l *listener) GetMessage() *model.SyslogMessage {
	return l.msg
}

func (l *listener) ExitPri(ctx *rfc5424.PriContext) {
	if l.err != nil || ctx == nil {
		return
	}
	pri, err := strconv.Atoi(ctx.GetText())
	if err != nil {
		l.setErr(new(base.InvalidPriorityError))
		return
	}
	l.msg.Priority = model.Priority(pri)
	l.msg.Facility = model.Facility(pri / 8)
	l.msg.Severity = model.Severity(pri % 8)
}

func (l *listener) ExitVersion(ctx *rfc5424.VersionContext) {
	if l.err != nil || ctx == nil {
		return
	}
	v, err := strconv.Atoi(ctx.GetText())
	if err != nil {
		l.setErr(new(base.InvalidPriorityError))
		return
	}
	l.msg.Version = model.Version(v)
}

func (l *listener) ExitTimestamp(ctx *rfc5424.TimestampContext) {
	if l.err != nil || ctx == nil {
		return
	}
	l.msg.TimeGeneratedNum = time.Now().UnixNano()
	t := ctx.GetText()
	if t == "-" {
		l.msg.TimeReportedNum = l.msg.TimeGeneratedNum
		return
	}
	rep, err := time.Parse(time.RFC3339, ctx.GetText())
	if err != nil {
		l.setErr(err)
		return
	}
	l.msg.TimeReportedNum = rep.UnixNano()
}

func (l *listener) ExitHostname(ctx *rfc5424.HostnameContext) {
	if l.err != nil || ctx == nil {
		return
	}
	h := ctx.GetText()
	if h != "-" {
		l.msg.HostName = h
	}
}

func (l *listener) ExitAppname(ctx *rfc5424.AppnameContext) {
	if l.err != nil || ctx == nil {
		return
	}
	app := ctx.GetText()
	if app != "-" {
		l.msg.AppName = app
	}
}

func (l *listener) ExitProcid(ctx *rfc5424.ProcidContext) {
	if l.err != nil || ctx == nil {
		return
	}
	proc := ctx.GetText()
	if proc != "-" {
		l.msg.ProcId = proc
	}
}

func (l *listener) ExitMsgid(ctx *rfc5424.MsgidContext) {
	if l.err != nil || ctx == nil {
		return
	}
	msgid := ctx.GetText()
	if msgid != "-" {
		l.msg.MsgId = msgid
	}
}

func (l *listener) ExitSid(ctx *rfc5424.SidContext) {
	if l.err != nil || ctx == nil {
		return
	}
	l.currentSID = ctx.GetText()
	if len(l.currentSID) == 0 {
		l.setErr(errors.New("Empty SDID"))
		return
	}
	l.msg.ClearDomain(l.currentSID)
}

func (l *listener) ExitParam(ctx *rfc5424.ParamContext) {
	if l.err != nil || ctx == nil {
		return
	}
	if len(l.currentSID) == 0 {
		l.setErr(errors.New("Empty SDID"))
		return
	}
	name := ctx.Name()
	value := ctx.Value()
	if name != nil {
		if value != nil {
			l.msg.SetProperty(l.currentSID, name.GetText(), value.GetText())
		} else {
			l.msg.SetProperty(l.currentSID, name.GetText(), "")
		}
	}
}

func (l *listener) ExitMsg(ctx *rfc5424.MsgContext) {
	if l.err != nil || ctx == nil {
		return
	}
	l.msg.Message = ctx.GetText()
}

func (l *listener) Err() error {
	return l.err
}

func (l *listener) setErr(err error) {
	if l.err == nil {
		l.err = err
	}
}
