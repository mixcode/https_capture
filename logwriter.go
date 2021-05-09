package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

var (
	logWriter   io.Writer
	chLogBuffer = make(chan []byte, 64)
)

type log struct {
	b *bytes.Buffer
}

func newLog() *log {
	return &log{b: &bytes.Buffer{}}
}

func (l *log) Write(p []byte) (n int, err error) {
	return l.b.Write(p)
}

func (l *log) writef(format string, arg ...interface{}) {
	fmt.Fprintf(l.b, format, arg...)
}

func (l *log) flush() {
	buf := l.b.Bytes()
	if len(buf) > 0 {
		go func() {
			chLogBuffer <- buf
		}()
	}
}

func startLog() {
	go func() {
		ok := true
		for buf := range chLogBuffer {
			if !ok {
				continue
			}
			_, err := logWriter.Write(buf)
			if err != nil {
				ok = false
				chError <- err
			}
			if tee {
				os.Stdout.Write(buf)
			}
		}
	}()
}

func stopLog() {
	close(chLogBuffer)
}
