package logging

import (
	"log/syslog"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

const timeKey = "t"
const lvlKey = "lvl"
const msgKey = "msg"

func SetupLogging(logger log15.Logger, level string, logJson bool, logSyslog bool, filename string) (log15.Logger, error) {
	if logger == nil {
		logger = log15.New()
	}
	handlers := []log15.Handler{}
	var formatter log15.Format
	if logJson {
		formatter = log15.JsonFormat()
	} else {
		formatter = log15.LogfmtFormat()
	}
	if logSyslog {
		h, err := log15.SyslogHandler(syslog.LOG_LOCAL0|syslog.LOG_DEBUG, "skewer", formatter)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error opening syslog")
		}
		handlers = append(handlers, h)
	}
	filename = strings.TrimSpace(filename)
	if len(filename) > 0 {
		h, err := log15.FileHandler(filename, formatter)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error opening log file")
		}
		handlers = append(handlers, h)
	}
	if len(handlers) == 0 {
		handlers = []log15.Handler{log15.StderrHandler}
	}
	handler := log15.MultiHandler(handlers...)

	lvl, e := log15.LvlFromString(level)
	if e != nil {
		lvl = log15.LvlInfo
	}
	handler = log15.LvlFilterHandler(lvl, handler)

	logger.SetHandler(handler)
	return logger, nil
}
