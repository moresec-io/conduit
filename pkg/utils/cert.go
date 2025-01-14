package utils

import (
	"crypto/tls"
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
