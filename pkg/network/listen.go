package network

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"

	"github.com/moresec-io/conduit/pkg/config"
	"k8s.io/klog/v2"
)

var (
	CiperSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_FALLBACK_SCSV,
	}
)

func Listen(listen *config.Listen) (net.Listener, error) {
	var (
		ln      net.Listener
		network string = listen.Network
		addr    string = listen.Addr
		err     error
	)

	if !listen.TLS.Enable {
		if ln, err = net.Listen(network, addr); err != nil {
			klog.Errorf("listen err: %s, network: %s, addr: %s", err, network, addr)
			return nil, err
		}

	} else {
		// load all certs to listen
		certs := []tls.Certificate{}
		for _, certFile := range listen.TLS.Certs {
			cert, err := tls.LoadX509KeyPair(certFile.Cert, certFile.Key)
			if err != nil {
				klog.Errorf("listen tls load x509 cert err: %s, cert: %s, key: %s", err, certFile.Cert, certFile.Key)
				continue
			}
			certs = append(certs, cert)
		}

		if !listen.TLS.MTLS {
			// tls
			if ln, err = tls.Listen(network, addr, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				CipherSuites: CiperSuites,
				Certificates: certs,
			}); err != nil {
				klog.Errorf("listen tls listen err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}

		} else {
			// mtls, require for edge cert
			// load all ca certs to pool
			caPool := x509.NewCertPool()
			for _, caFile := range listen.TLS.CAs {
				ca, err := os.ReadFile(caFile)
				if err != nil {
					klog.Errorf("listen read ca cert err: %s, file: %s", err, caFile)
					return nil, err
				}
				if !caPool.AppendCertsFromPEM(ca) {
					klog.Warningf("listen append ca cert to ca pool err: %s, file: %s", err, caFile)
					continue
				}
			}
			if ln, err = tls.Listen(network, addr, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				CipherSuites: CiperSuites,
				ClientCAs:    caPool,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: certs,
			}); err != nil {
				klog.Errorf("listen tls listen err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
		}
	}
	return ln, nil
}

func ListenDERMTLS(network, addr string, caraw, certraw, keyraw []byte) (net.Listener, error) {
	// ca
	caPool := x509.NewCertPool()
	ca, err := x509.ParseCertificate(caraw)
	if err != nil {
		return nil, err
	}
	caPool.AddCert(ca)
	// cert
	cert, err := x509.ParseCertificate(certraw)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS1PrivateKey(keyraw)
	if err != nil {
		return nil, err
	}
	tlscert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
	}
	return tls.Listen(network, addr, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: CiperSuites,
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{tlscert},
	})
}
