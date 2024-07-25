package repo

const (
	TblCert = "tbl_cert"
	TblCA   = "tbl_ca"
)

type Cert struct {
	ID                      uint64   `gorm:"id"`
	SNI                     string   `gorm:"sni;index:idx_sni"` // server name indication
	CommonName              string   `gorm:"common_name"`
	SubjectAlternativeNames []string `gorm:"subject_alternative_names"`
	Days                    int      `gorm:"days"`
	Cert                    string   `gorm:"cert;type:text"`
	Key                     string   `gorm:"key;type:text"`
	Deleted                 bool     `gorm:"deleted"`
	CreateTime              int64    `gorm:"create_time"`
	UpdateTime              int64    `gorm:"update_time"`
}

func (Cert) TableName() string {
	return TblCert
}

type CA struct {
	ID           uint64 `gorm:"id"`
	Organization string `gorm:"organization"`
	CommonName   string `gorm:"common_name"`
	NotAfter     string `gorm:"not_after"`
	Expiration   int64  `gorm:"expiration"`
	Cert         string `gorm:"cert;type:text"`
	Key          string `gorm:"key;type:text"`
	Deleted      bool   `gorm:"deleted"`
	CreateTime   int64  `gorm:"create_time"`
	UpdateTime   int64  `gorm:"update_time"`
}

func (CA) TableName() string {
	return TblCA
}
