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
	Network string `yaml:"network" json:"network"`
	Addr    string `yaml:"addr" json:"addr"`
	TLS     *TLS   `yaml:"tls,omitempty" json:"tls,omitempty"`
}

type Dial struct {
	Network string   `yaml:"network" json:"network"`
	Addrs   []string `yaml:"addrs" json:"addrs"`
	TLS     *TLS     `yaml:"tls,omitempty" json:"tls"`
}
