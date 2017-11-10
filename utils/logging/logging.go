package logging

import (
	"fmt"
	"log/syslog"
	"os"
	"strings"

	"github.com/inconshreveable/log15"
)

const timeKey = "t"
const lvlKey = "lvl"
const msgKey = "msg"

func SetLogging(logger log15.Logger, level string, logJson bool, logSyslog bool, filename string) log15.Logger {
	if logger == nil {
		logger = log15.New()
	}
	log_handlers := []log15.Handler{}
	var formatter log15.Format
	if logJson {
		formatter = log15.JsonFormat()
	} else {
		formatter = log15.LogfmtFormat()
	}
	if logSyslog {
		h, e := log15.SyslogHandler(syslog.LOG_LOCAL0|syslog.LOG_DEBUG, "skewer", formatter)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Error opening syslog file: %s\n", e)
		} else {
			log_handlers = append(log_handlers, h)
		}
	}
	filename = strings.TrimSpace(filename)
	if len(filename) > 0 {
		h, e := log15.FileHandler(filename, formatter)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file '%s': %s\n", filename, e)
		} else {
			log_handlers = append(log_handlers, h)
		}
	}
	if len(log_handlers) == 0 {
		log_handlers = []log15.Handler{log15.StderrHandler}
	}
	handler := log15.MultiHandler(log_handlers...)

	lvl, e := log15.LvlFromString(level)
	if e != nil {
		lvl = log15.LvlInfo
	}
	handler = log15.LvlFilterHandler(lvl, handler)

	logger.SetHandler(handler)
	return logger
}
