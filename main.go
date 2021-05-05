package main

//
// A MITM proxy to peek and save HTTP/HTTPS connections to files.
//
// github.com/mixcode, 2021-04
//

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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

	// options
	listenAddress  string = defaultListenAddr
	useBuiltinCert        = false

	captureDir   string = defaultCaptureDir
	listFileName string = filepath.Join(captureDir, defaultListFileName)

	logPostInline   = false
	cleanCaptureDir = false
	tee             = false
	verbose         = false

	force = false

	// cert and key filename (supplied by argument 0 and 1)
	certFile = ""
	keyFile  = ""

	// cert/key
	rootCert   *x509.Certificate
	privateKey interface{}

	// log writer
	listWriter io.Writer
)

// ==============================
// main

// main function 1
//
func run() (err error) {

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
			return fmt.Errorf("no certfiticate file given")
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
				fmt.Printf("A Private key is also read from the cert file\n")
			}
		}
	}

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
		w, e := os.Create(listFileName)
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

	if rootCert == nil {
		err = fmt.Errorf("root cert is nil")
		return
	}

	// build a TLS cert
	var cert tls.Certificate
	cert.Certificate = append(cert.Certificate, rootCert.Raw)
	cert.PrivateKey = privateKey

	// start proxy
	proxy := goproxy.NewProxyHttpServer()

	// prepare a new connection handler
	connectAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&cert),
	}
	var connectHandler goproxy.FuncHttpsHandler = func(host string, proxyCtx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		return connectAction, host
	}
	proxy.OnRequest().HandleConnect(connectHandler)

	// prepare request and response handlers
	proxy.OnRequest().DoFunc(reqHandler)
	proxy.OnResponse().DoFunc(respHandler)

	// set verbose flag of proxy engine
	if verbose {
		proxy.Verbose = goproxy.LOGLEVEL_VERBOSE
	} else {
		proxy.Verbose = goproxy.LOGLEVEL_NONE
	}

	if verbose {
		fmt.Println("proxy started")
	}

	// TODO: handle signal
	return http.ListenAndServe(listenAddress, proxy)
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

func flagUsage() {
	o := flag.CommandLine.Output()

	fmt.Fprintf(o, "\nA HTTP(s) capturing proxy that write contents of HTTP(s) to files.\n")
	fmt.Fprintf(o, "\t2021 github.com/mixcode\n\n")

	fmt.Fprintf(o, "Usage: %s [options] RootCA_filename_pem [key_filename_pem]\n\nOptions:\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {

	//
	// Command-line options
	//

	// help text
	flag.Usage = flagUsage

	// -addr: proxy listen address
	flag.StringVar(&listenAddress, "addr", defaultListenAddr, "proxy listen address")

	// -use-builtin-cert: use built-in cert
	flag.BoolVar(&useBuiltinCert, "use-builtin-cert", useBuiltinCert, "use the built-in Root CA cert (To get the built-in cert, use -print-builtin-cert flag)")

	// -dir: log dir
	flag.StringVar(&captureDir, "dir", defaultCaptureDir, "directory to store the captured files")
	// -log: log list file
	flag.StringVar(&listFileName, "log", listFileName, "filename to store the connections log")
	// -c:  clear the log dir on start
	flag.BoolVar(&cleanCaptureDir, "c", cleanCaptureDir, "clear the capture directory on start")

	// -inline: log POST bodies directly into the log list file
	flag.BoolVar(&logPostInline, "p", logPostInline, "log POST bodies directly into logfile")

	// -tee
	flag.BoolVar(&tee, "tee", tee, "print logs to stdout along with the logfile")

	// -v
	flag.BoolVar(&verbose, "v", verbose, "verbose; print internal proxy log to stdout")

	// -f
	flag.BoolVar(&force, "f", force, "force; overwrite existing file on -generate-cert")

	// -generate-cert : create a CA cert and save it to a file
	var genCertFlag = false
	flag.BoolVar(&genCertFlag, "generate-cert", false, "Generate a self-signed Root CA cert using built-in (insecure) default key and write it to given filename")

	// -print-builtin-cert : print the default built-in CA cert to a file
	var printCertFlag = false
	flag.BoolVar(&printCertFlag, "print-builtin-cert", false, "Write the built-in default insecure Root CA to a file")

	flag.Parse()

	certFile, keyFile = flag.Arg(0), flag.Arg(1)

	var err error
	if genCertFlag {
		// generate a new cert
		err = genCert()
	} else if printCertFlag {
		// print the built-in cert
		err = printCert()
	} else {
		// main
		err = run()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
