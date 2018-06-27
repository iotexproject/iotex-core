package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer"
)

var configFile = "../../../e2etest/config_local_delegate.yaml"

func Test_All(t *testing.T) {
	cfg, err := config.LoadConfigWithPath(configFile)
	require.Nil(t, err)
	httpPort := cfg.Explorer.Addr
	explorer.StartJSONServer(nil, nil, true, httpPort, 0)

	s := strings.Split(self(), " ")
	addr := s[len(s)-1]

	assert.Equal(t, "io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y", addr)
	assert.NotEqual(t, 0, height()) // height is random each time

	limit = 10
	tr := transfers([]string{addr, "10"})
	assert.Equal(t, 9, strings.Count(tr, "\n"))

	det := details([]string{addr})
	assert.Equal(t, 1, strings.Count(det, "\n"))
	assert.NotEqual(t, "", balance([]string{addr})) // no real way to test this because balance returned is random
}
