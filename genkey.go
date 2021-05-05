// +build ignore

// This code generates an *insecure* ECDSA key with P521 curve
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	gendir   = "./generated"
	keyfile  = "p521privatekey"
	certfile = "rootcert"
)

// a dummy reader with fixed value bytes
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

func genRootCA(key *ecdsa.PrivateKey) (cert *x509.Certificate, err error) {
	serial := big.NewInt(1)
	nb, _ := time.Parse("2006-01-02 15:04:05", "2020-01-01 00:00:00")
	na, _ := time.Parse("2006-01-02 15:04:05", "2039-12-31 23:59:59")

	//
	// Create a CA cert template
	// Note that name information are illegitimate
	//
	var template = &x509.Certificate{

		SerialNumber: serial,

		Subject: pkix.Name{
			Country:            []string{"ZZ"},
			Organization:       []string{"https_capture"},
			OrganizationalUnit: []string{"https_capture"},
			Locality:           []string{"Invalid City"},  // City
			Province:           []string{"Invalid State"}, // State
			StreetAddress:      []string{"Invalid Street Address"},
			PostalCode:         []string{"1-1"},

			SerialNumber: serial.String(),
			CommonName:   "https_capture default dummy Root CA",
		},
		//DNSNames: []string{},
		//EmailAddresses: []string{},
		//IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		//URIs: []*uri.URL

		NotBefore: nb,
		NotAfter:  na,

		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		MaxPathLenZero:        false,
	}

	// get pubkey
	pubKey := key.Public()

	// create cert bytes
	var z zeroReader = 0
	certBytes, err := x509.CreateCertificate(z, template, template, pubKey, key)
	if err != nil {
		return
	}

	// parse the cert into *x509.Certificate
	return x509.ParseCertificate(certBytes)
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

	// write the key in binary DER form
	err = os.MkdirAll(gendir, 0700)
	if err != nil {
		return
	}
	fkey := filepath.Join(gendir, keyfile)
	err = os.WriteFile(fkey+".der", b, 0600)
	if err != nil {
		return
	}
	// note: PEM key is not actually used but generated for convenience
	pb := &pem.Block{Type: "PRIVATE KEY", Bytes: b}
	pembytes := pem.EncodeToMemory(pb)
	err = os.WriteFile(fkey+".pem", pembytes, 0600)
	if err != nil {
		return
	}

	// create the Root Cert
	cert, err := genRootCA(key)
	if err != nil {
		return
	}
	// write the cert in binary DER form
	fcert := filepath.Join(gendir, certfile)
	err = os.WriteFile(fcert+".der", cert.Raw, 0600)
	if err != nil {
		return
	}
	// note: PEM cert is not actually used but generated for convenience
	pb = &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	pembytes = pem.EncodeToMemory(pb)
	err = os.WriteFile(fcert+".pem", pembytes, 0600)
	if err != nil {
		return
	}

	return
}

func main() {
	var err error

	err = run()

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
