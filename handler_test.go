package main

import (
	"testing"
)

func TestContentRange(t *testing.T) {
	var err error

	start, end, total, err := contentRange("bytes 1-100/999")
	if err != nil {
		t.Error(err)
	}
	if start != 1 || end != 100 || total != 999 {
		t.Errorf("invalid parsed values")
	}
	//log.Printf("%d-%d/%d", start, end, total)

	start, end, total, err = contentRange("bytes 1-100/*")
	if err != nil {
		t.Error(err)
	}
	if start != 1 || end != 100 || total != 0 {
		t.Errorf("invalid parsed values")
	}
	//log.Printf("%d-%d/%d", start, end, total)

	start, end, total, err = contentRange("bytes */999")
	if err != nil {
		t.Error(err)
	}
	if start != 0 || end != 0 || total != 999 {
		t.Errorf("invalid parsed values")
	}
	//log.Printf("%d-%d/%d", start, end, total)

}
