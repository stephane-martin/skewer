package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// FileHeader represents the header of a W3C Extended Log Format file.
type FileHeader struct {
	fieldNames []string
	Software   string
	Remark     string
	Meta       map[string]string
}

func (h *FileHeader) HasField(name string) bool {
	for _, fname := range h.fieldNames {
		if fname == name {
			return true
		}
	}
	return false
}

func (h *FileHeader) HasGmtTime() bool {
	return h.HasField("gmttime")
}

// FieldNames returns a copy of the field names
func (h *FileHeader) FieldNames() (ret []string) {
	if len(h.fieldNames) == 0 {
		return nil
	}
	ret = make([]string, 0, len(h.fieldNames))
	for _, name := range h.fieldNames {
		ret = append(ret, name)
	}
	return ret
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
					h.fieldNames = make([]string, 0)
					for _, f := range strings.Split(value, " ") {
						f = strings.ToLower(strings.TrimSpace(f))
						if len(f) > 0 {
							h.fieldNames = append(h.fieldNames, f)
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
		// use a big buffer to minimize disk reads
		bufreader = bufio.NewReaderSize(reader, 16*1024*1024)
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
	p.FileHeader.fieldNames = fieldNames
	return p
}

// Next returns the next parsed log line.
func (p *FileParser) Next() (*Line, error) {
	return p.NextTo(nil)
}

// NextTo returns the next parsed log line, reusing the given line.
func (p *FileParser) NextTo(l *Line) (*Line, error) {
	if len(p.FileHeader.fieldNames) == 0 {
		return nil, errors.New("No field names")
	}
	var name string
	var i int
	if p.scanner.Scan() {
		if l == nil {
			// allocate a new line
			l = NewLine(p.FileHeader.fieldNames)
		} else {
			// reuse the given line, but make sure to clean it before usage
			l.Reset(p.FileHeader.fieldNames)
		}
		fields := p.scanner.Strings()
		if len(fields) != len(p.FileHeader.fieldNames) {
			return nil, fmt.Errorf("Wrong number of fields: expected = %d, actual = %d", len(p.FileHeader.fieldNames), len(fields))
		}
		for i, name = range p.FileHeader.fieldNames {
			l.add(name, fields[i])
		}
		return l, nil
	}
	return nil, p.scanner.Err()
}
