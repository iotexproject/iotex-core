// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

const (
	Pubkey  = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	Prikey  = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
	RawAddr = "io1qyqsyqcy8uhx9jtdc2xp5wx7nxyq3xf4c3jmxknzkuej8y"
)

func TestPlainKeyStore(t *testing.T) {
	require := require.New(t)

	ks, err := NewPlainKeyStore(".")
	require.Nil(err)
	addr := testutil.ConstructAddress(Pubkey, Prikey)
	filePath := filepath.Join(".", addr.RawAddress)
	defer os.Remove(filePath)
	//Test Store
	err = ks.Store("123", addr)
	require.Equal(ErrAddr, errors.Cause(err))

	err = ks.Store(RawAddr, addr)
	require.Nil(err)

	err = ks.Store(RawAddr, addr)
	require.Equal(ErrExist, errors.Cause(err))

	// Test Get
	_, err = ks.Get("123")
	require.Equal(ErrAddr, errors.Cause(err))

	_, err = ks.Get("io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3")
	require.Equal(ErrNotExist, err)

	val, err := ks.Get(addr.RawAddress)
	require.Nil(err)
	require.Equal(addr, val)

	// Test Has
	_, err = ks.Has("123")
	require.Equal(ErrAddr, errors.Cause(err))

	exist, err := ks.Has(RawAddr)
	require.Nil(err)
	require.Equal(true, exist)

	// Test Remove
	err = ks.Remove("123")
	require.Equal(ErrAddr, errors.Cause(err))

	err = ks.Remove("io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3")
	require.Equal(ErrNotExist, err)

	err = ks.Remove(RawAddr)
	require.Nil(err)
	exist, err = ks.Has(RawAddr)
	require.Nil(err)
	require.Equal(false, exist)
}

func TestMemKeyStore(t *testing.T) {
	require := require.New(t)

	ks := NewMemKeyStore()
	addr := testutil.ConstructAddress(Pubkey, Prikey)
	// Test Store
	err := ks.Store("123", addr)
	require.Equal(ErrAddr, errors.Cause(err))

	err = ks.Store(RawAddr, addr)
	require.Nil(err)

	err = ks.Store(RawAddr, addr)
	require.Equal(ErrExist, err)

	// Test Get
	_, err = ks.Get("123")
	require.Equal(ErrAddr, errors.Cause(err))

	_, err = ks.Get("io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3")
	require.Equal(ErrNotExist, err)

	val, err := ks.Get(RawAddr)
	require.Nil(err)
	require.Equal(addr, val)

	// Test Has
	_, err = ks.Has("123")
	require.Equal(ErrAddr, errors.Cause(err))

	exist, err := ks.Has(RawAddr)
	require.Nil(err)
	require.Equal(true, exist)

	// Test Remove
	err = ks.Remove("123")
	require.Equal(ErrAddr, errors.Cause(err))

	err = ks.Remove("io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3")
	require.Equal(ErrNotExist, err)

	err = ks.Remove(RawAddr)
	require.Nil(err)
	exist, err = ks.Has(RawAddr)
	require.Nil(err)
	require.Equal(false, exist)
}
