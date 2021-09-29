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
	sessionMutex sync.Mutex
	session      = make(map[int64]*Connection)
)

// A captured HTTP connection
type Connection struct {
	Host string // host name for HTTP request. maybe empty.

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
			if contentType == "application/x-www-form-urlencoded" && !rawPostForm {
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
			if contentType == "application/x-www-form-urlencoded" && !rawPostForm {
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

	//
	// the handler main function
	//
	mainFunc := func(inErr error) (err error) {
		l := newLog()
		defer l.flush()

		if inErr != nil {
			// HTTP error happened
			l.writef("%s [%d] failed (%v) %s %s\n", timestamp(), sessionId, inErr.Error(), conn.Req.Method, conn.Req.URL.String())
			return nil // ignore and continue
		}

		// print the connection info
		l.writef("%s [%d] end (%s) %s %s\n", timestamp(), sessionId, conn.Resp.Status, conn.Req.Method, conn.Req.URL.String())

		// write request headers
		l.writef("\t==== Req: headers ====\n")
		for k, v := range conn.Req.Header {
			l.writef("\t\t%s: %v\n", k, v)
		}

		// write the request body
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

			body := conn.ReqBody.Buffer.Bytes()

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

		// write response headers
		l.writef("\t==== Resp (%s): headers ====\n", conn.Resp.Status)
		for k, v := range conn.Resp.Header {
			l.writef("\t\t%s: %v\n", k, v)
		}

		// Write the response body
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

			body := conn.RespBody.Buffer.Bytes()
			// TODO: log raw compressed body?

			saved := false
			saved, err = logFunc(l, isText, contentType, outpath, body, "\t\t")
			if err != nil {
				return
			}
			if saved {
				l.writef("\t\t(saved to %s)\n", shortname)
			}
		}
		l.writef("\n") // a blank line to improve readability
		return
	}

	// actual callback function for proxy engine
	return func(inErr error) {
		// remove the current session from the buffer
		sessionMutex.Lock()
		s, ok := session[sessionId]
		if !ok || s != conn {
			sessionMutex.Unlock()
			return
		}
		delete(session, sessionId)
		sessionMutex.Unlock()

		// call handler main
		err := mainFunc(inErr)
		if err != nil {
			chError <- err
		}
	}
}

// record the start of a HTTP request
func reqHandler(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {

	sessionId := ctx.Session
	conn := Connection{Host: ctx.Host, Req: req}
	newReq := req.Clone(context.Background())

	if req.Body != nil {
		conn.ReqBody = NewCaptureReadCloser(req.Body)
		newReq.Body = conn.ReqBody
	}

	sessionMutex.Lock()
	session[sessionId] = &conn
	sessionMutex.Unlock()

	log := newLog()
	defer log.flush()
	log.writef("%s [%d] start %s %s (%s)\n", timestamp(), sessionId, conn.Req.Method, conn.Req.URL.String(), conn.Host)
	return newReq, nil
}

// record a HTTP response
func respHandler(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	sessionId := ctx.Session
	sessionMutex.Lock()
	conn := session[sessionId]
	sessionMutex.Unlock()
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
