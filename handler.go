package main

//
// HTTP request, response, and close handlers
//
// If you want to customize the capturing data, you may modify httpCloseCallback() in this file.
//
// github.com/mixcode, 2021-04
//

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	//"github.com/elazarl/goproxy"
	"github.com/mixcode/goproxy" // a clone of elazarl/goproxy with fixes for TLS SNI
)

var (
	// session
	mutex   sync.Mutex
	session = make(map[int64]*Connection)
)

// A captured HTTP connection
type Connection struct {
	Req     *http.Request      // HTTP request
	ReqBody *CaptureReadCloser // HTTP request body stream

	Resp     *http.Response     // HTTP response
	RespBody *CaptureReadCloser // HTTP response body stream
}

// HTTP connection closed; write the result to file
func httpCloseCallback(sessionId int64, conn *Connection) func(error) {

	//
	// This sub-function is called when a HTTP(s) connection has closed.
	// session[sessionId] contains complete history of a connection
	//

	logFunc := func(l *log, isText bool, contentType, filename string, body []byte, indent string) (savedToFile bool, err error) {
		if logPostInlineAll || (logPostInline && isText) {
			savedToFile = false
			s := string(body)
			done := false
			if contentType == "application/x-www-form-urlencoded" && rawPostForm == false {
				// form-urlencoded
				values, e := url.ParseQuery(s)
				if e == nil {
					for k, v := range values {
						l.writef("%s%s=%s\n", indent, k, v)
					}
					done = true
				} else {
					if verbose {
						fmt.Printf("form POST data is not a http query string")
					}
				}
			}
			if !done {
				l.writef("%s%s\n", indent, s)
				done = true
			}
		} else {
			savedToFile = true
			if contentType == "application/x-www-form-urlencoded" && rawPostForm == false {
				// form-urlencoded
				values, e := url.ParseQuery(string(body))
				if e == nil {
					var buf bytes.Buffer
					for k, v := range values {
						_, err = fmt.Fprintf(&buf, "%s=%s\n", k, v)
						if err != nil {
							return
						}
					}
					body = buf.Bytes()
				}
			}
			err = os.WriteFile(filename, body, 0644)
		}
		return
	}

	return func(err error) {
		mutex.Lock()
		s, ok := session[sessionId]
		if !ok || s != conn {
			mutex.Unlock()
			return
		}
		delete(session, sessionId)
		mutex.Unlock()

		l := newLog()
		defer l.flush()

		// Print the connection
		l.writef("%s [%d] end %s (%s) %s\n", timestamp(), sessionId, conn.Req.Method, conn.Resp.Status, conn.Req.URL.String())

		l.writef("\t==== Req: headers ====\n")
		for k, v := range conn.Req.Header {
			l.writef("\t\t%s: %v\n", k, v)
		}
		if conn.ReqBody.Size > 0 {
			l.writef("\t---- Req: body ----\n")
			contentType := ""
			ct := conn.Req.Header["Content-Type"]
			if len(ct) > 0 {
				contentType = ct[0]
			}
			isText := true
			ext := ""
			if contentType != "" {
				_, _, ext, isText, _ = mediaType(contentType)
			}
			if ext == "" {
				ext = ".bin"
			}
			fname := fmt.Sprintf("%06d_a_%s%s", sessionId, conn.Req.Method, ext)
			fpath := filepath.Join(captureDir, fname)

			body := conn.ReqBody.Buffer[:conn.ReqBody.Size]

			ce := conn.Req.Header["Content-Encoding"]
			if len(ce) > 0 && ce[0] == "gzip" {
				b := bytes.NewBuffer(body)
				gz, e := gzip.NewReader(b)
				if e != nil {
					err = e
					return
				}
				o := new(bytes.Buffer)
				_, err = o.ReadFrom(gz)
				gz.Close()
				if err != nil {
					return
				}
				body = o.Bytes()
			}

			saved := false
			saved, err = logFunc(l, isText, contentType, fpath, body, "\t\t")
			if err != nil {
				return
			}
			if saved {
				l.writef("\t\t(saved to %s)\n", fname)
			}
		}

		l.writef("\t==== Resp (%s): headers ====\n", conn.Resp.Status)
		for k, v := range conn.Resp.Header {
			l.writef("\t\t%s: %v\n", k, v)
		}

		// Write the result body to a file
		if conn.RespBody.Size > 0 {

			l.writef("\t---- Resp: body ----\n")

			// determine file name and type
			contentType := ""
			ct := conn.Resp.Header["Content-Type"]
			if len(ct) > 0 {
				contentType = ct[0]
			}

			// detect filename by Content-Disposition
			outfilename, filename_unknown := "", false
			if disp, ok := conn.Resp.Header["Content-Disposition"]; ok {
				_, param, _ := mime.ParseMediaType(disp[0])
				outfilename = param["filename"]
			}
			// detect filename by URL path
			if outfilename == "" {
				_, outfilename = path.Split(conn.Req.URL.EscapedPath())
			}
			if outfilename == "" {
				// cannot determine filename
				filename_unknown = true
				outfilename = "unknown"
			}

			// determine file extension
			isText := true
			ext := path.Ext(outfilename)
			filebody := outfilename[:len(outfilename)-len(ext)]
			if ext == "" && contentType != "" {
				_, _, ext, isText, _ = mediaType(contentType)
			}
			if ext == "" {
				// unknown file type
				ext = ".bin"
			}

			if filename_unknown && ext == ".html" {
				outfilename = "index.html"
			} else {
				outfilename = filebody + ext
			}
			outfilename = fmt.Sprintf("%06d_b_%s", sessionId, outfilename)

			// trim if the filename is too long
			shortname := outfilename
			if len(shortname) > filenameMaxLen {
				l := len(ext)
				if l > filenameMaxLen {
					// a long filename starts with a dot
					shortname = shortname[:filenameMaxLen]
				} else {
					shortname = shortname[:filenameMaxLen-l] + ext
				}
			}
			outpath := filepath.Join(captureDir, shortname)

			body := conn.RespBody.Buffer[:conn.RespBody.Size]
			// TODO: log uncompressed response?

			saved := false
			saved, err = logFunc(l, isText, contentType, outpath, body, "\t\t")
			if err != nil {
				return
			}
			if saved {
				l.writef("\t\t(saved to %s)\n", shortname)
			}
		}
		l.writef("\n")
	}
}

// record the start of a HTTP request
func reqHandler(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	sessionId := ctx.Session
	conn := Connection{Req: req}
	newReq := req.Clone(context.Background())

	if req.Body != nil {
		conn.ReqBody = NewCaptureReadCloser(req.Body)
		newReq.Body = conn.ReqBody
	}

	mutex.Lock()
	session[sessionId] = &conn
	mutex.Unlock()

	log := newLog()
	defer log.flush()
	log.writef("%s [%d] start %s %s\n", timestamp(), sessionId, conn.Req.Method, conn.Req.URL.String())
	return newReq, nil
}

// record a HTTP response
func respHandler(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	sessionId := ctx.Session
	mutex.Lock()
	conn := session[sessionId]
	mutex.Unlock()
	if conn == nil {
		return resp
	}

	conn.Resp = resp
	if resp.Body != nil {
		conn.RespBody = NewCaptureReadCloserCallback(resp.Body, httpCloseCallback(sessionId, conn))
		resp.Body = conn.RespBody
	}
	return resp
}

func timestamp() string {
	return time.Now().Format(time.RFC3339)
}

/*
// write a message to the logfile
func writef(format string, arg ...interface{}) {
	fmt.Fprintf(listWriter, format, arg...)
	if tee {
		fmt.Printf(format, arg...)
	}
}
*/
