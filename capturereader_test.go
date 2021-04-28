//package httpscapture
package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestCaptureReader(t *testing.T) {
	var err error

	const testdata = "abcdefg"
	inbuf := bytes.NewBufferString(testdata)

	callback_ok := false
	callback_err := fmt.Errorf("dummy error")
	cr := NewCaptureReadCloserCallback(io.NopCloser(inbuf), func(err error) {
		callback_ok = true
		callback_err = err
	})

	outbuf := make([]byte, 32)

	sz, err := cr.Read(outbuf[:3])
	if err != nil {
		t.Error(err)
	}

	s2, err := cr.Read(outbuf[3:])
	if err != nil {
		t.Error(err)
	}

	sz += s2

	if sz != len(testdata) {
		t.Errorf("read length not match")
	}

	//fmt.Printf("%d:[%s]\n", cr.Size, string(cr.Buf[:sz]))

	if testdata != string(outbuf[:sz]) {
		t.Errorf("read data not match")
	}
	if testdata != string(cr.Buffer[:cr.Size]) {
		t.Errorf("captured data not match")
	}

	err = cr.Close()
	if err != nil {
		t.Error(err)
	}
	if !callback_ok {
		t.Errorf("callback not notified")
	}
	if callback_err != err {
		t.Errorf("callback error not registered")
	}

}
