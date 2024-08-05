package network

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/rand"
	"net"
	"os"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/config"
)

func DialRandom(dial *config.Dial) (net.Conn, error) {
	idx := rand.Intn(len(dial.Addresses))
	return Dial(dial, idx)
}

type TLS struct {
	Enable             bool
	MTLS               bool
	CAPool             *x509.CertPool
	Certs              []tls.Certificate
	InsecureSkipVerify bool
}

type DialConfig struct {
	Netwotk string
	Addrs   []string
	TLS     *TLS
}

func DialRandomWithConfig(dialconfig *DialConfig) (net.Conn, error) {
	idx := rand.Intn(len(dialconfig.Addrs))
	return DialWithConfig(dialconfig, idx)
}

func DialWithConfig(dialconfig *DialConfig, index int) (net.Conn, error) {
	if len(dialconfig.Addrs) == 0 {
		return nil, errors.New("illegal addrs")
	}
	var (
		network string = dialconfig.Netwotk
		addr    string
	)
	if index < len(dialconfig.Addrs) {
		addr = dialconfig.Addrs[index]
	} else {
		addr = dialconfig.Addrs[0]
	}

	if dialconfig.TLS == nil || !dialconfig.TLS.Enable {
		conn, err := net.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		return conn, err
	} else {
		if !dialconfig.TLS.MTLS {
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: dialconfig.TLS.Certs,
				// it's user's call to verify the server certs or not.
				InsecureSkipVerify: dialconfig.TLS.InsecureSkipVerify,
				RootCAs:            dialconfig.TLS.CAPool,
			})
			if err != nil {
				log.Errorf("tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		} else {
			conn, err := tls.Dial(network, addr, &tls.Config{
				Certificates: dialconfig.TLS.Certs,
				// it's user's call to verify the server certs or not.
				InsecureSkipVerify: dialconfig.TLS.InsecureSkipVerify,
				RootCAs:            dialconfig.TLS.CAPool,
			})
			if err != nil {
				log.Errorf("dial tls dial err: %s, network: %s, addr: %s", err, network, addr)
				return nil, err
			}
			return conn, nil
		}
	}
}

func ConvertDialConfig(dial *config.Dial) (*DialConfig, error) {
	dialConfig := &DialConfig{
		Netwotk: dial.Network,
		Addrs:   dial.Addresses,
	}
	if dial.TLS != nil && dial.TLS.Enable {
		// load all certs to dial
		certs := []tls.Certificate{}
		for _, certFile := range dial.TLS.Certs {
			cert, err := tls.LoadX509KeyPair(certFile.Cert, certFile.Key)
			if err != nil {
				log.Errorf("dial, tls load x509 cert err: %s, cert: %s, key: %s", err, certFile.Cert, certFile.Key)
				return nil, err
			}
			certs = append(certs, cert)
		}
		tls := &TLS{
			Enable:             true,
			Certs:              certs,
			InsecureSkipVerify: dial.TLS.InsecureSkipVerify,
		}
		if dial.TLS.MTLS {
			// mtls, dial with our certs
			// load all ca certs to pool
			caPool := x509.NewCertPool()
			for _, caFile := range dial.TLS.CAs {
				ca, err := os.ReadFile(caFile)
				if err != nil {
					log.Errorf("dial read ca cert err: %s, file: %s", err, caFile)
					return nil, err
				}
				if !caPool.AppendCertsFromPEM(ca) {
					log.Warnf("dial append ca cert to ca pool err: %s, file: %s", err, caFile)
					return nil, err
				}
			}
			tls.CAPool = caPool
		}
		dialConfig.TLS = tls
	}
	return dialConfig, nil
}

func Dial(dial *config.Dial, index int) (net.Conn, error) {
	if len(dial.Addresses) == 0 {
		return nil, errors.New("illegal addrs")
	}
	dialConfig, err := ConvertDialConfig(dial)
	if err != nil {
		return nil, err
	}
	return DialWithConfig(dialConfig, index)
}
