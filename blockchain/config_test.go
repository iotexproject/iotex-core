// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"strings"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/stretchr/testify/require"
)

func TestProducer(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	r.NotEmpty(cfg.ProducerAddress())
	r.NotEmpty(cfg.ProducerPrivateKeys())
}

func TestWhitelist(t *testing.T) {
	r := require.New(t)
	cfg := Config{}
	sk, err := crypto.HexStringToPrivateKey("308193020100301306072a8648ce3d020106082a811ccf5501822d0479307702010104202d57ec7da578b98dad465997748ed02af0c69092ad809598073e5a2356c20492a00a06082a811ccf5501822da14403420004223356f0c6f40822ade24d47b0cd10e9285402cbc8a5028a8eec9efba44b8dfe1a7e8bc44953e557b32ec17039fb8018a58d48c8ffa54933fac8030c9a169bf6")
	r.NoError(err)
	r.False(cfg.whitelistSignatureScheme(sk))
	cfg.ProducerPrivKey = sk.HexString()
	r.Panics(func() { cfg.ProducerPrivateKeys() })

	cfg.SignatureScheme = append(cfg.SignatureScheme, SigP256sm2)
	r.Equal(sk, cfg.ProducerPrivateKeys()[0])
	r.Equal(sk.PublicKey().Address().String(), cfg.ProducerAddress()[0].String())
}

func TestProducerPrivateKeys_RangeParsing(t *testing.T) {
	r := require.New(t)
	cfg := DefaultConfig
	genKeys := func(size int) string {
		var privKeyStrs []string
		for i := 0; i < size; i++ {
			sk, err := crypto.GenerateKey()
			r.NoError(err)
			privKeyStrs = append(privKeyStrs, sk.HexString())
		}
		return strings.Join(privKeyStrs, ",")
	}

	getKeys := func(privKey, privKeyRange string) (keys []crypto.PrivateKey, panicked bool) {
		cfg.ProducerPrivKey = privKey
		cfg.ProducerPrivKeyRange = privKeyRange
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		keys = cfg.ProducerPrivateKeys()
		return
	}

	privKeys := genKeys(5)
	keys, panicked := getKeys(privKeys, "")
	r.False(panicked)
	r.Len(keys, 5)

	keys, panicked = getKeys(privKeys, "[:]")
	r.False(panicked)
	r.Len(keys, 5)

	keys, panicked = getKeys(privKeys, "[0:0]")
	r.False(panicked)
	r.Len(keys, 0)

	keys, panicked = getKeys(privKeys, "[0:1]")
	r.False(panicked)
	r.Len(keys, 1)

	keys, panicked = getKeys(privKeys, "[1:3]")
	r.False(panicked)
	r.Len(keys, 2)

	keys, panicked = getKeys(privKeys, "[3:5]")
	r.False(panicked)
	r.Len(keys, 2)

	keys, panicked = getKeys(privKeys, "[5:]")
	r.False(panicked)
	r.Len(keys, 0)

	keys, panicked = getKeys(privKeys, "[:5]")
	r.False(panicked)
	r.Len(keys, 5)

	keys, panicked = getKeys(privKeys, "[2:]")
	r.False(panicked)
	r.Len(keys, 3)

	_, panicked = getKeys(privKeys, "[invalid]")
	r.True(panicked)

	_, panicked = getKeys(privKeys, "[-1:1]")
	r.True(panicked)

	_, panicked = getKeys(privKeys, "[2:1]")
	r.True(panicked)
	_, panicked = getKeys(privKeys, "[1:6]")
	r.True(panicked)
	_, panicked = getKeys(privKeys, "[1:5:7]")
	r.True(panicked)
}
