package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestLoadavg(t *testing.T) {
	require := require.New(t)
	c, err := NewLoadavgCollector()
	require.NoError(err)
	require.NotNil(c)
	ch := make(chan prometheus.Metric)
	go func() {
		c.Update(ch)
		close(ch)
	}()
	n := 0
	for m := range ch {
		n++
		pb := &dto.Metric{}
		err := m.Write(pb)
		require.NoError(err)
	}
	require.Equal(3, n)
}
