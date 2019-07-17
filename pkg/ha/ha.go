// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package ha

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Controller controls the node high availability status
type Controller struct {
	c consensus.Consensus
}

// New constructs a HA controller instance
func New(c consensus.Consensus) *Controller {
	return &Controller{
		c: c,
	}
}

// Handle handles admin request
func (ha *Controller) Handle(w http.ResponseWriter, r *http.Request) {
	val := strings.ToLower(r.URL.Query().Get("activate"))
	switch val {
	case "true":
		log.S().Info("Set the node to active mode")
		ha.c.Activate(true)
	case "false":
		log.S().Info("Set the node to stand-by mode")
		ha.c.Activate(false)
	case "":
		type payload struct {
			Active bool `json:"active"`
		}
		enc := json.NewEncoder(w)
		if err := enc.Encode(&payload{Active: ha.c.Active()}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
