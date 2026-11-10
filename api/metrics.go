package api

import "github.com/prometheus/client_golang/prometheus"

var (
	apiLimitMtcs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iotex_api_limit_metrics",
		Help: "api limit metrics.",
	}, []string{"limit"})

	// apiStreamDropMtc counts subscriptions dropped for not keeping up. This is
	// the meaningful signal for the backpressure path: a monotonic per-stream
	// counter that does not race between concurrent subscriptions. (A live
	// per-subscription queue-depth gauge would need a per-subscription label to
	// be meaningful; a single shared gauge is overwritten by every sender and
	// measures nothing, so it is intentionally omitted.)
	apiStreamDropMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iotex_api_stream_dropped_total",
		Help: "streaming API subscriptions dropped because the consumer was too slow.",
	}, []string{"stream"})
)

func init() {
	prometheus.MustRegister(apiLimitMtcs)
	prometheus.MustRegister(apiStreamDropMtc)
}
