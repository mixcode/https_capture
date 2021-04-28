package main

import (
	//"fmt"
	// "os"
	"testing"
)

func TestMediaType(t *testing.T) {
	var err error

	mt, params, fileExt, isText, err := mediaType("text/plain;charset=UTF-8")
	//fmt.Printf("%s|%s|%s|%v\n", mt, params, fileExt, isText)

	if mt != "text/plain" || params["charset"] != "UTF-8" || fileExt != ".txt" || !isText {
		t.Errorf("type decode failed")
	}

	/*
		types := []string {
			 "application/javascript",
			 "application/javascript; charset=utf-8",
			 "application/json",
			 "application/x-www-form-urlencoded; charset=UTF-8",
			 "image/gif",
			 "image/jpeg",
			 "image/png",
			 "image/webp",
			 "text/css",
			 "text/html; charset=UTF-8",
			 "text/javascript",
			 "text/javascript; charset=UTF-8",
			 "text/plain; charset=UTF-8",
			 "text/plain;charset=UTF-8",
		}
		for _, s := range types {
			mt, params, fileExt, isText, err = mediaType(s)
			fmt.Printf("%s : %s|%v|%s|%v\n", s, mt, params, fileExt, isText)
		}
	*/

	if err != nil {
		t.Error(err)
		// t.Errorf("%s", err)
		// t.Fatal(err)
		// t.Fatalf("%s", err)
	}
}
