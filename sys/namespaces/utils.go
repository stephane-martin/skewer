package namespaces

// deriveKeys returns the keys of the input map as a slice.
func deriveKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// deriveAnyHidden reports whether the predicate returns true for any of the elements in the given slice.
func deriveAnyHidden(pred func(string) bool, list []string) bool {
	for _, elem := range list {
		if pred(elem) {
			return true
		}
	}
	return false
}
