//package httpscapture
package main

import (
	"io"
)

// CaptureReader captures a io.Reader that captures all input data to internal Buffer
type CaptureReader struct {
	R      io.Reader
	Buffer []byte
	Size   int64
}

const BLOCKSIZE = 1024

func NewCaptureReader(r io.Reader) *CaptureReader {
	return &CaptureReader{R: r, Buffer: make([]byte, 0), Size: 0}
}

func (b *CaptureReader) Read(p []byte) (n int, err error) {
	n, err = b.R.Read(p)
	if n > 0 {
		b.Buffer = append(b.Buffer, p...)
		b.Size += int64(n)
	}
	return n, err
}

type CaptureReadCloser struct {
	CaptureReader
	C       io.Closer
	Closed  bool
	onClose func(error)
}

// CaptureReadCloser is a io.ReaderCloser for CaptureReader
func NewCaptureReadCloserCallback(r io.ReadCloser, onClose func(error)) *CaptureReadCloser {
	return &CaptureReadCloser{CaptureReader: CaptureReader{R: r, Buffer: make([]byte, 0), Size: 0}, C: r, Closed: false, onClose: onClose}
}

func NewCaptureReadCloser(r io.ReadCloser) *CaptureReadCloser {
	return &CaptureReadCloser{CaptureReader: CaptureReader{R: r, Buffer: make([]byte, 0), Size: 0}, C: r, Closed: false}
}

func (b *CaptureReadCloser) Close() error {
	err := b.C.Close()

	if b.onClose != nil {
		b.onClose(err)
	}
	if err == nil {
		b.Closed = true
	}
	return err
}
