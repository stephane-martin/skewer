// +build !linux nonsystemd

package journald

func Dummy() bool {
	return true
}

type JournaldReader interface {
	Start()
	Stop()
	Close() error
	Entries() chan map[string]string
}

type reader struct {
	entries chan map[string]string
}

func NewReader() (JournaldReader, error) {
	r := &reader{}
	r.entries = make(chan map[string]string)
	return r, nil
}

func (r *reader) Start() {}
func (r *reader) Stop()  {}

func (r *reader) Close() error {
	close(r.entries)
	return nil
}

func (r *reader) Entries() chan map[string]string {
	return r.entries
}
