package main

import (
	"bytes"
	"fmt"
	"io"
)

var (
	logWriter io.Writer

	logChannel = make(chan []byte, 64)
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
			logChannel <- buf
		}()
	}
}

func startLog() {
	go func() {
		for buf := range logChannel {
			_, err := logWriter.Write(buf)
			if err != nil {
				// todo: kill program
			}
		}
	}()
}

func stopLog() {
	close(logChannel)
}
