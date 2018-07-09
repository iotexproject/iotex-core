// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/logger"
)

// Topology is the neighbor list for each node. This is used for generating the P2P network in a given topology. Note
// that the list contains the outgoing connections.
type Topology struct {
	NeighborList map[string][]string `yaml:"neighborList"`
}

// NewTopology loads the topology struct from the given yaml file
func NewTopology(path string) (*Topology, error) {
	topologyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Error().Err(err).Msg("Error when reading the topology file")
		return nil, err
	}

	topology := Topology{}
	err = yaml.Unmarshal(topologyBytes, &topology)
	if err != nil {
		logger.Error().Err(err).Msg("Error when decoding the topology file")
		return nil, err
	}

	return &topology, nil
}
