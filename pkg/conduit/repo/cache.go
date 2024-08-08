package repo

import "github.com/moresec-io/conduit/pkg/network"

type Policy struct {
	PeerDialConfig *network.DialConfig
	DstTo          string
}

type cache struct {
	ipportPolicies map[string]*Policy
	portPolicies   map[string]*Policy
	ipPolicies     map[string]*Policy
}

func (cache *cache) AddIPPortPolicy() {

}
