package utmpx

// #include <utmpx.h>
// #include <string.h>
import "C"

func closeutmpx() {
	C.endutxent()
}

func resetutmpx() {
	C.setutxent()
}

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

func toStr(b *C.char, max C.size_t) string {
	if b == nil || max == 0 {
		return ""
	}
	l := C.strnlen(b, max)
	if l == 0 {
		return ""
	}
	return C.GoStringN(b, C.int(l))
}
