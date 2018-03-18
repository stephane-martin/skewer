package parser

import (
	"bytes"
	"strings"
)

var sp = []byte(" ")

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
