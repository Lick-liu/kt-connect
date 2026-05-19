package util

import (
	"io"
	"sync"
)

type InterpretableReader struct {
	r         io.Reader
	interrupt chan struct{}
	cancel    *sync.Once
}

func NewInterpretableReader(r io.Reader) InterpretableReader {
	return InterpretableReader{
		r,
		make(chan struct{}),
		&sync.Once{},
	}
}

func (r InterpretableReader) Read(p []byte) (n int, err error) {
	if r.r == nil {
		return 0, io.EOF
	}
	select {
	case <-r.interrupt:
		return 0, io.EOF
	default:
		return r.r.Read(p)
	}
}

func (r InterpretableReader) Cancel() {
	r.cancel.Do(func() {
		close(r.interrupt)
	})
}
