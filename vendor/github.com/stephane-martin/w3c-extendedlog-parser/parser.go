package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

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
