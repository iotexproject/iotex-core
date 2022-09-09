package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestCollector(t *testing.T) {
	require := require.New(t)
	c, err := NewNodeCollector("test")
	require.ErrorContains(err, "missing collector")
	c, err = NewNodeCollector("cpu")
	require.NoError(err)
	ch := make(chan prometheus.Metric)
	go func() {
		c.Collect(ch)
		close(ch)
	}()

	ch2 := make(chan *prometheus.Desc)
	go func() {
		c.Describe(ch2)
		close(ch2)
	}()
	n := 0
	for m := range ch {
		n++
		pb := &dto.Metric{}
		err := m.Write(pb)
		require.NoError(err)
	}
	require.Equal(4, n)
}
