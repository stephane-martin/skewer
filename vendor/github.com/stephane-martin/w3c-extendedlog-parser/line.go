package parser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Line represents a parsed log line
type Line struct {
	// fields stores the individual fields of the log line.
	fields map[string]interface{}
	names  []string
}

func NewLine(names []string) (l *Line) {
	l = &Line{}
	l.Reset(names)
	return l
}

func (l *Line) Reset(names []string) {
	l.names = names
	if l.fields == nil {
		l.fields = make(map[string]interface{}, len(l.names))
	} else {
		for k := range l.fields {
			delete(l.fields, k)
		}
	}
	for _, name := range l.names {
		l.fields[name] = nil
	}
}

func (l *Line) Names() (ret []string) {
	// we return a copy of internal names
	ret = make([]string, 0, len(l.names))
	for _, name := range l.names {
		ret = append(ret, name)
	}
	return ret
}

func (l *Line) Fields() (ret []interface{}) {
	ret = make([]interface{}, 0, len(l.names))
	for _, name := range l.names {
		ret = append(ret, l.Get(name))
	}
	return ret
}

func (l *Line) add(key string, value string) {
	if _, ok := l.fields[key]; !ok {
		return
	}
	// guess the real type of value
	v := ConvertValue(key, value)
	if v != nil {
		if s, ok := v.(string); ok {
			if len(s) > 0 {
				l.fields[key] = s
			}
		} else {
			l.fields[key] = v
		}
	}
}

func (l *Line) Get(key string) interface{} {
	if key == "gmttime" {
		return l.GetTime()
	}
	return l.fields[key]
}

func itostr(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func atostr(s []interface{}) (ret []string) {
	ret = make([]string, 0, len(s))
	for _, v := range s {
		ret = append(ret, itostr(v))
	}
	return ret
}

func (l *Line) GetAsString(key string) string {
	return itostr(l.fields[key])
}

func (l *Line) GetAll() map[string]interface{} {
	newFields := make(map[string]interface{}, len(l.fields)+1)
	for k, v := range l.fields {
		if v != nil {
			newFields[k] = v
		}
	}
	t := l.GetTime()
	if !t.IsZero() {
		newFields["@timestamp"] = t
	}
	return newFields
}

// MarshalJSON implements the json.Marshaler interface.
func (l *Line) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.GetAll())
}

// WriteTo writes the line to the given writer
func (l *Line) WriteTo(w io.Writer, jsonExport bool) (err error) {
	if jsonExport {
		return json.NewEncoder(w).Encode(l)
	}
	csvWriter := csv.NewWriter(w)
	err = csvWriter.Write(atostr(l.Fields()))
	csvWriter.Flush()
	return err
}

// GetTime returns the log line timestamp.
// It returns the time.Time zero value if the timestamp can not be found.
func (l *Line) GetTime() time.Time {
	if l.fields["gmttime"] != nil {
		return l.fields["gmttime"].(time.Time)
	}
	if l.fields["time"] != nil && l.fields["date"] != nil {
		t := l.fields["time"].(Time)
		d := l.fields["date"].(Date)
		return DateTime{Date: d, Time: t}.In(time.UTC)
	}
	if l.fields["localtime"] != nil {
		return l.fields["localtime"].(time.Time)
	}
	return time.Time{}
}

func (l *Line) GetDate() (d Date) {
	if l.fields["date"] != nil {
		return l.fields["date"].(Date)
	}
	t := l.GetTime()
	if !t.IsZero() {
		year, month, day := t.Date()
		return Date{Day: day, Month: month, Year: year}
	}
	return d
}
