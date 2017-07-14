// +build linux

package auditlogs

import (
	"encoding/json"
	"io"
)

type AuditWriter struct {
	e *json.Encoder
	w io.Writer
}

func NewAuditWriter(w io.Writer) *AuditWriter {
	return &AuditWriter{
		e: json.NewEncoder(w),
		w: w,
	}
}

func (a *AuditWriter) Write(msg *AuditMessageGroup) {
	err := a.e.Encode(msg)
	if err != nil {
		a.e = json.NewEncoder(a.w)
	}
}
