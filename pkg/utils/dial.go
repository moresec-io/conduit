package utils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/rand"
	"net"
	"os"

	"github.com/moresec-io/conduit/pkg/config"
	"k8s.io/klog/v2"
)

func DialRandom(dial *config.Dial) (net.Conn, error) {
	idx := rand.Intn(len(dial.Addrs))
	return Dial(dial, idx)
}

func Dial(dial *config.Dial, index int) (net.Conn, error) {
	if len(dial.Addrs) == 0 {
		return nil, errors.New("illegal addrs")
	}
	var (
		network string = dial.Network
		addr    string
	)
	if index < len(dial.Addrs) {
		addr = dial.Addrs[index]
	} else {
		addr = dial.Addrs[0]
	}

	if !dial.TLS.Enable {
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return conn, err
	} else {
		// load all certs to dial
		certs := []tls.Certificate{}
		for _, certFile := range dial.TLS.Certs {
			cert, err := tls.LoadX509KeyPair(certFile.Cert, certFile.Key)
			if err != nil {
				klog.Errorf("dial, tls load x509 cert err: %s, cert: %s, key: %s", err, certFile.Cert, certFile.Key)
				continue
			}
			certs = append(certs, cert)
		}

		if !dial.TLS.MTLS {
			// tls
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: certs,
				// it's user's call to verify the server certs or not.
				InsecureSkipVerify: dial.TLS.InsecureSkipVerify,
			})
			if err != nil {
				klog.Errorf("tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		} else {
			// mtls, dial with our certs
			// load all ca certs to pool
			caPool := x509.NewCertPool()
			for _, caFile := range dial.TLS.CAs {
				ca, err := os.ReadFile(caFile)
				if err != nil {
					klog.Errorf("dial read ca cert err: %s, file: %s", err, caFile)
					return nil, err
				}
				if !caPool.AppendCertsFromPEM(ca) {
					klog.Warningf("dial append ca cert to ca pool err: %s, file: %s", err, caFile)
					continue
				}
			}
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: certs,
				// we should not skip the verify.
				InsecureSkipVerify: dial.TLS.InsecureSkipVerify,
				RootCAs:            caPool,
			})
			if err != nil {
				klog.Errorf("dial tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		}
	}
}
