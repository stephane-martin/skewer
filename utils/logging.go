package utils

import (
	"context"
	"encoding/gob"
	"fmt"
	"log/syslog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
)

const timeKey = "t"
const lvlKey = "lvl"
const msgKey = "msg"

func receive(ctx context.Context, l log15.Logger, c net.Conn) {
	keyNames := log15.RecordKeyNames{
		Time: timeKey,
		Msg:  msgKey,
		Lvl:  lvlKey,
	}
	decoder := gob.NewDecoder(c)
	h := l.GetHandler()
	for {
		select {
		case <-ctx.Done():
			break
		default:
			r := Record{}
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			e := decoder.Decode(&r)
			if e == nil {
				logr := log15.Record{Lvl: log15.Lvl(r.Lvl), Msg: r.Msg, Time: r.Time, KeyNames: keyNames}
				logr.Ctx = make([]interface{}, 0, 2*len(r.Ctx))
				for k, v := range r.Ctx {
					logr.Ctx = append(logr.Ctx, k)
					logr.Ctx = append(logr.Ctx, v)
				}
				h.Log(&logr)
			}

		}
	}
}

func LogReceiver(ctx context.Context, l log15.Logger, connections []net.Conn) {
	for _, c := range connections {
		go receive(ctx, l, c)
	}
}

type Record struct {
	Time time.Time
	Lvl  int
	Msg  string
	Ctx  map[string]string
}

type RemoteLoggerHandler struct {
	remote  net.Conn
	encoder *gob.Encoder
	msgChan chan *log15.Record
	ctx     context.Context
}

func NewRemoteLogger(ctx context.Context, remote net.Conn) log15.Logger {
	logger := log15.New()
	h := RemoteLoggerHandler{remote: remote, encoder: gob.NewEncoder(remote), ctx: ctx}
	h.msgChan = make(chan *log15.Record, 1000)
	logger.SetHandler(&h)
	go func() {
		<-ctx.Done()
		c := h.msgChan
		h.msgChan = nil
		close(c)
		h.Close()
	}()
	go func() {
		var rbis Record
		for r := range h.msgChan {
			rbis = Record{Time: r.Time, Lvl: int(r.Lvl), Msg: r.Msg}
			rbis.Ctx = map[string]string{}

			l := len(r.Ctx)
			var i int
			var ok bool
			label := ""
			val := ""

			for i < l {
				label, ok = r.Ctx[i].(string)
				if ok {
					i++
					if i < l {
						val = formatValue(r.Ctx[i])
						rbis.Ctx[label] = val
						i++
					}
				}

			}
			h.encoder.Encode(rbis)
		}
	}()

	return logger
}

func (h *RemoteLoggerHandler) Log(r *log15.Record) error {
	select {
	case h.msgChan <- r:
	case <-h.ctx.Done():
	}
	return nil
}

func (h *RemoteLoggerHandler) Close() error {
	return h.remote.Close()
}

const timeFormat = "2006-01-02T15:04:05-0700"

func formatShared(value interface{}) (result interface{}) {
	switch v := value.(type) {
	case time.Time:
		return v.Format(timeFormat)

	case error:
		return v.Error()

	case fmt.Stringer:
		return v.String()

	default:
		return v
	}
}

func formatValue(value interface{}) string {
	value = formatShared(value)
	switch v := value.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', 3, 64)
	case float64:
		return strconv.FormatFloat(v, 'f', 3, 64)
	default:
		return fmt.Sprintf("%+v", value)
	}
}

func SetLogging(level string, logJson bool, logSyslog bool, filename string) log15.Logger {
	logger := log15.New()
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
