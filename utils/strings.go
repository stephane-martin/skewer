package utils

func PrintableUsASCII(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
	}
	return true
}
