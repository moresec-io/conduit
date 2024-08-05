package network

import (
	"testing"

	"github.com/singchia/go-hammer/log"
	"github.com/stretchr/testify/assert"
)

func TestListNetworks(t *testing.T) {
	ipNets, err := ListNetworks()
	assert.Equal(t, nil, err)
	for _, ipNet := range ipNets {
		log.Info(ipNet.String())
	}
}
