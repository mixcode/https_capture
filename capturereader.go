package main

import (
	"bytes"
	"io"
)

// CaptureReader is a io.Reader that captures all input data to a memory buffer
type CaptureReader struct {
	R      io.Reader
	Buffer *bytes.Buffer
	Size   int64
}

func NewCaptureReader(r io.Reader) *CaptureReader {
	return &CaptureReader{R: r, Buffer: new(bytes.Buffer), Size: 0}
}

func (b *CaptureReader) Read(p []byte) (n int, err error) {
	n, err = b.R.Read(p)
	if n > 0 {
		b.Buffer.Write(p[:n])
		b.Size += int64(n)
	}
	return n, err
}

// CaptureReadCloser is a io.ReaderCloser for CaptureReader
type CaptureReadCloser struct {
	CaptureReader
	C       io.Closer
	Closed  bool
	onClose func(error)
}

func NewCaptureReadCloserCallback(r io.ReadCloser, onClose func(error)) *CaptureReadCloser {
	return &CaptureReadCloser{CaptureReader: CaptureReader{R: r, Buffer: new(bytes.Buffer), Size: 0}, C: r, Closed: false, onClose: onClose}
}

func NewCaptureReadCloser(r io.ReadCloser) *CaptureReadCloser {
	return &CaptureReadCloser{CaptureReader: CaptureReader{R: r, Buffer: new(bytes.Buffer), Size: 0}, C: r, Closed: false}
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
