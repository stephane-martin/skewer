package utils

import "strings"
import "golang.org/x/text/encoding/unicode"
import "golang.org/x/text/encoding/charmap"
import "golang.org/x/text/encoding"

func SelectDecoder(coding string) *encoding.Decoder {
	coding = strings.TrimSpace(strings.ToLower(strings.Replace(coding, "-", "", -1)))
	var enc encoding.Encoding
	switch coding {
	case "utf8":
		enc = unicode.UTF8
	case "iso88591", "latin1":
		enc = charmap.ISO8859_1
	case "windows1252":
		enc = charmap.Windows1252
	case "iso885915", "latin15":
		enc = charmap.ISO8859_15
	default:
		enc = unicode.UTF8
	}
	return enc.NewDecoder()
}
