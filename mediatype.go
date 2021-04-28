package main

import (
	"mime"
	"strings"
)

var (
	textHeader = make(map[string]bool)
	textType   = make(map[string]bool)

	extensions = map[string]string{
		"text/plain": ".txt",
		"text/html":  ".html",
		"image/jpeg": ".jpg",
	}

	textMimeHeader = []string{
		"text", // "text/html"
		"xml",  // "xml/svg"
	}

	textMimeType = []string{
		"application/json",
		"application/javascript",
	}
)

func mediaType(mtype string) (mediaType string, params map[string]string, fileExt string, isText bool, err error) {
	mediaType, params, err = mime.ParseMediaType(mtype)
	if err != nil {
		return
	}

	a := strings.Split(mediaType, "/")
	isText = textHeader[a[0]] || textType[mediaType]

	ok := false
	if fileExt, ok = extensions[mediaType]; !ok {
		exts, e := mime.ExtensionsByType(mediaType)
		if e != nil {
			err = e
			return
		}
		if len(exts) > 0 {
			fileExt = exts[0]
		}
	}

	return
}

func init() {
	for _, s := range textMimeHeader {
		textHeader[s] = true
	}
	for _, s := range textMimeType {
		textType[s] = true
	}
}
