package sys

// deriveIntersectNames returns the intersection of the two lists' values
// It assumes that the first list only contains unique items.
func deriveIntersectNames(this, that []string) []string {
	intersect := make([]string, 0, deriveMin(len(this), len(that)))
	for i, v := range this {
		if deriveContains(that, v) {
			intersect = append(intersect, this[i])
		}
	}
	return intersect
}

// deriveContains returns whether the item is contained in the list.
func deriveContains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// deriveUniqueNames returns a list containing only the unique items from the input list.
// It does this by reusing the input list.
func deriveUniqueNames(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	u := 1
	for i := 1; i < len(list); i++ {
		if !deriveContains(list[:u], list[i]) {
			if i != u {
				list[u] = list[i]
			}
			u++
		}
	}
	return list[:u]
}

// deriveMin returns the mimimum of the two input values.
func deriveMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
