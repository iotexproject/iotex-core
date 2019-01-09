// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

var (
	// ErrKey indicates the error of key
	ErrKey = errors.New("key error")
	// ErrExist is the error that the key already exists in map
	ErrExist = errors.New("key already exists")
	// ErrNotExist is the error that the key does not exist in map
	ErrNotExist = errors.New("key does not exist")
)

// KeyStore defines an interface that supports operations on keystore object
type KeyStore interface {
	Has(string) (bool, error)
	Get(string) (keypair.PrivateKey, error)
	Store(string, keypair.PrivateKey) error
	Remove(string) error
	All() ([]string, error)
}

// plainKeyStore is a filesystem keystore which implements KeyStore interface
type plainKeyStore struct {
	directory string
}

// MemKeyStore is an in-memory keystore which implements KeyStore interface
type memKeyStore struct {
	accounts map[string]keypair.PrivateKey
}

// NewPlainKeyStore returns a new instance of plain keystore
func NewPlainKeyStore(dir string) (KeyStore, error) {
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "failed to get the status of directory %s", dir)
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, errors.Wrapf(err, "failed to make directory %s", dir)
		}
	}
	return &plainKeyStore{directory: dir}, nil
}

// Has returns whether the encoded address already exists in keystore filesystem
func (ks *plainKeyStore) Has(encodedAddr string) (bool, error) {
	if err := validateAddress(encodedAddr); err != nil {
		return false, err
	}
	filePath := filepath.Join(ks.directory, encodedAddr)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to get the status of directory %s", filePath)
	}
	return true, nil
}

// Get returns private key from keystore filesystem given encoded address
func (ks *plainKeyStore) Get(encodedAddr string) (keypair.PrivateKey, error) {
	if err := validateAddress(encodedAddr); err != nil {
		return keypair.ZeroPrivateKey, err
	}
	filePath := filepath.Join(ks.directory, encodedAddr)
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return keypair.ZeroPrivateKey, errors.Wrapf(ErrNotExist, "encoded address = %s", encodedAddr)
		}
		return keypair.ZeroPrivateKey, errors.Wrapf(err, "failed to open file %s", filePath)
	}
	keyBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return keypair.ZeroPrivateKey, errors.Wrapf(err, "failed to read file %s", filePath)
	}
	return keypair.BytesToPrivateKey(keyBytes)
}

// Store stores private key in keystore filesystem
func (ks *plainKeyStore) Store(encodedAddr string, key keypair.PrivateKey) error {
	if err := validateAddress(encodedAddr); err != nil {
		return err
	}
	filePath := filepath.Join(ks.directory, encodedAddr)

	_, err := os.Stat(filePath)
	if err == nil {
		return errors.Wrapf(ErrExist, "encoded address = %s", encodedAddr)
	}
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to get the status of file %s", filePath)
	}
	return ioutil.WriteFile(filePath, key[:], 0644)
}

// Remove removes the private key from keystore filesystem given encoded address
func (ks *plainKeyStore) Remove(encodedAddr string) error {
	if err := validateAddress(encodedAddr); err != nil {
		return err
	}
	filePath := filepath.Join(ks.directory, encodedAddr)
	err := os.Remove(filePath)
	if os.IsNotExist(err) {
		return errors.Wrapf(ErrNotExist, "encoded address = %s", encodedAddr)
	}
	return err
}

// All returns a list of encoded addresses currently stored in keystore filesystem
func (ks *plainKeyStore) All() ([]string, error) {
	fd, err := os.Open(ks.directory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open directory %s", ks.directory)
	}
	names, err := fd.Readdirnames(0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read directory names")
	}
	encodedAddrs := make([]string, 0, len(names))
	for _, encodedAddr := range names {
		if err := validateAddress(encodedAddr); err != nil {
			return nil, err
		}
		encodedAddrs = append(encodedAddrs, encodedAddr)
	}
	return encodedAddrs, nil
}

// NewMemKeyStore creates a new instance of MemKeyStore
func NewMemKeyStore() KeyStore {
	return &memKeyStore{
		accounts: make(map[string]keypair.PrivateKey),
	}
}

// Has returns whether the given encoded address already exists in map
func (ks *memKeyStore) Has(encodedAddr string) (bool, error) {
	if err := validateAddress(encodedAddr); err != nil {
		return false, err
	}
	_, ok := ks.accounts[encodedAddr]
	return ok, nil
}

// Get returns address stored in map given encoded address of the account
func (ks *memKeyStore) Get(encodedAddr string) (keypair.PrivateKey, error) {
	if err := validateAddress(encodedAddr); err != nil {
		return keypair.ZeroPrivateKey, err
	}
	key, ok := ks.accounts[encodedAddr]
	if !ok {
		return keypair.ZeroPrivateKey, errors.Wrapf(ErrNotExist, "encoded address = %s", encodedAddr)
	}
	return key, nil
}

// Store stores address in map
func (ks *memKeyStore) Store(encodedAddr string, key keypair.PrivateKey) error {
	if err := validateAddress(encodedAddr); err != nil {
		return err
	}
	// check if the key already exists in map
	if _, ok := ks.accounts[encodedAddr]; ok {
		return errors.Wrapf(ErrExist, "encoded address = %s", encodedAddr)
	}

	ks.accounts[encodedAddr] = key
	return nil
}

// Remove removes the entry corresponding to the given encoded address from map if exists
func (ks *memKeyStore) Remove(encodedAddr string) error {
	if err := validateAddress(encodedAddr); err != nil {
		return err
	}
	_, ok := ks.accounts[encodedAddr]
	if !ok {
		return errors.Wrapf(ErrNotExist, "encoded address = %s", encodedAddr)
	}
	delete(ks.accounts, encodedAddr)
	return nil
}

// All returns returns a list of encoded addresses currently stored in map
func (ks *memKeyStore) All() ([]string, error) {
	encodedAddrs := make([]string, 0, len(ks.accounts))
	for encodedAddr := range ks.accounts {
		if err := validateAddress(encodedAddr); err != nil {
			return nil, err
		}
		encodedAddrs = append(encodedAddrs, encodedAddr)
	}
	return encodedAddrs, nil
}

//======================================
// private functions
//======================================
func validateAddress(addr string) error {
	// check if the address is valid
	_, err := address.Bech32ToAddress(addr)
	if err != nil {
		return errors.Wrapf(err, "address format is invalid %s", addr)
	}
	return nil
}
