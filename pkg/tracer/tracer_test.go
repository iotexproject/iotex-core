package tracer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracer(t *testing.T) {
	require := require.New(t)
	prv, err := NewProvider()
	require.NoError(err)
	require.Nil(prv)
}
