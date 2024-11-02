package network

import (
	"testing"

	"github.com/singchia/go-hammer/log"
	"github.com/stretchr/testify/assert"
)

func TestListNetworks(t *testing.T) {
	ips, err := ListIPs()
	assert.Equal(t, nil, err)
	for _, ip := range ips {
		log.Info(ip.String())
	}
}
