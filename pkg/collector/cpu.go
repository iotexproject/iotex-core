package collector

import (
	"fmt"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/prometheus/client_golang/prometheus"
)

type cpuCollector struct {
	metric *prometheus.Desc
}

func init() {
	registerCollector("cpu", NewCPUCollector)
}

// NewCPUCollector returns a new Collector exposing kernel/system statistics.
func NewCPUCollector() (Collector, error) {
	return &cpuCollector{
		metric: prometheus.NewDesc(namespace+"_cpu_stats", "cpu statistics", []string{"mode"}, nil),
	}, nil
}

func (c *cpuCollector) Update(ch chan<- prometheus.Metric) error {
	stats, err := cpu.Get()
	if err != nil {
		return fmt.Errorf("couldn't get cpu: %w", err)
	}
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(stats.User), "user")
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(stats.System), "system")
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(stats.Total), "total")
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(stats.Idle), "idle")

	return err
}
