package repo

import (
	"time"

	"gorm.io/gorm"
)

// CA
func (dao *dao) CreateCA(ca *CA) error {
	tx := dao.db.Model(&CA{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	return tx.Create(ca).Error
}

func (dao *dao) GetCA() (*CA, error) {
	tx := dao.db.Model(&CA{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	ca := &CA{}
	tx.Where("deleted", false).Limit(1).Find(ca)
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return ca, tx.Error
}

func (dao *dao) DeleteCA(id uint64) error {
	tx := dao.db.Model(&CA{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	tx = tx.Where("id", id)
	now := time.Now().Unix()
	return tx.Updates(map[string]interface{}{"update_time": now, "deleted": true}).Error
}

// Cert
func (dao *dao) CreateCert(cert *Cert) error {
	tx := dao.db.Model(&Cert{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	return tx.Create(cert).Error
}

func (dao *dao) DeleteCert(delete *CertDelete) error {
	tx := dao.db.Model(&Cert{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	tx = buildCertDelete(tx, delete)
	now := time.Now().Unix()
	return tx.Updates(map[string]interface{}{"update_time": now, "deleted": true}).Error
}

func (dao *dao) GetCert(sni string) (*Cert, error) {
	tx := dao.db.Model(&Cert{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}
	tx = tx.Where("sni = ?", sni).Where("deleted = ?", false).Limit(1)

	cert := &Cert{}
	tx = tx.Find(cert)
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return cert, tx.Error
}

func (dao *dao) ListCert(query *CertQuery) ([]*Cert, error) {
	tx := dao.db.Model(&Cert{})
	if dao.conf.Debug {
		tx = tx.Debug()
	}

	certs := []*Cert{}
	tx = tx.Find(&certs)
	return certs, nil
}

func buildCertQuery(tx *gorm.DB, query *CertQuery) *gorm.DB {
	tx = tx.Where("deleted", false)
	if query.SNI != "" {
		tx = tx.Where("sni = ?", query.SNI)
	}
	return tx
}

func buildCertDelete(tx *gorm.DB, delete *CertDelete) *gorm.DB {
	if delete.SNI != "" {
		tx = tx.Where("sni = ?", delete.SNI).Where("deleted", false)
	}
	return tx
}
