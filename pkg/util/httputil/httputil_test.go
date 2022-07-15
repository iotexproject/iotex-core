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
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       DefaultServerConfig.ReadTimeout,
			WriteTimeout:      DefaultServerConfig.WriteTimeout,
			IdleTimeout:       DefaultServerConfig.IdleTimeout,
			Addr:              addr,
			Handler:           handler,
		}
		result := NewServer(addr, handler, HeaderTimeout(2*time.Second))
		require.Equal(t, expectValue, result)
	})
}

func TestLimitListener(t *testing.T) {
	t.Run("missing port in address", func(t *testing.T) {
		expectedErr := errors.New("listen tcp: address myAddress: missing port in address")
		result, err := LimitListener("myAddress")
		require.Error(t, err)
		require.Equal(t, expectedErr.Error(), err.Error())
		require.Nil(t, result)
	})

	t.Run("input empty string", func(t *testing.T) {
		_, err := LimitListener("")
		// fix none root permission error
		if err != nil {
			require.Equal(t, "listen tcp :80: bind: permission denied", err.Error())
		} else {
			require.NoError(t, err)
		}
	})
}
