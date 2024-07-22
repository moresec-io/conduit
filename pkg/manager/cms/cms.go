package cms

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strconv"
	"strings"
	"time"
)

type CMS struct{}

// notAfter: 1,2,3 means now add 1 year 2 months and 3 days
func (cms *CMS) GenCA(notAfter string) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	years, months, days := getDate(notAfter)
	now := time.Now()
	catemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Conduit"},
			CommonName:   "Conduit CA",
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(years, months, days),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	// ASN.1 DER
	ca, err := x509.CreateCertificate(rand.Reader, &catemplate, &catemplate, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	return ca, x509.MarshalPKCS1PrivateKey(key), nil
}

func (cms *CMS) GenCert(cacert []byte, cakey []byte, notAfter string) ([]byte, []byte, error) {
	ca, err := x509.ParseCertificate(cacert)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS1PrivateKey(cakey)
	if err != nil {
		return nil, nil, err
	}

	signkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	years, months, days := getDate(notAfter)
	certtemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Conduit"},
			CommonName:   "Conduit",
		},
		NotBefore: now,
		NotAfter:  now.AddDate(years, months, days),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	signcert, err := x509.CreateCertificate(rand.Reader, &certtemplate, ca, signkey.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	return signcert, x509.MarshalPKCS1PrivateKey(signkey), nil
}

func getDate(str string) (int, int, int) {
	elems := strings.Split(str, ",")
	years, months, days := 0, 0, 0
	for index, elem := range elems {
		value, _ := strconv.Atoi(elem) // if err not nil, then
		switch index {
		case 0:
			years = value
		case 1:
			months = value
		case 2:
			days = value
		}
	}
	return years, months, days
}
