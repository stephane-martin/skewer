package utmpx

func All() (entries []Entry) {
	entries = make([]Entry, 0)
	var entry *Entry
	resetutmpx()
	for {
		entry = next()
		if entry == nil {
			closeutmpx()
			break
		}
		entries = append(entries, *entry)
	}
	return entries
}
