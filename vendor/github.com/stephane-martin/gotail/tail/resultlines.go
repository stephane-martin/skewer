package tail

import (
	"bytes"
	"context"
)

type resultLines struct {
	output chan []byte
	buf    *bytes.Buffer
}

func copybuf(buf *bytes.Buffer) (res []byte) {
	l := buf.Len()
	if l == 0 {
		return nil
	}
	res = make([]byte, l)
	copy(res, buf.Bytes())
	buf.Reset()
	return res
}

func makeWriter(ctx context.Context, results chan []byte) (w *resultLines) {
	w = &resultLines{
		output: make(chan []byte),
		buf:    bytes.NewBuffer(nil),
	}
	removeNLChans(ctx, w.output, results)
	return w
}

func (r *resultLines) Close() {
	rest := bytes.Trim(copybuf(r.buf), lineEndString)
	if len(rest) > 0 {
		r.output <- rest
	}
	close(r.output)
}

func (r *resultLines) Write(p []byte) (int, error) {
	lorig := len(p)
	if r == nil {
		return lorig, nil
	}
	if r.output == nil {
		return lorig, nil
	}
	if lorig == 0 {
		return 0, nil
	}
	var l int
	for {
		l = len(p)
		if l == 0 {
			return lorig, nil
		}
		idx := bytes.Index(p, lineEndS)
		if idx == -1 {
			r.buf.Write(p)
			return lorig, nil
		}
		r.buf.Write(p[0 : idx+1])
		r.output <- copybuf(r.buf)
		if idx == l-1 {
			return lorig, nil
		}
		p = p[idx+1:]
	}
}
