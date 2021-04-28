package main

//
// A MITM proxy to peek and save HTTP/HTTPS connections to files.
//
// github.com/mixcode, 2021-04
//

import (
	"context"
	"flag"
	"fmt"
	"io"
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

const (
	defaultListenAddr   = ":38080"
	defaultCaptureDir   = "./captured"
	defaultListFileName = "log.txt"

	filenameMaxLen = 32
)

var (

	// parameters
	listenAddress string = defaultListenAddr
	captureDir    string = defaultCaptureDir
	listFileName  string = defaultListFileName

	logPostInline = true
	tee           = false
	verbose       = false
	cleanCaptureDir = false

	// output
	listWriter io.Writer

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

// Record the start of a HTTP request
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

// Record a HTTP response
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
		conn.RespBody = NewCaptureReadCloserCallback(resp.Body, closeCallback(sessionId, conn))
		resp.Body = conn.RespBody
	}
	return resp
}

// HTTP connection closed; write the result to file
func closeCallback(sessionId int64, conn *Connection) func(error) {
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
			writef("\t\t[%s]: %v\n", k, v)
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
			writef("\t\t[%s]: %v\n", k, v)
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
				shortname = shortname[:filenameMaxLen-len(ext)] + ext
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

// =============================
// utils

func timestamp() string {
	return time.Now().Format(time.RFC3339)
}

func writef(format string, arg ...interface{}) {
	fmt.Fprintf(listWriter, format, arg...)
	if tee {
		fmt.Printf(format, arg...)
	}
}

// remove files in a directory
func emptyDir(path string) (err error){
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := filepath.Join(path, f.Name())
		os.Remove(name)
	}
	return
}

// ==============================
// main

func run() (err error) {

	err = os.MkdirAll(captureDir, 0755)
	if err != nil {
		return
	}
	if cleanCaptureDir {
		emptyDir(captureDir)
	}


	if listFileName == "-" {
		listWriter = os.Stdout
	} else {
		fn := filepath.Join(captureDir, listFileName)
		w, e := os.Create(fn)
		if e != nil {
			err = e
			return
		}
		defer func() {
			e := w.Close()
			if err == nil {
				err = e
			}
		}()
		listWriter = w
	}

	// start proxy
	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest().DoFunc(reqHandler)
	proxy.OnResponse().DoFunc(respHandler)
	proxy.Verbose = verbose
	return http.ListenAndServe(listenAddress, proxy)
}

func main() {

	flag.StringVar(&listenAddress, "addr", defaultListenAddr, "proxy listen address")

	flag.StringVar(&captureDir, "dir", defaultCaptureDir, "directory to store the captured files")
	flag.StringVar(&listFileName, "log", defaultListFileName, "filename to store the connections log")
	flag.BoolVar(&cleanCaptureDir, "e", false, "erase the files in the capture directory on start")

	flag.BoolVar(&logPostInline, "inline", false, "log POST bodies directly into logfile")
	flag.BoolVar(&tee, "t", false, "tee; make logs also printed to stdout")

	flag.BoolVar(&verbose, "v", false, "verbose; print internal proxy log to stdout")

	flag.Parse()

	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
