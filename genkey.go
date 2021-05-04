// +build ignore

// This code generates an *insecure* ECDSA key with P521 curve
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

const (
	gendir  = "./generated"
	keyfile = "p521privatekey.der"
)

// a dummy reader with zero-bytes
type zeroReader byte

func (z zeroReader) Read(p []byte) (n int, err error) {
	for n = 0; n < len(p); n++ {
		p[n] = byte(z)
	}
	return
}

// Create a ecdsa Private Key with input of zero-bytes
func generateZeroInputP521Key() (key *ecdsa.PrivateKey, err error) {
	var z zeroReader = 0
	return ecdsa.GenerateKey(elliptic.P521(), z)
}

func run() (err error) {

	// create a key with NON-random input bytes
	key, err := generateZeroInputP521Key()
	if err != nil {
		return
	}
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return
	}

	// write
	err = os.MkdirAll(gendir, 0755)
	if err != nil {
		return
	}
	fname := filepath.Join(gendir, keyfile)
	return os.WriteFile(fname, b, 0644)
}

func main() {
	var err error

	err = run()

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
