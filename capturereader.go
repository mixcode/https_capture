package main

import (
	"bytes"
	"io"
)

/*
// FileCaptureReadCloser is a file capturer to a temp file
type FileCaptureReadCloser struct {
	R           io.ReadCloser
	TmpFileName string
	Size        int64
	TmpFile     *os.File

	Closed  bool
	onClose func(error)
}

func NewFileCaptureReaderCloser(r io.ReadCloser, tmpDir string) (fr *FileCaptureReadCloser, err error) {
	tmpfile, err := os.CreateTemp(tmpDir, "_tmp_")
	if err != nil {
		return
	}
	return &FileCaptureReadCloser{R: r, TmpFile: tmpfile, TmpFileName: tmpfile.Name(), Size: 0}, nil
}

func (b *FileCaptureReadCloser) Read(p []byte) (n int, err error) {
	n, err = b.R.Read(p)
	if n > 0 {
		_, err = b.TmpFile.Write(p[:n])
		if err != nil {
			return
		}
		b.Size += int64(n)
	}
	return n, err
}

func (b *FileCaptureReadCloser) Close() error {
	err := b.R.Close()
	if b.onClose != nil {
		b.onClose(err)
	}
	b.Closed = true

	b.TmpFile.Close()

	return err
}

func (b *FileCaptureReadCloser) RemoveFile() error {
	return os.Remove(b.TmpFileName)
}
*/

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

/*
// MemoryCaptureReader is a io.Reader that captures all input data to a memory buffer
type CaptureReadCloser struct {
	R    io.ReadCloser
	Size int64

	Threshold int64 // if data is larger than this size, use a temporary file

	// use in-memory buffer if data is smaller then Threshold
	Buffer *bytes.Buffer

	// use a temporary file if data size is equal or largher than Threshold
	TmpDir      string
	TmpFile     *os.File
	TmpFileName string

	Closed  bool
	onClose func(error)
}

func (b *CaptureReadCloser) Read(p []byte) (n int, err error) {
	n, err = b.R.Read(p)
	if n > 0 {
		b.Buffer.Write(p[:n])
		b.Size += int64(n)
	}
	return n, err
}

func NewCaptureReadCloserCallback(r io.ReadCloser, tmpDir string, onClose func(error)) *CaptureReadCloser {
	return &CaptureReadCloser{R: r, Buffer: new(bytes.Buffer), Size: 0, Closed: false, onClose: onClose, TmpDir: tmpDir}
}

func NewCaptureReadCloser(r io.ReadCloser, tmpDir string) *CaptureReadCloser {
	return &CaptureReadCloser{R: r, Buffer: new(bytes.Buffer), Size: 0, Closed: false, TmpDir: tmpDir}
}

func (b *CaptureReadCloser) Close() error {
	err := b.R.Close()

	if b.onClose != nil {
		b.onClose(err)
	}
	if err == nil {
		b.Closed = true
	}

	if b.TmpFile != nil {
		b.TmpFile.Close()
	}

	return err
}

func (b *CaptureReadCloser) SaveToFile(filename string) (err error) {
	if b.TmpFile == nil {
		// write byte buffer to a file
		return os.WriteFile(filename, b.Buffer.Bytes(), 644)
	}
	fo, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 644)
	if err != nil {
		return
	}
	defer fo.Close()
	fi, err := os.Open(b.TmpFileName)
	if err != nil {
		return
	}
	defer func() {
		fi.Close()
		os.Remove(b.TmpFileName)
	}()
	_, err = io.Copy(fo, fi)
	return
}
*/
