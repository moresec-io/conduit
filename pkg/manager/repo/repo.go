package repo

import "github.com/moresec-io/conduit/pkg/manager/config"

type Repo interface {
	CreateCA(ca *CA) error
	GetCA() (*CA, error)

	CreateCert(cert *Cert) error
	DeleteCert(delete *CertDelete) error
	GetCert(sni string) (*Cert, error)
	ListCert(query *CertQuery) ([]*Cert, error)
}

func NewRepo(conf *config.DB) (Repo, error) {
	dao, err := NewDao(conf)
	return dao, err
}
