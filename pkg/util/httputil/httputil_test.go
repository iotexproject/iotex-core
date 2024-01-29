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

		result := NewServer(addr, handler, ReadHeaderTimeout(2*time.Second))
		require.Equal(t, 2*time.Second, result.ReadHeaderTimeout)
		require.Equal(t, DefaultServerConfig.ReadTimeout, result.ReadTimeout)
		require.Equal(t, DefaultServerConfig.WriteTimeout, result.WriteTimeout)
		require.Equal(t, DefaultServerConfig.IdleTimeout, result.IdleTimeout)
		require.Equal(t, addr, result.Addr)
		require.Equal(t, handler, result.Handler)
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
