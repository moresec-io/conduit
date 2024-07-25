package storage

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	driver "github.com/go-sql-driver/mysql"
	"github.com/jumboframes/armorigo/log"
	gconfig "github.com/moresec-io/conduit/pkg/config"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewMySQL(conf *config.DB) (*gorm.DB, error) {
	dsn, err := getMySQLUri(conf)
	if err != nil {
		return nil, err
	}
	// 连接db
	db, err := gorm.Open(
		mysql.New(mysql.Config{
			DriverName: "mysql",
			DSN:        dsn,
		}),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			IgnoreRelationshipsWhenMigrating:         true,
		})
	if err != nil {
		return nil, err
	}
	if conf.Debug {
		db = db.Debug()
	}
	return db, nil
}

func getMySQLUri(conf *config.DB) (string, error) {
	uri := fmt.Sprintf("%s:%s@tcp(%s)/%s", conf.Username, conf.Password, conf.Address, conf.DB)
	// opts
	if conf.Options == "" {
		conf.Options = "parseTime=true"
	}
	opts := []string{conf.Options}

	// tls
	if conf.TLS != nil && conf.TLS.Enable {
		tlsconfig, err := getTLSConfig(conf.TLS)
		if err != nil {
			return "", err
		}
		if err = driver.RegisterTLSConfig("custom", tlsconfig); err != nil {
			return "", fmt.Errorf("sql: failed to register tls config: %v", err)
		}
		opts = append(opts, "tls=custom")
	}
	return uri + "?" + strings.Join(opts, "&"), nil
}

func getTLSConfig(conf *gconfig.TLS) (*tls.Config, error) {
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
