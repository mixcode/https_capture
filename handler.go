package main

//
// HTTP request, response, and close handlers
//
// Modify the
//
// github.com/mixcode, 2021-04
//

import (
	"context"
	"fmt"
	"mime"
	"net/http"
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

type Connection struct {
	Req      *http.Request
	ReqBody  *CaptureReadCloser
	Resp     *http.Response
	RespBody *CaptureReadCloser
}

// HTTP connection closed; write the result to file
func httpCloseCallback(sessionId int64, conn *Connection) func(error) {

	//
	// This sub-function is called when a NTTP(s) connection has closed.
	// session[sessionId] contains complete history of a connection
	//
	return func(err error) {
		mutex.Lock()
		s, ok := session[sessionId]
		if !ok || s != conn {
			mutex.Unlock()
			return
		}
		delete(session, sessionId)
		mutex.Unlock()

		// Print connection
		writef("%s [%d] end %s %s\n", timestamp(), sessionId, conn.Req.Method, conn.Req.URL.String())

		writef("\t==== Req header ====\n")
		for k, v := range conn.Req.Header {
			writef("\t\t%s: %v\n", k, v)
		}
		if conn.ReqBody.Size > 0 {
			writef("\t---- Req body ----\n")
			contentType := ""
			ct := conn.Req.Header["Content-Type"]
			if len(ct) > 0 {
				contentType = ct[0]
			}
			_, _, ext, isText, _ := mediaType(contentType)

			if logPostInline && isText {
				writef("\t%s\n", string(conn.ReqBody.Buffer[:conn.ReqBody.Size]))
			} else {
				if ext == "" {
					ext = ".bin"
				}
				filename := fmt.Sprintf("%06d_a_request%s", sessionId, ext)
				err = os.WriteFile(filepath.Join(captureDir, filename), conn.ReqBody.Buffer[:conn.ReqBody.Size], 0644)
				if err != nil {
					return
				}
				writef("\t\tfilename: %s\n", filename)
			}
		}

		writef("\t==== Resp header ====\n")
		for k, v := range conn.Resp.Header {
			writef("\t\t%s: %v\n", k, v)
		}

		// Write result body to file
		if conn.RespBody.Size > 0 {

			// determine file name and type
			contentType := ""
			ct := conn.Resp.Header["Content-Type"]
			if len(ct) > 0 {
				contentType = ct[0]
			}

			outfilename := ""
			if disp, ok := conn.Resp.Header["Content-Disposition"]; ok {
				_, param, _ := mime.ParseMediaType(disp[0])
				outfilename = param["filename"]

			}
			if outfilename == "" {
				_, outfilename = path.Split(conn.Req.URL.EscapedPath())
			}
			if outfilename == "" {
				outfilename = "unknown.bin"
			}
			outfilename = fmt.Sprintf("%06d_b_%s", sessionId, outfilename)

			ext := path.Ext(outfilename)
			filebody := outfilename[:len(outfilename)-len(ext)]
			if ext == "" {
				_, _, ext, _, _ = mediaType(contentType)
				if ext == "" {
					// unknown file type
					ext = ".bin"
				}
			}
			outfilename = filebody + ext
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

			writef("\t---- Resp body ----\n")
			writef("\t\t(saved to: [%s])\n", shortname)
			//writef("\t\(filename: [%s])\n", outfilename)

			err = os.WriteFile(filepath.Join(captureDir, shortname), conn.RespBody.Buffer[:conn.RespBody.Size], 0644)
			if err != nil {
				return
			}
			//writef("\t---- Resp body [%s] ----\n", contentType)
			//writef("\t%s\n", string(conn.RespBody.Buffer[:conn.RespBody.Size]))
		}
		writef("\n")
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

	writef("%s [%d] start %s %s\n", timestamp(), sessionId, conn.Req.Method, conn.Req.URL.String())
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

// write message to the logfile
func writef(format string, arg ...interface{}) {
	fmt.Fprintf(listWriter, format, arg...)
	if tee {
		fmt.Printf(format, arg...)
	}
}
