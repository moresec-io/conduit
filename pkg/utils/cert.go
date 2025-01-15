package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
)

func TLSCertToPEM(cert *tls.Certificate) string {
	chain := ""
	for _, elem := range cert.Certificate {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: elem,
		}
		str := string(pem.EncodeToMemory(block))
		chain += str + "\n"
	}
	return chain
}

func X509CertoToPem(cert *x509.Certificate) string {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return string(pem.EncodeToMemory(block))
}
