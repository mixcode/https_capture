package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestGenCert(t *testing.T) {

	var err error

	// check default key is valid non-random input key
	key, err := generateZeroInputP521Key()
	if err != nil {
		t.Fatal(err)
	}
	if !key.Equal(defaultKey) {
		t.Fatalf("incorrect default key")
	}

	// test default cert
	if defaultRootCA.Subject.SerialNumber != "1" ||
		defaultRootCA.Subject.CommonName != "https_capture default dummy Root CA" {

		t.Fatalf("incorrect default Root CA")
	}
	//fmt.Println(defaultRootCA.Subject)

	ca, err := genRootCA(key)
	if err != nil {
		t.Error(err)
	}
	_ = ca
	//fmt.Println(ca.Issuer)
	//fmt.Println(ca.Subject)
}

func TestPEM(t *testing.T) {

	// PEM encoding test
	privPem, publicPem, err := PEMfromECPrivateKey(defaultKey)
	if err != nil {
		t.Error(err)
	}
	_ = publicPem

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

	//fmt.Println(string(privPem))
	//fmt.Println(string(publicPem))
}

func TestRootCAPEM(t *testing.T) {
	var CA_CERT = []byte(`-----BEGIN CERTIFICATE-----
MIIDfTCCAt6gAwIBAgIBATAKBggqhkjOPQQDBDCB1TELMAkGA1UEBhMCWloxFjAU
BgNVBAgTDUludmFsaWQgU3RhdGUxFTATBgNVBAcTDEludmFsaWQgQ2l0eTEfMB0G
A1UECRMWSW52YWxpZCBTdHJlZXQgQWRkcmVzczEMMAoGA1UEERMDMS0xMRYwFAYD
VQQKDA1odHRwc19jYXB0dXJlMRYwFAYDVQQLDA1odHRwc19jYXB0dXJlMSwwKgYD
VQQDDCNodHRwc19jYXB0dXJlIGRlZmF1bHQgZHVtbXkgUm9vdCBDQTEKMAgGA1UE
BRMBMTAeFw0yMDAxMDEwMDAwMDBaFw0zOTEyMzEyMzU5NTlaMIHVMQswCQYDVQQG
EwJaWjEWMBQGA1UECBMNSW52YWxpZCBTdGF0ZTEVMBMGA1UEBxMMSW52YWxpZCBD
aXR5MR8wHQYDVQQJExZJbnZhbGlkIFN0cmVldCBBZGRyZXNzMQwwCgYDVQQREwMx
LTExFjAUBgNVBAoMDWh0dHBzX2NhcHR1cmUxFjAUBgNVBAsMDWh0dHBzX2NhcHR1
cmUxLDAqBgNVBAMMI2h0dHBzX2NhcHR1cmUgZGVmYXVsdCBkdW1teSBSb290IENB
MQowCAYDVQQFEwExMIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQAxoWOBrcEBOnN
nj7LZiOVtEKcZIE5BT+1Ifgor2BrTT26oUted+/nWSj+HcEnov+o3jNIs8GFakKb
+X5+McLlvWYBGDkpaniaO8AEXIpftCx9G9mY9URJV5tEaBevvRcnPmYsl+5ymV70
JkDFULkBP60HYTU8cIaicsJAiL6Udp/RZlCjWjBYMA4GA1UdDwEB/wQEAwIBBjAT
BgNVHSUEDDAKBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/AgECMB0GA1UdDgQW
BBTA99OY7rqv1KfsLPoZBoIMymuqRjAKBggqhkjOPQQDBAOBjAAwgYgCQgHHeKr/
zZsK7sYGd3L3Fy/fQK9gYab6FT8mpfkwSXy7YclCGcA6sjE2tFQow9RV/oz16XJg
0YEmC3/LF2CbtgG1PQJCAet6IEVFZPZwkjZ+pMvzNipbZIBgxtVqbClFfOyKM7JT
s4+BHCG5l7nMa6mrOPUYNZQ1/2+bINw9U2Ti8iRIVjPF
-----END CERTIFICATE-----
`)

	pb, _ := pem.Decode(CA_CERT)
	var o bytes.Buffer
	err := pem.Encode(&o, pb)
	if err != nil {
		t.Fatal(err)
	}
	if o.String() != string(CA_CERT) {
		t.Errorf("cert mismatching")
	}
	c, err := x509.ParseCertificate(pb.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Equal(defaultRootCA) {
		t.Fatalf("PEM not matched to the root CA")
	}

}
