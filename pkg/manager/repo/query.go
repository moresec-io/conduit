package repo

type CertDelete struct {
	ID  uint64
	SAN string
}

type CertQuery struct {
	SAN string
}
