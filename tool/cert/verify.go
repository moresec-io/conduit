package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

var (
	ca = `-----BEGIN CERTIFICATE-----
MIIDDzCCAfegAwIBAgIRAJKTqHnYUTZJIrMCKQ6mZoIwDQYJKoZIhvcNAQELBQAw
ITEJMAcGA1UEChMAMRQwEgYDVQQDEwtjb25kdWl0LmNvbTAeFw0yNTAxMTIxNDIw
NTZaFw0yNjAxMTIxNDIwNTZaMCExCTAHBgNVBAoTADEUMBIGA1UEAxMLY29uZHVp
dC5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDVDjcWDAPzHx5k
ScIXOhybqXJYI5rR3suGzh8PyRiQJYFYNXM9TpiBAjEz9+Nvko10PkXy996ELYaX
Jewt3K/cRm+DytskuL+AmLYBChferG5Tnvfhw3N2FJN6nbDKTL3Zol9MOkv2shLc
KNBosq3vNqSFQNp0n3lSjJ7D6nUFkcpVvumJgZ9p0ynEUI+BtKxggnm8rUaXhu4O
I2IX0UG/4I7I2HGLWooPjMdd8HU3fVF8V8E1xcfnBC5fL+rH504Fru5TGZddd8vc
QitCTCH405TZerXuKpK2iRqd4xaFaVirI+xupBfVO3fQBQmBiokiHrzPbMzeqd83
qnz+FQeDAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBQyQ9e6GWJY2fz4ITGBjzrHMyfYqjANBgkqhkiG9w0BAQsFAAOC
AQEAumlzzPZbrMoBT6k/iDuIX1agMeJ9WxfbYVnkycfD8bLakoeMnjfWNhsN1hCD
nNRKTt99R8wzHTDFKvdXH1VVwAuyo8U7u4ZCnr9bv9qRzIk5PkhopwfB+IH831bY
/24AHFHT4P6nxY2hUsxTn3cVRwwomhklrtAjwJ78uE5M50RPGc883kY7vD5ZrtnG
5Dmlarzm+NRpEIxV6iaTjUXTIGypDmRJnzLr7JsvZNaPZdccVu5I1B3ro3y5/5o0
IcDyX0W2b8x1wo3/xiwmXLIczZc8MCVBGf4pWs6f+LPffZKofXZgFY/5QgWMcqPc
4ys536wqR8HibvnjnGd5dxRorw==
-----END CERTIFICATE-----`
	cert = `-----BEGIN CERTIFICATE-----
MIIDFzCCAf+gAwIBAgIQf8EJIt7eP/VSraLKaHqd7jANBgkqhkiG9w0BAQsFADAh
MQkwBwYDVQQKEwAxFDASBgNVBAMTC2NvbmR1aXQuY29tMB4XDTI1MDExMjE0Mzgz
N1oXDTI2MDExMjE0MzgzN1owFjEUMBIGA1UEChMLbW9yZXNlYy5jb20wggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCaYNtkGaB7J7dFcgV6S6M7lEbNjspt
21cMt2/x3qSJ44Rn6Pbj43wEaI733MA4paLn8rnl+eCDVOSCQ2be2qcLCBnDZDlu
qrFbjkZzMpE9rBriYhslnoItjRak8/juU4hMEvE1PFrpUhRmPmuFxQL1vHJiO+4f
DN3DGTOc4df+kDg62uZdxSMIdF+SQ9fEaqTe/tqgAc1rht3sK7eljd/n6HyatLKe
YVE2rww7iT6TUmPTsA5c2o4to2sItaVXShzxJ7xewNl10CuhMdbLGByw1PArHA8S
607DTbTb+H6ysdUmpYOkgyA8gRgzOaDNwXMW/b+UlSZQCs2VbFKJE7WzAgMBAAGj
VjBUMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDAjAMBgNVHRMB
Af8EAjAAMB8GA1UdIwQYMBaAFDJD17oZYljZ/PghMYGPOsczJ9iqMA0GCSqGSIb3
DQEBCwUAA4IBAQBWicZ04IkxM4toYD3dq2WUx+KPgOVgYxK4HHfe87mWBkewSwpM
1mcyZ0kq87iHDTnLLbmKxsWE80CRhyWTyqZOHiHJGWXxiwIoXcmwp4jtVFkwcfqW
3JDWlHbII/vndw+3RpkjSvbBdjUvfadey/O9RwNXJ6aXVgX5kEbWvKdYKmL4yy6G
xQzldf+jTNBfVwCnkCYWv5+aV34Kku4tDuIuZa/A7Tg2AWF+/eauTKNhdCFUgY7y
Wlypn4qaavrsp4QBCyriz350yL7JDI/Ls1SPVJvdWdde6Ij6U/mPgnJWjxK0Q3LF
K0BceXrOrb0X4xB1IClTEXNX61NiGEkovcYr
-----END CERTIFICATE-----`
)

func main() {
	caBlock, _ := pem.Decode([]byte(ca))
	if caBlock == nil || caBlock.Type != "CERTIFICATE" {
		fmt.Println("Invalid CA certificate")
		return
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		fmt.Println("Failed to parse CA certificate:", err)
		return
	}

	certBlock, _ := pem.Decode([]byte(cert))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		fmt.Println("Invalid certificate")
		return
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		fmt.Println("Failed to parse certificate:", err)
		return
	}
	// 创建一个证书池并添加 CA 证书
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	// 设置验证选项
	opts := x509.VerifyOptions{
		Roots:     caCertPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// 验证证书
	if _, err := cert.Verify(opts); err != nil {
		fmt.Println("Certificate verification failed:", err)
	} else {
		fmt.Println("Certificate successfully verified against the CA")
	}
}
