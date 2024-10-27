package cms

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"github.com/singchia/go-timer/v2"
	"gorm.io/gorm"
)

type CMS interface {
	GetCert(san net.IP) (*Cert, error)
	ListCerts() ([]*Cert, error)
	DelCertBySAN(san net.IP) error
}

type Cert struct {
	CA   []byte
	Cert []byte
	Key  []byte
}

type cms struct {
	repo repo.Repo
	conf *config.Config
	tmr  timer.Timer

	// cache
	cacert []byte
	cakey  []byte
}

func NewCMS(conf *config.Config, repo repo.Repo) (CMS, error) {
	cms := &cms{
		repo: repo,
		conf: conf,
		tmr:  timer.NewTimer(),
	}
	err := cms.initCA()
	if err != nil {
		log.Errorf("newcms init err: %s", err)
		return nil, err
	}
	return cms, nil
}

func (cms *cms) initCA() error {
	caconf := cms.conf.Cert.CA
	now := time.Now()

	createCA := func() ([]byte, []byte, int64, error) {
		years, months, days := getDate(caconf.NotAfter)
		notBefore, notAfter := now, now.AddDate(years, months, days)
		cert, key, err := cms.genCA(notBefore, notAfter,
			caconf.Organization, caconf.CommonName, 2048)
		if err != nil {
			return nil, nil, 0, err
		}
		mca := &repo.CA{
			Organization: caconf.Organization,
			CommonName:   caconf.CommonName,
			NotAfter:     caconf.NotAfter,
			Expiration:   notAfter.Unix(),
			Cert:         cert,
			Key:          key,
			Deleted:      false,
			CreateTime:   now.Unix(),
			UpdateTime:   now.Unix(),
		}
		err = cms.repo.CreateCA(mca)
		if err != nil {
			return nil, nil, 0, err
		}
		return cert, key, int64(notAfter.Sub(notBefore).Seconds()), nil
	}
	var (
		expiration int64
		cert       []byte
		key        []byte
	)
	ca, err := cms.repo.GetCA()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			cert, key, expiration, err = createCA()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if ca.Expiration >= now.Unix() {
		err = cms.repo.DeleteCA(ca.ID)
		if err != nil {
			return err
		}
		cert, key, expiration, err = createCA()
		if err != nil {
			return err
		}
	} else {
		cert, key = ca.Cert, ca.Key
	}

	cms.cacert, cms.cakey = cert, key
	cms.tmr.Add(time.Duration(expiration)*time.Second, timer.WithHandler(func(e *timer.Event) {
		err = cms.initCA()
		if err != nil {
			log.Errorf("cms init ca err: %s", err)
		}
	}))
	return nil
}

func (cms *cms) initCert() error {
	certconf := cms.conf.Cert.Cert
	now := time.Now()

	createCert := func(san net.IP) (int64, error) {
		years, months, days := getDate(certconf.NotAfter)
		notBefore, notAfter := now, now.AddDate(years, months, days)
		cert, key, err := cms.genCert(cms.cacert, cms.cakey, notBefore, notAfter, certconf.Organization, certconf.CommonName, san, 2048)
		if err != nil {
			return 0, err
		}
		mcert := &repo.Cert{
			Organization:           certconf.Organization,
			CommonName:             certconf.CommonName,
			SubjectAlternativeName: san.String(),
			NotAfter:               certconf.NotAfter,
			Expiration:             notAfter.Unix(),
			Cert:                   cert,
			Key:                    key,
			Deleted:                false,
			CreateTime:             now.Unix(),
			UpdateTime:             now.Unix(),
		}
		err = cms.repo.CreateCert(mcert)
		if err != nil {
			return 0, err
		}
		return int64(notAfter.Sub(notBefore).Seconds()), nil
	}
	minexpiration := math.MaxInt64
	mcerts, err := cms.repo.ListCert(&repo.CertQuery{})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		} else {
			return err
		}
	}
	for _, mcert := range mcerts {
		if mcert.Expiration >= now.Unix() {
			err = cms.repo.DeleteCert(&repo.CertDelete{ID: mcert.ID})
			if err != nil {
				return err
			}
			expiration, err := createCert(net.ParseIP(mcert.SubjectAlternativeName))
			if err != nil {
				return err
			}
			if expiration < int64(minexpiration) {
				minexpiration = int(expiration)
			}
		}
	}
	cms.tmr.Add(time.Duration(minexpiration)*time.Second, timer.WithHandler(func(e *timer.Event) {
		err = cms.initCert()
		if err != nil {
			log.Errorf("cms init cert err: %s", err)
		}
	}))
	return nil
}

// generate cert if not exist
func (cms *cms) GetCert(san net.IP) (*Cert, error) {
	cert, err := cms.repo.GetCert(san.String())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			certconf := cms.conf.Cert.Cert
			now := time.Now()
			years, months, days := getDate(certconf.NotAfter)
			notBefore, notAfter := now, now.AddDate(years, months, days)
			cert, key, err := cms.genCert(cms.cacert, cms.cakey, notBefore, notAfter, certconf.Organization, certconf.CommonName, san, 2048)
			if err != nil {
				return nil, err
			}
			mcert := &repo.Cert{
				Organization:           certconf.Organization,
				CommonName:             certconf.CommonName,
				SubjectAlternativeName: san.String(),
				NotAfter:               certconf.NotAfter,
				Expiration:             notAfter.Unix(),
				Cert:                   cert,
				Key:                    key,
				Deleted:                false,
				CreateTime:             now.Unix(),
				UpdateTime:             now.Unix(),
			}
			err = cms.repo.CreateCert(mcert)
			if err != nil {
				return nil, err
			}
			return &Cert{
				CA:   cms.cacert,
				Cert: cert,
				Key:  key}, nil
		}
		return nil, err
	}
	return &Cert{
		CA:   cms.cacert,
		Cert: cert.Cert,
		Key:  cert.Key}, nil
}

func (cms *cms) ListCerts() ([]*Cert, error) {
	mcerts, err := cms.repo.ListCert(&repo.CertQuery{})
	if err != nil {
		return nil, err
	}
	certs := []*Cert{}
	for _, mcert := range mcerts {
		cert := &Cert{
			CA:   cms.cacert,
			Cert: mcert.Cert,
			Key:  mcert.Key,
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func (cms *cms) DelCertBySAN(san net.IP) error {
	return cms.repo.DeleteCert(&repo.CertDelete{
		SAN: san.String(),
	})
}

// notAfter: 1,2,3 means now add 1 year 2 months and 3 days
func (cms *cms) GenCA(notAfterStr string, organization, commonName string) ([]byte, []byte, error) {
	years, months, days := getDate(notAfterStr)
	notBefore := time.Now()
	notAfter := notBefore.AddDate(years, months, days)
	return cms.genCA(notBefore, notAfter, organization, commonName, 2048)
}

func (cms *cms) genCA(notBefore, notAfter time.Time,
	organization, commonName string, bits int) ([]byte, []byte, error) {

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	catemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{organization},
			CommonName:   commonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
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

func (cms *cms) GenCert(notAfterStr string, organization, commonName string, san net.IP) ([]byte, []byte, error) {
	years, months, days := getDate(notAfterStr)
	notBefore := time.Now()
	notAfter := notBefore.AddDate(years, months, days)
	ca, err := cms.repo.GetCA()
	if err != nil {
		return nil, nil, err
	}
	return cms.genCert(ca.Cert, ca.Key, notBefore, notAfter, organization, commonName, san, 2048)
}

func (cms *cms) genCert(cacert, cakey []byte,
	notBefore, notAfter time.Time,
	organization, commonName string, san net.IP, bits int) ([]byte, []byte, error) {
	ca, err := x509.ParseCertificate(cacert)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS1PrivateKey(cakey)
	if err != nil {
		return nil, nil, err
	}

	signkey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	certtemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{organization},
			CommonName:   commonName,
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		IPAddresses: []net.IP{san},
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
