package parser

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
	"sync"
	"time"
)

// ErrQuoteLeftOpen is the error returned by ExtractStrings when the input
// shows an unclosed quoted string.
var ErrQuoteLeftOpen = errors.New("Unclosed quote in a W3C log line")

// ErrEndlineInsideQuotes is the error returned when the input shows an
// endline character inside a quoted string.
var ErrEndlineInsideQuotes = errors.New("Endline character appears in a quoted string")

// ErrNoEndline is the error returned by ExtractStrings when the input
// does not end with an endline character.
var ErrNoEndline = errors.New("No endline at end of input")

var sp = []byte(" ")

type buff struct {
	B []byte
}

func (b *buff) WriteByte(c byte) {
	b.B = append(b.B, c)
}

func (b *buff) String() string {
	return string(b.B)
}

type bPool struct {
	pool *sync.Pool
}

func (p *bPool) Get() *buff {
	b := p.pool.Get().(*buff)
	b.B = b.B[:0]
	return b
}

func (p *bPool) Put(b *buff) {
	if b != nil {
		p.pool.Put(b)
	}
}

var pool *bPool

func init() {
	pool = &bPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return &buff{
					B: make([]byte, 0, 256),
				}
			},
		},
	}
}

func guessType(fieldName string, value string) interface{} {
	value = strings.TrimSpace(value)
	switch fieldName {
	case "date", "x-cookie-date", "x-http-date":
		d, err := ParseDate(value)
		if err != nil {
			return nil
		}
		return d
	case "time":
		t, err := ParseTime(value)
		if err != nil {
			return nil
		}
		return t
	case "time-taken", "rs-time-taken", "sc-time-taken", "rs-service-time-taken", "rs-download-time-taken", "cs-categorization-time-dynamic", "duration":
		ttaken, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil
		}
		return ttaken
	case "bytes":
		return makeInt(value)
	case "cached":
		return value == "1"
	case "x-client-address", "x-bluecoat-appliance-primary-address", "x-bluecoat-proxy-primary-address", "cs-uri-address", "c-uri-address":
		return net.ParseIP(value)
	case "sr-uri-address", "s-uri-address", "x-cs-user-login-address":
		return net.ParseIP(value)
	case "connect-time", "dnslookup-time":
		return makeInt(value)
	case "gmttime":
		t, err := time.Parse("02/01/2006:15:04:05", value)
		if err != nil {
			return nil
		}
		return t.UTC()
	case "localtime":
		var t time.Time
		var err error
		pluspos := strings.Index(value, "+")
		if pluspos == -1 {
			t, err = time.Parse("02/Jan/2006:15:04:05", value)
		} else {
			t, err = time.Parse("02/Jan/2006:15:04:05 -0700", value)
		}
		if err != nil {
			return nil
		}
		return t.UTC()
	case "timestamp", "x-timestamp-unix", "x-timestamp-unix-utc":
		i := makeInt(value)
		if i != nil {
			return time.Unix(i.(int64), 0).UTC()
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
	l.Fields[key] = guessType(key, value)
}

// MarshalJSON implements the json.Marshaler interface.
func (l *Line) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Fields)
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

// FileHeader represents the header of a W3C Extended Log Format file.
type FileHeader struct {
	FieldNames []string
	Software   string
	Remark     string
	Meta       map[string]string
}

func parseFileHeader(reader *bufio.Reader) (*FileHeader, error) {
	h := new(FileHeader)
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

// FileParser is used to parse a W3C Extended Log Format file.
type FileParser struct {
	FileHeader
	reader  *bufio.Reader
	scanner *Scanner
}

// NewFileParser constructs a FileParser
func NewFileParser(reader io.Reader) *FileParser {
	var bufreader *bufio.Reader
	if r, ok := reader.(*bufio.Reader); ok {
		bufreader = r
	} else {
		bufreader = bufio.NewReader(reader)
	}
	parser := FileParser{
		reader:  bufreader,
		scanner: NewScanner(bufreader),
	}
	return &parser
}

// ParseHeader is used to parse the header part of a W3C Extended Log Format file.
// The io.Reader should be at the start of the file.
func (p *FileParser) ParseHeader() error {
	header, err := parseFileHeader(p.reader)
	if err != nil {
		return err
	}
	p.FileHeader = *header
	return nil
}

// SetFieldNames can be used to set the Field names manually, instead of parsing the header file.
func (p *FileParser) SetFieldNames(fieldNames []string) *FileParser {
	p.FileHeader.FieldNames = fieldNames
	return p
}

// Next returns the next parsed log line.
func (p *FileParser) Next() (*Line, error) {
	var name string
	var i int
	if p.scanner.Scan() {
		l := newLine()
		fields := p.scanner.Strings()
		if len(fields) != len(p.FieldNames) {
			return nil, fmt.Errorf("Wrong number of fields: expected = %d, actual = %d", len(p.FieldNames), len(fields))
		}
		for i, name = range p.FieldNames {
			l.add(name, fields[i])
		}
		return &l, nil
	}
	return nil, p.scanner.Err()
}

// Scanner is a stream oriented parser for W3C Extended Log Format lines.
type Scanner struct {
	reader  io.Reader
	strings []string
	done    bool
	buf     []byte
	origbuf []byte
	err     error
}

// NewScanner constructs a Scanner.
func NewScanner(reader io.Reader) *Scanner {
	s := Scanner{
		reader:  reader,
		origbuf: make([]byte, 0, 4096),
	}
	s.buf = s.origbuf
	return &s
}

// Scan advances the Scanner to the next log line, which will then be available
// through the Strings method. It returns false when the scan stops, either by
// reaching the end of the input or an error. After Scan returns false, the Err
// method will return any error that occurred during scanning, except that if
// it was io.EOF, Err will return nil.
func (s *Scanner) Scan() bool {
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
			rest, strings, err = ExtractStrings(s.buf)
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
			} else if err != ErrNoEndline && err != ErrQuoteLeftOpen {
				// parsing error
				s.err = err
				return false
			} else if s.err == io.EOF && err == ErrNoEndline {
				// there is no more available data to read
				// just output the last content
				if len(strings) > 0 {
					s.strings = strings
					s.buf = rest
					return true
				}
				return false
			} else if s.err == io.EOF && err == ErrQuoteLeftOpen {
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
		if err == io.EOF {
			if s.err == nil {
				s.err = io.EOF
			}
		} else if err != nil {
			s.err = err
			return false
		} else if n == 0 {
			// err == nil but n == 0
			s.err = io.ErrNoProgress
			return false
		}
		s.buf = s.buf[:len(s.buf)+n]
	}
}

// Strings returns the most recent fields generated by a call to Scan as a newly allocated string slice.
func (s *Scanner) Strings() []string {
	return s.strings
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
	if s.err != nil && s.err != io.EOF {
		return s.err
	}
	return nil
}

// ExtractStrings scans the input for the next available log line.
// It returns the unparsed part of input in rest.
//
// fields will contain the parsed fields of the log line.
//
// err will be nil, ErrEndlineInsideQuotes, ErrNoEndline or ErrQuoteLeftOpen.
func ExtractStrings(input []byte) (rest []byte, fields []string, err error) {
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
	c := bytes.Count(m[:linelen], sp)
	// we assume that we have c+1 fields to extract
	buffers := make([]*buff, 0, c+1)
	var curbuf *buff
	var haveString bool
	var haveFirstChar bool
	var curchar byte
	var icur int

	w := func(buffers *[]*buff, curbuf **buff, b byte) {
		if (*curbuf) == nil {
			*curbuf = pool.Get()
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
				return input, nil, ErrEndlineInsideQuotes
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
					pool.Put(buf)
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
		return input, nil, ErrQuoteLeftOpen
	}

	if len(buffers) == 0 {
		// no content
		return nil, nil, nil
	}

	fields = make([]string, 0, len(buffers))
	for _, buf := range buffers {
		fields = append(fields, replace20(buf.String()))
		pool.Put(buf)
	}
	// we have reached the end of input, but no endline char was present
	// that may or may not be normal, so let's report it
	return nil, fields, ErrNoEndline
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
