package utils

// PrintableUsASCII returns true if the provided string is only composed by ASCII characters.
func PrintableUsASCII(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
	}
	return true
}
