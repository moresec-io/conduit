package repo

import (
	"sync"

	"github.com/moresec-io/conduit/pkg/network"
)

type Policy struct {
	PeerDialConfig *network.DialConfig // dial using our tls
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

func (cache *cache) DelIPPortPolicy(ipport string) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	delete(cache.ipPolicies, ipport)
}

func (cache *cache) AddPortPolicy(port int, policy *Policy) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	cache.portPolicies[port] = policy
}

func (cache *cache) DelPortPolicy(port int) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	delete(cache.portPolicies, port)
}

func (cache *cache) AddIPPolicy(ip string, policy *Policy) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	cache.ipPolicies[ip] = policy
}

func (cache *cache) DelIPPolicy(ip string) {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	delete(cache.ipPolicies, ip)
}

func (cache *cache) GetPolicyByIP(ip string) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy := cache.ipPolicies[ip]
	return policy
}

func (cache *cache) GetPolicyByIPPort(ipport string) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy := cache.ipportPolicies[ipport]
	return policy
}

func (cache *cache) GetPolicyByPort(port int) *Policy {
	cache.mtx.RLock()
	defer cache.mtx.RUnlock()

	policy := cache.portPolicies[port]
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
	policy = cache.ipPolicies[ip]
	return policy
}
