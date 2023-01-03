// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import "github.com/prometheus/client_golang/prometheus"

var nodeDelegateHeightGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "iotex_node_delegate_height_gauge",
		Help: "delegate height",
	},
	[]string{"address", "version"},
)

func init() {
	prometheus.MustRegister(nodeDelegateHeightGauge)
}
