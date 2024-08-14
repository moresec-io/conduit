package repo

import (
	"net"

	"github.com/jumboframes/armorigo/log"
)

type Repo interface {
	AddIPPortPolicy(ipport string, policy *Policy)
	AddPortPolicy(port int, policy *Policy)
	AddIPPolicy(ip string, policy *Policy)
	GetPolicyByIP(ip string) *Policy
	GetPolicyByIPPort(ipport string) *Policy
	GetPolicyByPort(port int) *Policy
	GetPolicy(ipport string, port int, ip string) *Policy

	InitIPSet() error
	AddIPSetIPPort(ip net.IP, port uint16) error
	AddIPSetPort(port uint16) error
	AddIPSetIP(ip net.IP) error
	DelIPSetIPPort(ip net.IP, port uint16) error
	DelIPSetPort(port uint16) error
	DelIPSetIP(ip net.IP) error
	FiniIPSet(level log.Level, prefix string) error
}

type repo struct {
	*cache
	*ipset
}

func NewRepo() Repo {
	return &repo{
		cache: &cache{
			ipportPolicies: make(map[string]*Policy),
			portPolicies:   make(map[int]*Policy),
			ipPolicies:     make(map[string]*Policy),
		},
		ipset: &ipset{},
	}
}
