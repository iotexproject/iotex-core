package network

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestLoadTestTopology(t *testing.T) {
	topology1 := NewTestTopology()
	topologyStr, err := yaml.Marshal(topology1)
	assert.Nil(t, err)
	path := "/tmp/topology_" + strconv.Itoa(rand.Int()) + ".yaml"
	err = ioutil.WriteFile(path, topologyStr, 0666)
	assert.NoError(t, err)

	defer func() {
		if os.Remove(path) != nil {
			assert.Fail(t, "Error when deleting the test file")
		}
	}()

	topology2, err := NewTopology(path)
	assert.Nil(t, err)
	assert.NotNil(t, topology2)
	assert.Equal(t, topology1, topology2)
}

func NewTestTopology() *Topology {
	return &Topology{
		NeighborList: map[string][]string{
			"127.0.0.1:10001": {"127.0.0.1:10002", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10002": {"127.0.0.1:10001", "127.0.0.1:10003", "127.0.0.1:10004"},
			"127.0.0.1:10003": {"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10004"},
			"127.0.0.1:10004": {"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10003"},
		},
	}
}
