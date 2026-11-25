package dmi

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestGenerateCertificate(t *testing.T) {
	certPEM, keyPEM, err := GenerateCertificate("anglerphish")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := pem.Decode([]byte(certPEM))
	if b == nil {
		t.Fatal("no cert pem")
	}
	cert, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() != 2048 {
		t.Fatal("want RSA 2048")
	}
	if d := time.Until(cert.NotAfter); d < 700*24*time.Hour || d > 740*24*time.Hour {
		t.Errorf("NotAfter in %v", d)
	}
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		t.Errorf("key mismatch: %v", err)
	}
	tp, err := CertificateThumbprint(certPEM)
	if err != nil || len(tp) != 27 {
		t.Errorf("thumbprint %q %v", tp, err)
	}
	if _, err := CertificateThumbprint("junk"); err == nil {
		t.Error("want error for junk")
	}
}
