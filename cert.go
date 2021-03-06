package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	_ "embed"
)

// A default *insecure* ECDSA key

//go:embed generated/p521privatekey.der
var defaultKeyDer []byte // default key embedded from generated/p521privatekey.der

//go:embed generated/rootcert.der
var defaultRootCADer []byte // default cert embedded from generated/rootcert.def

// p521privatekey.der and rootcert.der are generated by 'go generate'
//go:generate go run genkey.go
var defaultKey *ecdsa.PrivateKey
var defaultRootCA *x509.Certificate

// Create a pair of PEM-encoded PKC8 private key and PKIX public key
func PEMfromECPrivateKey(key *ecdsa.PrivateKey) (privatePem []byte, publicPem []byte, err error) {
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return
	}
	privatePem = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: b,
	})

	pubKey := key.Public()
	b2, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return
	}
	publicPem = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: b2,
	})
	return
}

// Parse PEM encoded bytes and get a ecdsa private key
func ECPrivateKeyfromPEM(pembytes []byte) (privKey *ecdsa.PrivateKey, err error) {
	block, _ := pem.Decode(pembytes)
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return
	}
	privKey, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		err = fmt.Errorf("not an ECDSA key")
	}
	return
}

// Make a semi-unique serial number from the current time. (YYYYMMDDHHMMSSmilsec)
func makeSerial() *big.Int {
	t := time.Now()
	m := big.NewInt(1000000)

	// yyyymmdd
	serial := big.NewInt(int64(t.Year()*10000 + int(t.Month())*100 + t.Day()))
	serial.Mul(serial, m)
	serial.Mul(serial, m)

	// hhmmss
	t2 := big.NewInt(int64(t.Hour()*10000 + t.Minute()*100 + t.Second()))
	t2.Mul(t2, m)
	serial.Add(serial, t2)

	// milliseconds
	t3 := big.NewInt(int64(t.Nanosecond() / 1000))
	serial.Add(serial, t3)

	return serial
}

// Create a ecdsa Private Key with input of zero-bytes
func generateZeroInputP521Key() (key *ecdsa.PrivateKey, err error) {
	var z zeroReader = 0
	return ecdsa.GenerateKey(elliptic.P521(), z)
}

// a dummy reader with fixed value bytes
type zeroReader byte

func (z zeroReader) Read(p []byte) (n int, err error) {
	for n = 0; n < len(p); n++ {
		p[n] = byte(z)
	}
	return
}

//
// Generate a Root CA cert
// DO NOT USE THE CERT ON REAL WORLD USAGE. THE CERT WILL BE ILLEGIMATE.
//
func genRootCA(key *ecdsa.PrivateKey) (cert *x509.Certificate, err error) {

	serial := makeSerial()

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
			CommonName:   "https_capture dummy Root CA",
		},
		//DNSNames: []string{},
		//EmailAddresses: []string{},
		//IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		//URIs: []*uri.URL

		NotBefore: time.Now().Add(-10 * 24 * time.Hour),
		NotAfter:  time.Now().Add(10 * 365 * 24 * time.Hour),

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
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, key)
	if err != nil {
		return
	}

	// parse the cert into *x509.Certificate
	return x509.ParseCertificate(certBytes)
}

func init() {
	k, e := x509.ParsePKCS8PrivateKey(defaultKeyDer)
	if e != nil {
		panic("default key initialization error")
	}
	defaultKey = k.(*ecdsa.PrivateKey)

	defaultRootCA, e = x509.ParseCertificate(defaultRootCADer)
	if e != nil {
		panic("default cert initialization error")
	}
}
