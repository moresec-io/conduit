package repo

import "github.com/moresec-io/conduit/pkg/manager/config"

type Repo interface {
	CreateCA(ca *CA) error
	GetCA() (*CA, error)
	DeleteCA(id uint64) error

	CreateCert(cert *Cert) error
	DeleteCert(delete *CertDelete) error
	GetCert(san string) (*Cert, error)
	ListCert(query *CertQuery) ([]*Cert, error)
}

func NewRepo(conf *config.Config) (Repo, error) {
	dao, err := NewDao(conf)
	return dao, err
}
