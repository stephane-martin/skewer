package parser

import "sync"

type buff struct {
	B []byte
}

func (b *buff) WriteByte(c byte) {
	b.B = append(b.B, c)
}

func (b *buff) String() string {
	return string(b.B)
}

type bPool struct {
	pool *sync.Pool
}

func (p *bPool) Get() *buff {
	b := p.pool.Get().(*buff)
	b.B = b.B[:0]
	return b
}

func (p *bPool) Put(b *buff) {
	if b != nil {
		p.pool.Put(b)
	}
}

var pool *bPool

func init() {
	pool = &bPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return &buff{
					B: make([]byte, 0, 256),
				}
			},
		},
	}
}
