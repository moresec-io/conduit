package repo

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/jumboframes/armorigo/log"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"gorm.io/gorm"
)

type dao struct {
	db *gorm.DB
}

func NewDao(config *config.Config) (*dao, error) {
	return nil, nil
}

func getSQLUri() (string, error) {
	return "", nil
}

func getTLSConfig(conf gconfig.TLS) (*tls.Config, error) {
	tlsconfig := &tls.Config{
		InsecureSkipVerify: conf.InsecureSkipVerify,
	}
	// load all certs to dial
	certs := []tls.Certificate{}
	for _, certFile := range conf.Certs {
		cert, err := tls.LoadX509KeyPair(certFile.Cert, certFile.Key)
		if err != nil {
			log.Errorf("dial, tls load x509 cert err: %s, cert: %s, key: %s", err, certFile.Cert, certFile.Key)
			return nil, err
		}
		certs = append(certs, cert)
		tlsconfig.Certificates = certs
	}
	if conf.MTLS {
		// mtls, dial with our certs
		// load all ca certs to pool
		caPool := x509.NewCertPool()
		for _, caFile := range conf.CAs {
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
		tlsconfig.RootCAs = caPool
	}
	return tlsconfig, nil
}
