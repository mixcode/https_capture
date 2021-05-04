package main

import (
	//"crypto/x509"
	"fmt"
	"testing"
)

func TestGenCert(t *testing.T) {

	var err error

	// Compare embedded key
	key, err := generateZeroInputP521Key()
	if err != nil {
		t.Fatal(err)
	}
	if !key.Equal(defaultKey) {
		t.Fatalf("incorrect default key")
	}

	ca, err := genRootCA(key)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ca.Issuer)
	fmt.Println(ca.Subject)
}

func TestPEM(t * testing.T) {

	// PEM encoding test
	privPem, publicPem, err := PEMfromECPrivateKey(defaultKey)
	if err != nil {
		t.Error(err)
	}

	key2, err := ECPrivateKeyfromPEM(privPem)
	if err != nil {
		t.Error(err)
	}

	pem2, _, err := PEMfromECPrivateKey(key2)
	if err != nil {
		t.Error(err)
	}

	if string(privPem) != string(pem2) {
		t.Errorf("key does not match")
	}

	fmt.Println(string(privPem))
	fmt.Println(string(publicPem))

}
