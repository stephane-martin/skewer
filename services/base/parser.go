package base

import (
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model/decoders"
)

type ParsersEnv struct {
	jsenv javascript.ParsersEnvironment
}

func NewParsersEnv(parsersConf []conf.ParserConfig, logger log15.Logger) *ParsersEnv {
	jsenv := javascript.NewParsersEnvironment(logger)
	for _, parserConf := range parsersConf {
		err := jsenv.AddParser(parserConf.Name, parserConf.Func)
		if err != nil {
			logger.Warn("Error initializing parser", "name", parserConf.Name, "error", err)
		}
	}
	return &ParsersEnv{jsenv: jsenv}
}

func (e *ParsersEnv) GetParser(parserName string) (decoders.Parser, error) {
	frmt := decoders.ParseFormat(parserName)
	if frmt != -1 {
		return decoders.GetParser(frmt)
	}
	return e.jsenv.GetParser(parserName)
}
