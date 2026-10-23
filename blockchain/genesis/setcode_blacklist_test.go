// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
)

// mainnetGenesis returns the genesis a node loads with no genesis file, which
// is MainNet's.
func mainnetGenesis(t *testing.T) Genesis {
	g, err := New("")
	require.NoError(t, err)
	require.True(t, g.IsMainnet())
	return g
}

func TestMainnetSetCodeAuthorityBlackList(t *testing.T) {
	r := require.New(t)
	digest := func(addrs []string) string {
		h := sha256.Sum256([]byte(strings.Join(addrs, "\n")))
		return hex.EncodeToString(h[:])
	}
	// These are the values MainNet has executed with since the list was
	// introduced; changing any of them is a hard fork.
	bl := MainnetSetCodeAuthorityBlackList()
	rm := MainnetSetCodeAuthorityBlackListRemoval()
	r.Len(bl, 29)
	r.Equal("a626100d81dc4e5f1a7c1600877df59c5066889c4fadd4ef2048522af6feaa3d", digest(bl))
	r.Equal(uint64(45404174), MainnetSetCodeAuthorityBlackListActiveHeight)
	r.Len(rm, 20)
	r.Equal("7d5dd77df3d1b1bd9643c08e1811d5d7a64b15a8feeb473968a78f9f51f1648d", digest(rm))
	for _, addr := range bl {
		_, err := address.FromString(addr)
		r.NoError(err, addr)
	}
	for _, addr := range rm {
		r.Contains(bl, addr)
	}
	// each call hands out its own copy
	bl[0] = "changed"
	r.NotEqual("changed", MainnetSetCodeAuthorityBlackList()[0])
}

func TestSetCodeAuthorityBlackListParams(t *testing.T) {
	t.Run("mainnet is fixed in code", func(t *testing.T) {
		r := require.New(t)
		g := mainnetGenesis(t)
		// a literal is not validated, but the fields are still not consulted
		g.SetCodeAuthorityBlackList = []string{"io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu"}
		g.SetCodeAuthorityBlackListActiveHeight = 1
		r.True(g.IsMainnet())
		bl, active, rm := g.SetCodeAuthorityBlackListParams()
		r.Equal(MainnetSetCodeAuthorityBlackList(), bl)
		r.Equal(MainnetSetCodeAuthorityBlackListActiveHeight, active)
		r.Equal(MainnetSetCodeAuthorityBlackListRemoval(), rm)
	})
	t.Run("mainnet rejects an override", func(t *testing.T) {
		r := require.New(t)
		g := mainnetGenesis(t)
		r.NoError(g.validate())
		g.SetCodeAuthorityBlackList = []string{}
		r.NoError(g.validate())
		g.SetCodeAuthorityBlackList = []string{"io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu"}
		r.ErrorContains(g.validate(), "must not be set on mainnet")
		g = mainnetGenesis(t)
		g.SetCodeAuthorityBlackListActiveHeight = 1
		r.ErrorContains(g.validate(), "must not be set on mainnet")
		g = mainnetGenesis(t)
		g.SetCodeAuthorityBlackListRemoval = []string{"io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu"}
		r.ErrorContains(g.validate(), "must not be set on mainnet")
	})
	t.Run("other chains default to empty", func(t *testing.T) {
		r := require.New(t)
		for _, g := range []Genesis{Default, TestDefault()} {
			r.False(g.IsMainnet())
			bl, active, rm := g.SetCodeAuthorityBlackListParams()
			r.Empty(bl)
			r.Zero(active)
			r.Empty(rm)
		}
	})
	t.Run("other chains read genesis yaml", func(t *testing.T) {
		r := require.New(t)
		path := filepath.Join(t.TempDir(), "genesis.yaml")
		r.NoError(os.WriteFile(path, []byte(`blockchain:
  timestamp: 1571036400
  setCodeAuthorityBlackList:
  - io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu
  - io1va6umgyewzjatq8nrznyct9f2yp49rkpxtx3jj
  setCodeAuthorityBlackListActiveHeight: 7
  setCodeAuthorityBlackListRemoval:
  - io1va6umgyewzjatq8nrznyct9f2yp49rkpxtx3jj
`), 0o600))
		g, err := New(path)
		r.NoError(err)
		r.False(g.IsMainnet())
		bl, active, rm := g.SetCodeAuthorityBlackListParams()
		r.Equal([]string{"io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu", "io1va6umgyewzjatq8nrznyct9f2yp49rkpxtx3jj"}, bl)
		r.Equal(uint64(7), active)
		r.Equal([]string{"io1va6umgyewzjatq8nrznyct9f2yp49rkpxtx3jj"}, rm)
	})
}
