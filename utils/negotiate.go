package utils

import (
	"net/http"
	"strings"
)

type octetType byte

const (
	isToken octetType = 1 << iota
	isSpace
)

var octetTypes [256]octetType

func init() {
	// OCTET      = <any 8-bit sequence of data>
	// CHAR       = <any US-ASCII character (octets 0 - 127)>
	// CTL        = <any US-ASCII control character (octets 0 - 31) and DEL (127)>
	// CR         = <US-ASCII CR, carriage return (13)>
	// LF         = <US-ASCII LF, linefeed (10)>
	// SP         = <US-ASCII SP, space (32)>
	// HT         = <US-ASCII HT, horizontal-tab (9)>
	// <">        = <US-ASCII double-quote mark (34)>
	// CRLF       = CR LF
	// LWS        = [CRLF] 1*( SP | HT )
	// TEXT       = <any OCTET except CTLs, but including LWS>
	// separators = "(" | ")" | "<" | ">" | "@" | "," | ";" | ":" | "\" | <">
	//              | "/" | "[" | "]" | "?" | "=" | "{" | "}" | SP | HT
	// token      = 1*<any CHAR except CTLs or separators>
	// qdtext     = <any TEXT except <">>

	for c := 0; c < 256; c++ {
		var t octetType
		isCtl := c <= 31 || c == 127
		isChar := 0 <= c && c <= 127
		isSeparator := strings.ContainsRune(" \t\"(),/:;<>?@[]\\{}", rune(c))
		if strings.ContainsRune(" \t\r\n", rune(c)) {
			t |= isSpace
		}
		if isChar && !isCtl && !isSeparator {
			t |= isToken
		}
		octetTypes[c] = t
	}
}

func skipSpace(s string) (rest string) {
	i := 0
	for ; i < len(s); i++ {
		if octetTypes[s[i]]&isSpace == 0 {
			break
		}
	}
	return s[i:]
}

func expectTokenSlash(s string) (token, rest string) {
	i := 0
	for ; i < len(s); i++ {
		b := s[i]
		if (octetTypes[b]&isToken == 0) && b != '/' {
			break
		}
	}
	return s[:i], s[i:]
}

func expectQuality(s string) (q float64, rest string) {
	switch {
	case len(s) == 0:
		return -1, ""
	case s[0] == '0':
		q = 0
	case s[0] == '1':
		q = 1
	default:
		return -1, ""
	}
	s = s[1:]
	if !strings.HasPrefix(s, ".") {
		return q, s
	}
	s = s[1:]
	i := 0
	n := 0
	d := 1
	for ; i < len(s); i++ {
		b := s[i]
		if b < '0' || b > '9' {
			break
		}
		n = n*10 + int(b) - '0'
		d *= 10
	}
	return q + float64(n)/float64(d), s[i:]
}

type AcceptSpec struct {
	Range   string
	Quality float64
	Params  []string
}

func (spec *AcceptSpec) Type() string {
	i := strings.Index(spec.Range, "/")
	if i < 0 {
		return spec.Range
	}
	return spec.Range[:i]
}

func (spec *AcceptSpec) SubType() string {
	i := strings.Index(spec.Range, "/")
	if i < 0 {
		return ""
	}
	if i == (len(spec.Range) - 1) {
		return ""
	}
	return spec.Range[i+1:]
}

func (self *AcceptSpec) Inferior(other AcceptSpec) bool {
	if self.Quality < other.Quality {
		return true
	}

	selfIsWild := self.IsWild()
	selfIsHalfWild := self.IsHalfWild()
	otherIsWild := other.IsWild()
	otherIsHalfWild := other.IsHalfWild()
	selfIsPlain := !selfIsWild && !selfIsHalfWild
	otherIsPlain := !otherIsWild && !otherIsHalfWild

	if selfIsWild {
		if !otherIsWild {
			return true
		}
	}

	if otherIsWild {
		if !selfIsWild {
			return false
		}
	}

	if selfIsHalfWild {
		if otherIsWild {
			return false
		}
		if otherIsPlain {
			return true
		}
	}

	if otherIsHalfWild {
		if selfIsWild {
			return true
		}
		if selfIsPlain {
			return false
		}
	}

	return len(self.Params) < len(other.Params)
}

func (spec *AcceptSpec) IsWild() bool {
	return spec.Type() == "*"
}

func (spec *AcceptSpec) IsHalfWild() bool {
	return spec.Type() != "*" && spec.SubType() == "*"
}

func (spec *AcceptSpec) Match(offer string) bool {
	if spec.IsWild() {
		return true
	}
	if spec.IsHalfWild() && strings.HasPrefix(offer, spec.Type()+"/") {
		return true
	}
	return spec.Range == offer
}

func parseOneAccept(header string, specs []AcceptSpec) []AcceptSpec {
	var q float64
	for {
		spec := AcceptSpec{
			Quality: 1,
			Params:  make([]string, 0),
		}
		spec.Range, header = expectTokenSlash(header)
		if spec.Range == "" {
			return specs
		}
		for {
			header = skipSpace(header)
			if !strings.HasPrefix(header, ";") {
				break
			}
			header = skipSpace(header[1:])
			if !strings.HasPrefix(header, "q=") {
				var v string
				v, header = expectTokenSlash(header)
				spec.Params = append(spec.Params, v)
				continue
			}
			q, header = expectQuality(header[2:])
			if q != -1 {
				spec.Quality = q
			}
			break
		}
		specs = append(specs, spec)
		header = skipSpace(header)
		if !strings.HasPrefix(header, ",") {
			return specs
		}
		header = skipSpace(header[1:])
	}
}

func ParseAccept(headers []string) (specs []AcceptSpec) {
	specs = make([]AcceptSpec, 0)
	for _, header := range headers {
		specs = parseOneAccept(header, specs)
	}
	return specs
}

func NegotiateContentType(r *http.Request, serverOffers []string, defaultOffer string) string {
	return negotiateContentType(ParseAccept(r.Header["Accept"]), serverOffers, defaultOffer)
}

func negotiateContentType(clientSpecs []AcceptSpec, serverOffers []string, defaultOffer string) string {
	bestClientSpec := AcceptSpec{
		Range:   defaultOffer,
		Quality: -1,
		Params:  []string{},
	}
	bestOffer := defaultOffer
	for _, clientSpec := range clientSpecs {
		for _, serverOffer := range serverOffers {
			if !clientSpec.Match(serverOffer) {
				continue
			}
			if bestClientSpec.Inferior(clientSpec) {
				bestClientSpec = clientSpec
				bestOffer = serverOffer
			}
		}
	}
	return bestOffer
}
