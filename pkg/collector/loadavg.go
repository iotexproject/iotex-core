package collector

import (
	"fmt"

	"github.com/mackerelio/go-osstat/loadavg"
	"github.com/prometheus/client_golang/prometheus"
)

type loadavgCollector struct {
	metric []typedDesc
}

func init() {
	registerCollector("loadavg", NewLoadavgCollector)
}

// NewLoadavgCollector returns a new Collector exposing load average stats.
func NewLoadavgCollector() (Collector, error) {
	return &loadavgCollector{
		metric: []typedDesc{
			{prometheus.NewDesc(namespace+"_load1", "1m load average.", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(namespace+"_load5", "5m load average.", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(namespace+"_load15", "15m load average.", nil, nil), prometheus.GaugeValue},
		},
	}, nil
}

func (c *loadavgCollector) Update(ch chan<- prometheus.Metric) error {
	loads, err := loadavg.Get()
	if err != nil {
		return fmt.Errorf("couldn't get load: %w", err)
	}
	ch <- c.metric[0].mustNewConstMetric(loads.Loadavg1)
	ch <- c.metric[1].mustNewConstMetric(loads.Loadavg5)
	ch <- c.metric[2].mustNewConstMetric(loads.Loadavg15)
	return err
}
