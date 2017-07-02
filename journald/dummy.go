// +build !linux

package journald

func Dummy() bool {
	return true
}

type Reader struct {
	Entries chan map[string]string
}

func NewReader() (r *Reader, err error) {
	r = &Reader{}
	r.Entries = make(chan map[string]string)
	return r, nil
}

func (r *Reader) Start()       {}
func (r *Reader) Stop()        {}
func (r *Reader) Close() error { return nil }
