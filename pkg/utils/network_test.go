package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListNetwork(t *testing.T) {
	err := ListNetworks()
	assert.Equal(t, nil, err)
}
