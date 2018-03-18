package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

// Line represents a parsed log line
type Line struct {
	// Fields stores the individual fields of the log line.
	Fields map[string]interface{}
}

func newLine() (l Line) {
	l.Fields = make(map[string]interface{})
	return l
}

func (l *Line) add(key string, value string) {
	// guess the real type of value
	v := ConvertValue(key, value)
	if v != nil {
		if s, ok := v.(string); ok {
			if len(s) > 0 {
				l.Fields[key] = s
			}
		} else {
			l.Fields[key] = v
		}
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (l *Line) MarshalJSON() ([]byte, error) {
	newFields := make(map[string]interface{}, len(l.Fields)+1)
	for k, v := range l.Fields {
		newFields[k] = v
	}
	t := l.GetTime()
	if !t.IsZero() {
		newFields["@timestamp"] = t
	}
	return json.Marshal(newFields)
}

// GetTime returns the log line timestamp.
// It returns the time.Time zero value if the timestamp can not be found.
func (l *Line) GetTime() time.Time {
	if l.Fields["gmttime"] != nil {
		return l.Fields["gmttime"].(time.Time)
	}
	if l.Fields["time"] != nil && l.Fields["date"] != nil {
		t := l.Fields["time"].(Time)
		d := l.Fields["date"].(Date)
		return DateTime{Date: d, Time: t}.In(time.UTC)
	}
	if l.Fields["localtime"] != nil {
		return l.Fields["localtime"].(time.Time)
	}
	return time.Time{}
}

// GetProperties returns the log line fieds as a map[string]string.
func (l *Line) GetProperties() map[string]string {
	if len(l.Fields) == 0 {
		return nil
	}
	m := make(map[string]string, len(l.Fields))
	var k string
	var v interface{}
	for k, v = range l.Fields {
		m[k] = fmt.Sprintf("%v", v)
	}
	return m
}
