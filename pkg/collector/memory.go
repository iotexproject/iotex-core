package collector

import (
	"fmt"

	"github.com/mackerelio/go-osstat/memory"
	"github.com/prometheus/client_golang/prometheus"
)

type meminfoCollector struct {
	metric *prometheus.Desc
}

func init() {
	registerCollector("memory", NewMeminfoCollector)
}

// NewMeminfoCollector returns a new Collector exposing memory stats.
func NewMeminfoCollector() (Collector, error) {
	return &meminfoCollector{
		metric: prometheus.NewDesc(namespace+"_memory_stats", "memory stats", []string{"mode"}, nil),
	}, nil
}

// Update to get the platform specific memory stats.
func (c *meminfoCollector) Update(ch chan<- prometheus.Metric) error {
	memInfo, err := memory.Get()
	if err != nil {
		return fmt.Errorf("couldn't get meminfo: %w", err)
	}
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(memInfo.Total), "total")
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(memInfo.Free), "free")
	ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, float64(memInfo.Used), "used")
	return nil
}
