package repo

import (
	"sync"

	"github.com/moresec-io/conduit/pkg/network"
)

type Policy struct {
	PeerDialConfig *network.DialConfig
	DstTo          string
}

type cache struct {
	ipportPolicies map[string]*Policy
	portPolicies   map[int]*Policy
	ipPolicies     map[string]*Policy

	mtx sync.RWMutex
}

func (cache *cache) AddIPPortPolicy(ipport string, policy *Policy) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	cache.ipportPolicies[ipport] = policy
}

func (cache *cache) AddPortPolicy(port int, policy *Policy) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	cache.portPolicies[port] = policy
}

func (cache *cache) AddIPPolicy(ip string, policy *Policy) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	cache.ipPolicies[ip] = policy
}

func (cache *cache) GetPolicyByIP(ip string) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy, _ := cache.ipPolicies[ip]
	return policy
}

func (cache *cache) GetPolicyByIPPort(ipport string) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy, _ := cache.ipportPolicies[ipport]
	return policy
}

func (cache *cache) GetPolicyByPort(port int) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy, _ := cache.portPolicies[port]
	return policy
}

// first ipport, then port, last dstIP
func (cache *cache) GetPolicy(ipport string, port int, ip string) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy, ok := cache.ipportPolicies[ipport]
	if ok {
		return policy
	}
	policy, ok = cache.portPolicies[port]
	if ok {
		return policy
	}
	policy, ok = cache.ipPolicies[ip]
	return policy
}
