package logging

import (
	"sort"
)

// deriveSortRecord sorts the slice inplace and also returns it.
func deriveSortRecord(list []string) []string {
	sort.Strings(list)
	return list
}

// deriveKeysRecord returns the keys of the input map as a slice.
func deriveKeysRecord(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
