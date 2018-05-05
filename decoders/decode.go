package decoders

import (
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/zond/gotomic"
	"golang.org/x/text/encoding/unicode"
)

type BaseParser func([]byte) ([]*model.SyslogMessage, error)

var parsers = map[base.Format](func([]byte) ([]*model.SyslogMessage, error)){
	base.RFC5424:     p5424,
	base.RFC3164:     p3164,
	base.JSON:        pJSON,
	base.RsyslogJSON: pRsyslogJSON,
	base.GELF:        pGELF,
	base.InfluxDB:    pInflux,
	base.Protobuf:    pProtobuf,
	base.Collectd:    pCollectd,
	base.LTSV:        pLTSV,
	base.W3C:         nil,
}

type Parser interface {
	Parse(m []byte) ([]*model.SyslogMessage, error)
	Release()
}

// nativeParser is a decoder implemented as golang function
type nativeParser struct {
	baseParser func([]byte) ([]*model.SyslogMessage, error)
}

func (p *nativeParser) Release() {}

func (p *nativeParser) Parse(m []byte) ([]*model.SyslogMessage, error) {
	return p.baseParser(m)
}

// jsParser is a decoder implemented as a JS function
type jsParser struct {
	baseParser func([]byte) ([]*model.SyslogMessage, error)
	jsEnv      *javascript.Environment
	parsersEnv *ParsersEnv
}

func (p *jsParser) Release() {
	p.parsersEnv.jsEnvsPool.Put(p.jsEnv)
}

func (p *jsParser) Parse(m []byte) ([]*model.SyslogMessage, error) {
	return p.baseParser(m)
}

// ParsersEnv encapsulates JS and Golang parsers.
type ParsersEnv struct {
	sync.Mutex
	parserCache *gotomic.Hash
	jsFuncs     map[string]string
	jsEnvsPool  *sync.Pool
	logger      log15.Logger
}

func NewParsersEnv(config []conf.ParserConfig, logger log15.Logger) *ParsersEnv {
	env := ParsersEnv{
		jsFuncs:     make(map[string]string, len(config)),
		logger:      logger,
		parserCache: gotomic.NewHash(),
	}
	for _, c := range config {
		env.jsFuncs[c.Name] = c.Func
	}
	env.jsEnvsPool = &sync.Pool{New: env.newJSEnv}
	return &env
}

func (e *ParsersEnv) newJSEnv() interface{} {
	jsEnv := javascript.NewParsersEnvironment(e.logger)
	var jsFuncName, jsFuncBody string
	var err error
	for jsFuncName, jsFuncBody = range e.jsFuncs {
		err = jsEnv.AddParser(jsFuncName, jsFuncBody)
		if err != nil {
			e.logger.Warn("Error initializing parser", "name", jsFuncName, "error", err)
		}
	}
	return jsEnv
}

func (e *ParsersEnv) getJSEnv() *javascript.Environment {
	return e.jsEnvsPool.Get().(*javascript.Environment)
}

func (e *ParsersEnv) GetParser(c *conf.DecoderBaseConfig) (p Parser, err error) {
	frmt := base.ParseFormat(c.Format)
	if frmt == -1 {
		// look for a JS function
		return e.getJSParser(c.Format)
	}
	// casual parser
	return e.getNonJSParser(frmt, c)
}

func (e *ParsersEnv) getJSParser(funcName string) (*jsParser, error) {
	if _, ok := e.jsFuncs[funcName]; !ok {
		return nil, DecodingError(eerrors.Errorf("Unknown decoder: '%s'", funcName))
	}
	jsEnv := e.getJSEnv()
	baseParser, err := jsEnv.GetParser(funcName)
	if err != nil {
		return nil, err
	}
	return &jsParser{
		baseParser: baseParser,
		jsEnv:      jsEnv,
		parsersEnv: e,
	}, nil
}

func (e *ParsersEnv) getNonJSParser(frmt base.Format, c *conf.DecoderBaseConfig) (*nativeParser, error) {
	// some parsers may be heavy to build, so we cache them
	var p func([]byte) ([]*model.SyslogMessage, error)
	if thing, have := e.parserCache.Get(c); have {
		// fast path: we have already built such a parser
		return &nativeParser{baseParser: thing.(func([]byte) ([]*model.SyslogMessage, error))}, nil
	}
	// slow path
	e.Lock()
	defer e.Unlock()
	if frmt == base.W3C {
		// W3C parsed is parametrized
		if len(c.W3CFields) == 0 {
			return nil, DecodingError(eerrors.New("No fields specified for W3C Extended Log Format decoder"))
		}
		p = W3CDecoder(c.W3CFields)
	} else {
		p = parsers[frmt]
	}
	// add a decoding step to deal with charsets
	p = parserWithEncoding(frmt, c.Charset, p)
	// now the parser has been built. cache it so that we don't have to build it again later.
	// we assume that the parser func is "pure" and "thread-safe".
	e.parserCache.Put(c, p)
	return &nativeParser{baseParser: p}, nil
}

func parserWithEncoding(frmt base.Format, charset string, p func([]byte) ([]*model.SyslogMessage, error)) func([]byte) ([]*model.SyslogMessage, error) {
	switch frmt {
	case base.RFC3164, base.RFC5424, base.W3C:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = utils.SelectDecoder(charset).Bytes(m)
			if err != nil {
				return nil, InvalidCharsetError(err)
			}
			return p(m)
		}
	case base.JSON, base.RsyslogJSON, base.GELF, base.InfluxDB, -1:
		return func(m []byte) ([]*model.SyslogMessage, error) {
			var err error
			m, err = unicode.UTF8.NewDecoder().Bytes(m)
			if err != nil {
				return nil, InvalidCharsetError(err)
			}
			return p(m)
		}
	case base.Protobuf, base.Collectd:
		return p
	default:
		return p
	}
}
