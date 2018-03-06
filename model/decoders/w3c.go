package decoders

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/civil"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/text/encoding"
)

var QuoteLeftOpenError = errors.New("Unclosed quote in a W3C log line")
var EndLineInQuotesError = errors.New("Endline character appears in a quoted string")
var NoEndLineError = errors.New("No endline at end of input")

var wpool bytebufferpool.Pool

type W3CFieldType int

func guessType(fieldName string, value string) interface{} {
	switch fieldName {
	case "date", "x-cookie-date", "x-http-date":
		d, err := civil.ParseDate(value)
		if err != nil {
			return nil
		}
		return d
	case "time":
		t, err := civil.ParseTime(value)
		if err != nil {
			return nil
		}
		return t
	case "time-taken", "rs-time-taken", "sc-time-taken", "rs-service-time-taken", "rs-download-time-taken", "cs-categorization-time-dynamic":
		ttaken, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil
		}
		return ttaken
	case "bytes":
		return makeInt(value)
	case "cached":
		return value == "1"
	case "x-client-address", "x-bluecoat-appliance-primary-address", "x-bluecoat-proxy-primary-address", "cs-uri-address", "c-uri-address", "sr-uri-address", "s-uri-address", "x-cs-user-login-address":
		return net.ParseIP(value)
	case "connect-time", "dnslookup-time", "duration":
		return makeInt(value)
	case "gmttime":
		t, err := time.Parse("02/01/2006:15:04:05", value)
		if err != nil {
			return nil
		}
		return t.UTC()
	// TODO: fix
	//  case "localtime":
	//	t, err := time.Parse("02/Jan/2006:15:04:05 +nnnn", value)
	case "timestamp", "x-timestamp-unix", "x-timestamp-unix-utc":
		i := makeInt(value)
		if i != nil {
			return time.Unix(i.(int64), 0)
		}
		return nil
	default:
	}
	if strings.IndexByte(fieldName, '(') != -1 {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-ip") {
		return net.ParseIP(value)
	}
	if strings.HasSuffix(fieldName, "-dns") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-status") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-comment") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-method") {
		return makeStr(value)
	}
	if strings.HasSuffix(fieldName, "-uri") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-uri-stem") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-uri-query") {
		return decodeURI(value)
	}
	if strings.HasSuffix(fieldName, "-length") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-headerlength") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-bytes") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-written") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-read") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-operations") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-size") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-port") {
		return makeInt(value)
	}
	if strings.HasSuffix(fieldName, "-count") {
		return makeInt(value)
	}
	return makeStr(value)

}

func W3CDecoder(fields []string) Parser {
	// https://www.w3.org/TR/WD-logfile.html
	return func(m []byte, decoder *encoding.Decoder) ([]*model.SyslogMessage, error) {
		return nil, nil
	}
}

type W3CLine struct {
	Fields map[string]interface{}
}

func newLine() (l W3CLine) {
	l.Fields = make(map[string]interface{})
	return l
}

func (l *W3CLine) Add(key string, value string) {
	// guess the real type of value
	l.Fields[key] = guessType(key, value)
}

func (l *W3CLine) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Fields)
}

func (l *W3CLine) GetTime() time.Time {
	if l.Fields["gmttime"] != nil {
		return l.Fields["gmttime"].(time.Time)
	}
	if l.Fields["time"] != nil && l.Fields["date"] != nil {
		t := l.Fields["time"].(civil.Time)
		d := l.Fields["date"].(civil.Date)
		return civil.DateTime{Date: d, Time: t}.In(time.UTC)
	}
	return time.Time{}
}

func makeStr(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

func makeInt(s string) interface{} {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return i
}

func makeFloat(s string) interface{} {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return f
}

func decodeURI(s string) interface{} {
	uri, err := url.QueryUnescape(s)
	if err != nil {
		return nil
	}
	return uri
}

type W3CFileHeader struct {
	FieldNames []string
	Software   string
	Remark     string
	Meta       map[string]string
}

func w3cParseFileHeader(reader *bufio.Reader) (*W3CFileHeader, error) {
	h := new(W3CFileHeader)
	h.Meta = make(map[string]string)
	for {
		c, err := reader.Peek(1)
		if err != nil {
			return nil, err
		}
		if c[0] != '#' {
			break
		}
		metaline, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		metaline = strings.TrimSpace(metaline[1:])
		if len(metaline) > 0 {
			kv := strings.SplitN(metaline, ":", 2)
			if len(kv) == 2 {
				key := strings.ToLower(strings.TrimSpace(kv[0]))
				value := strings.TrimSpace(kv[1])
				switch key {
				case "software":
					h.Software = value
				case "remark":
					h.Remark = value
				case "fields":
					h.FieldNames = make([]string, 0)
					for _, f := range strings.Split(value, " ") {
						f = strings.ToLower(strings.TrimSpace(f))
						if len(f) > 0 {
							h.FieldNames = append(h.FieldNames, f)
						}
					}
				default:
					h.Meta[key] = value
				}
			}
		}
	}
	return h, nil
}

type W3CFileParser struct {
	W3CFileHeader
	reader  *bufio.Reader
	scanner *W3CScanner
}

func NewW3CFileParser(reader io.Reader) *W3CFileParser {
	var bufreader *bufio.Reader
	if r, ok := reader.(*bufio.Reader); ok {
		bufreader = r
	} else {
		bufreader = bufio.NewReader(reader)
	}
	parser := W3CFileParser{
		reader:  bufreader,
		scanner: NewW3CScanner(bufreader),
	}
	return &parser
}

func (p *W3CFileParser) ParseHeader() error {
	header, err := w3cParseFileHeader(p.reader)
	if err != nil {
		return err
	}
	p.W3CFileHeader = *header
	return nil
}

func (p *W3CFileParser) SetFieldNames(fieldNames []string) {
	p.W3CFileHeader.FieldNames = fieldNames
}

func (p *W3CFileParser) Next() (*W3CLine, error) {
	var name string
	var i int
	if p.scanner.Scan() {
		l := newLine()
		fields := p.scanner.Strings()
		if len(fields) != len(p.FieldNames) {
			return nil, fmt.Errorf("Wrong number of fields: expected = %d, actual = %d", len(p.FieldNames), len(fields))
		}
		for i, name = range p.FieldNames {
			l.Add(name, fields[i])
		}
		return &l, nil
	}
	return nil, p.scanner.Error()
}

type W3CScanner struct {
	reader  io.Reader
	strings []string
	done    bool
	buf     []byte
	origbuf []byte
	err     error
}

func NewW3CScanner(reader io.Reader) *W3CScanner {
	s := W3CScanner{
		reader:  reader,
		origbuf: make([]byte, 0, 4096),
	}
	s.buf = s.origbuf
	return &s
}

func (s *W3CScanner) Scan() bool {
	if s.done {
		return false
	}
	var err error
	var rest []byte
	var strings []string
	var n int
	for {
		if s.err != nil && s.err != io.EOF {
			return false
		}
		if len(s.buf) > 0 {
			// try to parse what we have in buf
			rest, strings, err = W3CExtractStrings(s.buf)
			if err == nil {
				if len(strings) > 0 {
					// we got a log line
					s.strings = strings
					s.buf = rest
					return true
				}
				// there was no content that could be extracted
				// so we need more data, just get rid of the useless spaces
				s.buf = rest
			} else if err != NoEndLineError && err != QuoteLeftOpenError {
				// parsing error
				s.err = err
				return false
			} else if s.err == io.EOF && err == NoEndLineError {
				// there is no more available data to read
				// just output the last content
				if len(strings) > 0 {
					s.strings = strings
					s.buf = rest
					return true
				}
				return false
			} else if s.err == io.EOF && err == QuoteLeftOpenError {
				// there is no more available data to read
				// but the last content is not valid
				s.err = err
				return false
			}
			// here, at the end of the if/elseif, we know that err is a
			// "incomplete line" error, and that we can try to read more data
		}
		// there was not enough data to generate new content
		if s.err != nil {
			return false
		}
		// if there is no more space on the right side of s.buf, or if there is
		// much space on the left side of s.buf, then copy the data to the
		// beginning of s.origbuf
		if cap(s.buf) < 4096 && (len(s.buf) == cap(s.buf) || cap(s.buf) < 2048) {
			copy(s.origbuf[:len(s.buf)], s.buf)
			s.buf = s.origbuf[:len(s.buf)]
		}
		if len(s.buf) == 4096 {
			// the line to parse is too long
			s.err = bufio.ErrTooLong
			return false
		}
		// read some more data into the free space on the right side of s.buf
		n, err = s.reader.Read(s.buf[len(s.buf):cap(s.buf)])
		if err == io.EOF && n > 0 {
			s.buf = s.buf[:len(s.buf)+n]
			s.err = io.EOF
		} else if err == io.EOF && n == 0 {
			s.err = io.EOF
			return false
		} else if err != nil {
			s.err = err
			return false
		} else if n == 0 {
			s.err = io.ErrNoProgress
			return false
		} else {
			s.buf = s.buf[:len(s.buf)+n]
		}
	}
}

func (s *W3CScanner) Strings() []string {
	return s.strings
}

func (s *W3CScanner) Error() error {
	return s.err
}

func W3CExtractStrings(input []byte) (rest []byte, fields []string, err error) {
	// get rid of superfluous spaces at the beginning of the input
	m := bytes.TrimLeft(input, "\r\n\t ")
	l := len(m)
	if l == 0 {
		// nothing to do...
		return nil, nil, nil
	}

	// we want to parse until some endline appears
	linelen := bytes.IndexAny(m, "\r\n")
	if linelen == -1 {
		linelen = len(m)
	}
	// how many spaces in that line?
	c := bytes.Count(m[:linelen], SP)
	// we assume that we have c+1 fields to extract
	buffers := make([]*bytebufferpool.ByteBuffer, 0, c+1)
	var curbuf *bytebufferpool.ByteBuffer
	var haveString bool
	var haveFirstChar bool
	var curchar byte
	var icur int

	w := func(buffers *[]*bytebufferpool.ByteBuffer, curbuf **bytebufferpool.ByteBuffer, b byte) {
		if *curbuf == nil {
			*curbuf = wpool.Get()
			*buffers = append(*buffers, *curbuf)
		}
		(*curbuf).WriteByte(b)
		haveFirstChar = true
	}

	for icur < l {
		curchar = m[icur]
		if isEndline(curchar) {
			// end of log line
			if haveString {
				// we should not meet an endline inside a quoted string
				return input, nil, EndLineInQuotesError
			}
			curbuf = nil
			// consume any superfluous spaces and lineends
			for icur < l && (isSpace(m[icur]) || isEndline(m[icur])) {
				icur++
			}
			if len(buffers) > 0 {
				// we have finished processing the current log line
				fields = make([]string, 0, len(buffers))
				for _, buf := range buffers {
					fields = append(fields, replace20(buf.String()))
					wpool.Put(buf)
				}
				return m[icur:], fields, nil
			}
			// if there was no content on that line, we just continue to consume
		} else if isSpace(curchar) {
			if haveString {
				// this a normal space inside a string
				w(&buffers, &curbuf, ' ')
				icur++
			} else {
				// this is a field separator, we should stop to add chars to
				// the current buffer
				curbuf = nil
				// consume superfluous spaces
				for icur < l && isSpace(m[icur]) {
					icur++
				}
			}
		} else if isQuote(curchar) {
			if haveString {
				if icur == (l - 1) {
					// last character, we are sure that it is not an escaped quote
					haveString = false
					icur++
				} else if m[icur+1] == '"' {
					// this is an escaped quote
					w(&buffers, &curbuf, '"')
					icur++
					icur++
				} else {
					// normal closing quote
					haveString = false
					icur++
				}
			} else {
				// opening quote
				haveString = true
				icur++
			}
		} else if isSharp(curchar) {
			if haveFirstChar {
				// we consider that this '#' character is just a part of a field
				w(&buffers, &curbuf, curchar)
				icur++
			} else {
				// we haven't see a field so far, so this '#' signals a comment line
				// so let's consume the comment line
				for ; icur < l; icur++ {
					if m[icur] == '\n' {
						break
					}
				}
				icur++ // consume the newline char
			}
		} else {
			w(&buffers, &curbuf, curchar)
			icur++
		}
	}

	if haveString {
		// quoted string has not been closed
		// it probably means that we need more content
		return input, nil, QuoteLeftOpenError
	}

	if len(buffers) == 0 {
		// no content
		return nil, nil, nil
	}
	fields = make([]string, 0, len(buffers))
	for _, buf := range buffers {
		fields = append(fields, replace20(buf.String()))
		wpool.Put(buf)
	}
	// we have reached the end of input, but no endline char was present
	// that may or may not be normal, so let's report it
	return nil, fields, NoEndLineError
}

func isEndline(b byte) bool {
	return b == '\r' || b == '\n'
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func isQuote(b byte) bool {
	return b == '"'
}

func isSharp(b byte) bool {
	return b == '#'
}

func replace20(s string) string {
	return strings.Replace(s, "%20", " ", -1)
}
