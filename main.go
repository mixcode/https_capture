package main

//
// A MITM proxy to peek and save HTTP/HTTPS connections to files.
//
// github.com/mixcode, 2021-04
//

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	//"github.com/elazarl/goproxy"
	"github.com/mixcode/goproxy" // a clone of elazarl/goproxy with fixes for TLS SNI
)

const (
	defaultListenAddr  = ":38080"
	defaultCaptureDir  = "./captured"
	defaultLogFileName = "log.txt"

	filenameMaxLen = 32
)

var (

	// options
	listenAddress  string = defaultListenAddr
	useBuiltinCert        = false

	captureDir  string = defaultCaptureDir
	logFileName string = filepath.Join(captureDir, defaultLogFileName)

	rawPostForm      = false // print x-www-form-urlencoded in raw querystring
	logPostInline    = false
	logPostInlineAll = false
	//rawCompressedBody	= false	// if true, body is stored in its raw (maybe compressed) form
	cleanCaptureDir = false
	tee             = false
	verbose         = false

	force = false

	// non-TLS servers for connect
	nonTLSPort = make(map[int]bool)
	//nonTLSAddr = make(map[string]bool)

	// cert and key filename (supplied by argument 0 and 1)
	certFile = ""
	keyFile  = ""

	// cert/key
	rootCert   *x509.Certificate
	privateKey interface{}

	// error handling
	chError = make(chan error, 1)

	// host:port matcher
	mMatchHost = regexp.MustCompile(`^(.*):(\d+)$`)
)

// ==============================
// main

// main function 1
//
func runProxy() (err error) {

	// prepare the cert
	rootCert = defaultRootCA
	privateKey = defaultKey
	if useBuiltinCert {
		// use the built-in cert
		if verbose {
			fmt.Printf("Using the built-in cert\n")
		}
	} else {
		// load the cert
		if certFile == "" {
			return fmt.Errorf("no certfiticate file supplied. A Root CA cert in PEM format must be given.\n(If you don't have a cert, '%[1]s -generate-cert' will give you a dummy insecure self-signed cert. Be sure to install the cert to your web client and try again. See '%[1]s -help' for all options)", os.Args[0])
		}
		var pm, rest []byte
		var pb *pem.Block
		pm, err = os.ReadFile(certFile)
		if err != nil {
			return
		}
		pb, rest = pem.Decode(pm)
		if pb == nil {
			return fmt.Errorf("cert file contains no PEM block")
		}
		rootCert, err = x509.ParseCertificate(pb.Bytes)
		if err != nil {
			return
		}
		if verbose {
			fmt.Printf("Root CA cert read from '%s'\n", certFile)
		}

		if keyFile != "" {
			// open keyfile
			pm, err = os.ReadFile(keyFile)
			if err != nil {
				return
			}
			pb, _ = pem.Decode(pm)
			if pb == nil {
				return fmt.Errorf("key file contains no PEM block")
			}
			privateKey, err = x509.ParsePKCS8PrivateKey(pb.Bytes)
			if err != nil {
				return
			}
			if verbose {
				fmt.Printf("Private key read from '%s'\n", keyFile)
			}
		} else {
			// check whether there is an additional key at the end of the cert
			pb, _ = pem.Decode(rest)
			if pb != nil {
				privateKey, err = x509.ParsePKCS8PrivateKey(pb.Bytes)
				if err != nil {
					return
				}
				if verbose {
					fmt.Printf("A Private key is also read from the cert file\n")
				}
			}
		}
	}

	// prepare the capturing directory
	err = os.MkdirAll(captureDir, 0755)
	if err != nil {
		return
	}
	if cleanCaptureDir {
		emptyDir(captureDir)
	}

	// prepare the logfile
	if logFileName == "-" {
		logWriter = os.Stdout
	} else {
		w, e := os.Create(logFileName)
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
		logWriter = w
	}
	startLog()
	defer stopLog()

	// build a TLS cert
	if rootCert == nil {
		err = fmt.Errorf("root cert is nil")
		return
	}
	var cert tls.Certificate
	cert.Certificate = append(cert.Certificate, rootCert.Raw)
	cert.PrivateKey = privateKey

	// prepare the proxy engine
	proxy := goproxy.NewProxyHttpServer()

	tlsConnectAction := &goproxy.ConnectAction{ // new connection handler
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&cert),
	}
	rawConnectAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectHTTPMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&cert),
	}
	var connectHandler goproxy.FuncHttpsHandler = func(host string, proxyCtx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		m := mMatchHost.FindStringSubmatch(host)
		if m != nil {
			// see the port number and test for non-TLS ports
			port, e := strconv.Atoi(m[2])
			if e == nil && nonTLSPort[port] {
				fmt.Printf("RAW CONNECT: host[%s], %v\n", host, proxyCtx.Req)
				return rawConnectAction, host
			}
		}
		fmt.Printf("TLS CONNECT: host[%s], %v\n", host, proxyCtx.Req)

		return tlsConnectAction, host
	}
	proxy.OnRequest().HandleConnect(connectHandler)

	proxy.OnRequest().DoFunc(reqHandler)   // http request handler
	proxy.OnResponse().DoFunc(respHandler) // http response handler

	if verbose {
		proxy.Verbose = goproxy.LOGLEVEL_VERBOSE
	} else {
		proxy.Verbose = goproxy.LOGLEVEL_NONE
	}

	// start the proxy engine
	var wg sync.WaitGroup
	server := &http.Server{Addr: listenAddress, Handler: proxy}

	wg.Add(1)
	go func() {
		defer wg.Done()
		e := server.ListenAndServe()
		if e == http.ErrServerClosed {
			e = nil
		}
		chError <- e // send a nil error
	}()
	if verbose {
		fmt.Println("proxy started")
	}

	// wait for an OS signal
	chSignal := make(chan os.Signal, 1)
	signal.Notify(chSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case s := <-chSignal: // a signal received
		if verbose {
			fmt.Printf("received an OS signal (%v)\n", s)
		}

	case err = <-chError: // some error
		if err != nil {
			if verbose {
				fmt.Printf("an error detected: %v\n", err)
			}
		}
	}

	// terminate the proxy
	if verbose {
		fmt.Println("terminating proxy...")
	}
	if e := server.Shutdown(context.TODO()); err == nil {
		err = e
	}
	wg.Wait()
	if verbose {
		fmt.Println("proxy terminated")
	}

	return
}

// main function 2
// create a cert and write to a file
func genCert() (err error) {

	var w io.Writer
	if certFile == "" || certFile == "-" {
		w = os.Stdout
	} else {
		if !promptOverwriteFile(certFile) {
			err = fmt.Errorf("Aborted")
			return
		}
		fo, e := os.Create(certFile)
		if e != nil {
			err = e
			return
		}
		defer func() {
			e := fo.Close()
			if err == nil {
				err = e
			}
		}()
		w = fo
	}
	ca, err := genRootCA(defaultKey)
	if err != nil {
		return
	}
	block := &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}
	err = pem.Encode(w, block)
	if err != nil {
		return
	}
	if verbose && certFile != "" {
		fmt.Printf("New Root CA created and saved to '%s'\n", certFile)
	}
	return
}

// main function 3
// print the default cert to a file
func printCert() (err error) {
	var w io.Writer
	if certFile == "" || certFile == "-" {
		w = os.Stdout
	} else {
		if !promptOverwriteFile(certFile) {
			err = fmt.Errorf("Aborted")
			return
		}
		fo, e := os.Create(certFile)
		if e != nil {
			err = e
			return
		}
		defer func() {
			e := fo.Close()
			if err == nil {
				err = e
			}
		}()
		w = fo
	}
	block := &pem.Block{Type: "CERTIFICATE", Bytes: defaultRootCA.Raw}
	err = pem.Encode(w, block)
	if err != nil {
		return
	}
	if verbose && certFile != "" {
		fmt.Printf("Default Root CA certificate saved to '%s'\n", certFile)
	}
	return
}

//==================================
// utilities
//==================================

// check for file existency and prompt for overwriting it
func promptOverwriteFile(filename string) bool {
	if force {
		// overwrite it no matther of what
		return true
	}
	_, e := os.Stat(certFile)
	if os.IsNotExist(e) {
		return true
	}
	fmt.Printf("File %s exists. Overwrite? [y/N] ", filename)
	return promptYN(false)
}

func promptYN(defaultValue bool) bool {
	r := bufio.NewReader(os.Stdin)
	b, err := r.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	c := b[0]
	if c == 'Y' || c == 'y' {
		return true
	} else if c == 'N' || c == 'n' {
		return false
	}
	return defaultValue
}

// remove files in a directory
func emptyDir(path string) (err error) {
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

//=================================
// program startup
//=================================

func runMain() (err error) {

	//
	// command-line options
	//

	// help text
	flag.Usage = func() {
		o := flag.CommandLine.Output()

		fmt.Fprintf(o, "\nA HTTP(s) capturing proxy that write contents of HTTP(s) to files.\n")
		fmt.Fprintf(o, "\t2021 github.com/mixcode\n\n")

		fmt.Fprintf(o, "Usage: %s [options] RootCA_pem_file [privkey_pem_file]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}

	// -addr: proxy listen address
	flag.StringVar(&listenAddress, "addr", defaultListenAddr, "proxy listen address")

	// -use-builtin-cert: use built-in cert
	flag.BoolVar(&useBuiltinCert, "use-builtin-cert", useBuiltinCert, "use the built-in Root CA cert (To get the built-in cert, use -print-builtin-cert flag)")

	// -dir: log dir
	flag.StringVar(&captureDir, "dir", defaultCaptureDir, "directory to store the captured files")
	// -log: log list file
	flag.StringVar(&logFileName, "log", logFileName, "filename to store the connections log")
	// -c:  clear the log dir on start
	flag.BoolVar(&cleanCaptureDir, "c", cleanCaptureDir, "clear the capture directory on start")

	// -p: log POST bodies directly into the log list file
	flag.BoolVar(&logPostInline, "p", logPostInline, "log POST request bodies directly into the logfile")
	flag.BoolVar(&logPostInlineAll, "pall", logPostInlineAll, "log POST request bodies directly into the logfile, even if it is known as a binary")

	// -rawform: log x-www-form-urlencoded data as raw query strings
	flag.BoolVar(&rawPostForm, "rawform", rawPostForm, "log x-www-form-urlencoded forms in raw query string")

	// -rawbody: save req/resp bodies in its raw (maybe compressed) form
	//flag.BoolVar(&rawCompressedBody, "rawbody", rawCompressedBody, "save req/resp podies in its raw (maybe compressed) form")

	// -tee
	flag.BoolVar(&tee, "tee", tee, "print logs to stdout along with the logfile")

	// -v
	flag.BoolVar(&verbose, "v", verbose, "verbose; print internal proxy log to stdout")

	// -f
	flag.BoolVar(&force, "f", force, "force; overwrite existing file on -generate-cert")

	// -generate-cert : create a CA cert and save it to a file
	var genCertFlag = false
	flag.BoolVar(&genCertFlag, "generate-cert", false, "generate a self-signed Root CA cert using built-in (insecure) default key and write it to given filename")

	// -print-builtin-cert : print the default built-in CA cert to a file
	var printCertFlag = false
	flag.BoolVar(&printCertFlag, "print-builtin-cert", false, "write the built-in default insecure Root CA to a file")

	// -non-tls-ports: list of non-TLS connect ports
	var nonTLSPortList = ""
	flag.StringVar(&nonTLSPortList, "non-tls-ports", "80", "comma-separated list of non-TLS ports")

	// Save content types
	var contentTypes = ""
	flag.StringVar(&contentTypes, "contenttypes", "", "comma-separated list of content types to be recorded.")

	var contentNameMatch = ""
	flag.StringVar(&contentNameMatch, "contentname", "", "regex match of content names to be recorded.")

	flag.Parse()

	// set arguments
	certFile, keyFile = flag.Arg(0), flag.Arg(1)

	// main function
	if genCertFlag {
		// generate a new cert
		return genCert()
	} else if printCertFlag {
		// print the built-in cert
		return printCert()
	} else {
		// Init flags

		// set non-TLS port numbers
		s := strings.Split(nonTLSPortList, ",")
		for _, p := range s {
			portnum, e := strconv.Atoi(strings.TrimSpace(p))
			if e == nil {
				nonTLSPort[portnum] = true
			}
		}

		// set log file name
		logFileName = filepath.Join(captureDir, defaultLogFileName)

		// set content type match
		if contentTypes != "" {
			saveContentType = make(map[string]bool)
			for _, s := range strings.Split(contentTypes, ",") {
				saveContentType[s] = true
			}
		}

		// filename match
		if contentNameMatch != "" {
			m, e := regexp.Compile(contentNameMatch)
			if e != nil {
				return e
			}
			saveIfMatch = make([]*regexp.Regexp, 1)
			saveIfMatch[0] = m
		}

		// run proxy
		err = runProxy()
	}
	return
}

func main() {

	err := runMain()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
