package cms

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
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

type CMS struct {
	repo repo.Repo
	conf *config.Config
	tmr  timer.Timer
}

func NewCMS(conf *config.Config, repo repo.Repo) (*CMS, error) {
	cms := &CMS{
		repo: repo,
		conf: conf,
		tmr:  timer.NewTimer(),
	}
	err := cms.init()
	if err != nil {
		log.Errorf("newcms init err: %s", err)
		return nil, err
	}
	return cms, nil
}

func (cms *CMS) init() error {
	caconf := cms.conf.Cert.CA
	now := time.Now()

	initCA := func() (int64, error) {
		years, months, days := getDate(caconf.NotAfter)
		notBefore, notAfter := now, now.AddDate(years, months, days)
		cert, key, err := cms.genCA(notBefore, notAfter,
			caconf.Organization, caconf.CommonName, 2048)
		if err != nil {
			return 0, err
		}
		ca := &repo.CA{
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
		err = cms.repo.CreateCA(ca)
		if err != nil {
			return 0, err
		}
		return int64(notAfter.Sub(notBefore).Seconds()), nil
	}
	var expiration int64
	ca, err := cms.repo.GetCA()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			expiration, err = initCA()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if ca.Expiration <= now.Unix() {
		err = cms.repo.DeleteCA(ca.ID)
		if err != nil {
			return err
		}
		expiration, err = initCA()
		if err != nil {
			return err
		}
	}

	cms.tmr.Add(time.Duration(expiration)*time.Second, timer.WithHandler(func(e *timer.Event) {
		err = cms.init()
		if err != nil {
			log.Errorf("cms init err: %s", err)
		}
	}))
	return nil
}

// notAfter: 1,2,3 means now add 1 year 2 months and 3 days
func (cms *CMS) GenCA(notAfterStr string, organization, commonName string) ([]byte, []byte, error) {
	years, months, days := getDate(notAfterStr)
	notBefore := time.Now()
	notAfter := notBefore.AddDate(years, months, days)
	return cms.genCA(notBefore, notAfter, organization, commonName, 2048)
}

func (cms *CMS) genCA(notBefore, notAfter time.Time,
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

func (cms *CMS) GenCert(notAfterStr string, organization, commonName string, san net.IP) ([]byte, []byte, error) {
	years, months, days := getDate(notAfterStr)
	notBefore := time.Now()
	notAfter := notBefore.AddDate(years, months, days)
	ca, err := cms.repo.GetCA()
	if err != nil {
		return nil, nil, err
	}
	return cms.genCert(ca.Cert, ca.Key, notBefore, notAfter, organization, commonName, san, 2048)
}

func (cms *CMS) genCert(cacert, cakey []byte,
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
