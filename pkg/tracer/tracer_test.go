package tracer

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracer(t *testing.T) {
	require := require.New(t)
	prv, err := NewProvider()
	require.NoError(err)
	require.Nil(prv)

	_, err = NewProvider(
		WithEndpoint("http://aa"),
		WithSamplingRatio("4a32"),
	)
	require.ErrorIs(err, strconv.ErrSyntax)

	_, err = NewProvider(
		WithEndpoint("http://aa"),
		WithSamplingRatio(".5"),
	)
	require.NoError(err)
}
