package httputil

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.Run("creates a HTTP server with time out settings", func(t *testing.T) {
		var handler http.Handler
		addr := "myAddress"

		expectValue := http.Server{
			ReadTimeout:  35 * time.Second,
			WriteTimeout: 35 * time.Second,
			IdleTimeout:  120 * time.Second,
			Addr:         addr,
			Handler:      handler,
		}

		result := Server(addr, handler)

		require.Equal(t, expectValue, result)
	})
}

func TestLimitListener(t *testing.T) {
	t.Run("missing port in address", func(t *testing.T) {
		expectedErr := errors.New("listen tcp: address myAddress: missing port in address")
		result, err := LimitListener("myAddress")
		require.Error(t, err)
		require.Equal(t, expectedErr.Error(), err.Error())
		require.Equal(t, nil, result)
	})

	t.Run("input empty string", func(t *testing.T) {
		_, err := LimitListener("")
		require.NoError(t, err)
	})
}
