package api

import "github.com/prometheus/client_golang/prometheus"

var (
	apiLimitMtcs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iotex_api_limit_metrics",
		Help: "api limit metrics.",
	}, []string{"limit"})
)

func init() {
	prometheus.MustRegister(apiLimitMtcs)
}
