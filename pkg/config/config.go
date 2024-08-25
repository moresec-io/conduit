package config

// listen related
type CertKey struct {
	Cert string `yaml:"cert" json:"cert"`
	Key  string `yaml:"key" json:"key"`
}

type TLS struct {
	Enable             bool      `yaml:"enable,omitempty" json:"enable"`
	MTLS               bool      `yaml:"mtls,omitempty" json:"mtls"`
	CAs                []string  `yaml:"cas" json:"cas"`                                             // ca certs paths
	Certs              []CertKey `yaml:"certs" json:"certs"`                                         // certs paths
	InsecureSkipVerify bool      `yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify"` // for client use
}

type Listen struct {
	Network     string `yaml:"network" json:"network"`
	Addr        string `yaml:"addr" json:"addr"`
	TLSStrategy string `yaml:"tls_strategy"`                       // "local" or "manager"
	TLS         *TLS   `yaml:"tls,omitempty" json:"tls,omitempty"` // only when tls_strategy is local
}

type Dial struct {
	Network   string   `yaml:"network" json:"network"`
	Addresses []string `yaml:"addresses" json:"addresses"`
	TLS       *TLS     `yaml:"tls,omitempty" json:"tls"`
}
