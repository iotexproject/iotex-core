// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setEVMChainIDFlag binds the shared _evmChainIDFlag to a throwaway command and
// sets it to v for the duration of a test (registering resets the bound value to
// its default of 0, which the cleanup relies on to restore the global state).
func setEVMChainIDFlag(t *testing.T, v uint64) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	_evmChainIDFlag.RegisterCommand(cmd) // resets bound value to default (0)
	require.NoError(t, cmd.Flags().Set("evm-chain-id", strconv.FormatUint(v, 10)))
	t.Cleanup(func() {
		_evmChainIDFlag.RegisterCommand(&cobra.Command{Use: "cleanup"}) // reset to 0
	})
}

func TestResolveEVMChainID(t *testing.T) {
	r := require.New(t)

	t.Run("flag override wins over node and fallback", func(t *testing.T) {
		setEVMChainIDFlag(t, 12345)
		fromNode := func() (uint64, error) { return 999, nil }
		got, err := resolveEVMChainID(1, fromNode)
		r.NoError(err)
		r.Equal(uint64(12345), got)
	})

	t.Run("node value used when flag unset", func(t *testing.T) {
		setEVMChainIDFlag(t, 0)
		fromNode := func() (uint64, error) { return 4689, nil }
		got, err := resolveEVMChainID(2, fromNode) // iotex id 2 maps to 4690, but node wins
		r.NoError(err)
		r.Equal(uint64(4689), got)
	})

	t.Run("fallback mapping when node fetch errors", func(t *testing.T) {
		setEVMChainIDFlag(t, 0)
		fromNode := func() (uint64, error) { return 0, fmt.Errorf("unreachable") }
		got, err := resolveEVMChainID(2, fromNode)
		r.NoError(err)
		r.Equal(uint64(4690), got)
	})

	t.Run("fallback mapping when node returns zero", func(t *testing.T) {
		setEVMChainIDFlag(t, 0)
		fromNode := func() (uint64, error) { return 0, nil }
		got, err := resolveEVMChainID(3, fromNode)
		r.NoError(err)
		r.Equal(uint64(4691), got)
	})

	t.Run("fallback mapping when node fetcher is nil", func(t *testing.T) {
		setEVMChainIDFlag(t, 0)
		got, err := resolveEVMChainID(1, nil)
		r.NoError(err)
		r.Equal(uint64(4689), got)
	})

	t.Run("error on unknown chain id without flag or node", func(t *testing.T) {
		setEVMChainIDFlag(t, 0)
		fromNode := func() (uint64, error) { return 0, fmt.Errorf("unreachable") }
		_, err := resolveEVMChainID(99, fromNode)
		r.Error(err)
	})
}

func TestDeriveWeb3Endpoint(t *testing.T) {
	r := require.New(t)
	cases := []struct {
		name     string
		endpoint string
		secure   bool
		want     string
	}{
		{"mainnet public", "api.iotex.one:443", true, "https://babel-api.mainnet.iotex.io"},
		{"testnet public", "api.testnet.iotex.one:443", true, "https://babel-api.testnet.iotex.io"},
		{"self-hosted insecure", "127.0.0.1:14014", false, "http://127.0.0.1:15014"},
		{"self-hosted secure", "node.example.com:14014", true, "https://node.example.com:15014"},
		{"host without port", "localhost", false, "http://localhost:15014"},
		{"ipv6 host", "[::1]:14014", false, "http://[::1]:15014"},
		{"empty", "", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r.Equal(c.want, deriveWeb3Endpoint(c.endpoint, c.secure))
		})
	}
}

func TestFetchEthChainID(t *testing.T) {
	r := require.New(t)

	t.Run("parses hex result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1251"}`))
		}))
		defer srv.Close()
		got, err := fetchEthChainID(srv.URL)
		r.NoError(err)
		r.Equal(uint64(4689), got) // 0x1251 == 4689
	})

	t.Run("propagates rpc error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`))
		}))
		defer srv.Close()
		_, err := fetchEthChainID(srv.URL)
		r.Error(err)
	})

	t.Run("errors on non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()
		_, err := fetchEthChainID(srv.URL)
		r.Error(err)
	})
}
