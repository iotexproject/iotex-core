// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package keystore

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

var (
	// ErrKey indicates the error of key
	ErrKey = errors.New("key error")
	// ErrAddr indicates the error of address
	ErrAddr = errors.New("address error")
	// ErrExist is the error that the key already exists in map
	ErrExist = errors.New("key already exists")
	// ErrNotExist is the error that the key does not exist in map
	ErrNotExist = errors.New("key does not exist")
)

// Key defines the struct to be stored in keystore object
type Key struct {
	PublicKey  string
	PrivateKey string
	RawAddress string
}

// KeyStore defines a interface that supports operations on keystore object
type KeyStore interface {
	Has(string) (bool, error)
	Get(string) (*iotxaddress.Address, error)
	Store(string, *iotxaddress.Address) error
	Remove(string) error
}

// plainKeyStore is a filesystem keystore which implements KeyStore interface
type plainKeyStore struct {
	directory string
}

// MemKeyStore is an in-memory keystore which implements KeyStore interface
type MemKeyStore struct {
	accounts map[string]*iotxaddress.Address
}

// NewPlainKeyStore returns a new instance of plain keystore
func NewPlainKeyStore(dir string) (KeyStore, error) {
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}
	return &plainKeyStore{directory: dir}, nil
}

// Has returns whether the raw address already exists in keystore filesystem
func (ks *plainKeyStore) Has(rawAddr string) (bool, error) {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return false, errors.Wrap(ErrAddr, "Address format is invalid")
	}
	filePath := filepath.Join(ks.directory, rawAddr)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get returns iotxaddress from keystore filesystem given raw address
func (ks *plainKeyStore) Get(rawAddr string) (*iotxaddress.Address, error) {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return nil, errors.Wrap(ErrAddr, "Address format is invalid")
	}
	filePath := filepath.Join(ks.directory, rawAddr)
	fd, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotExist
		}
		return nil, err
	}
	defer fd.Close()
	key := &Key{}
	if err := json.NewDecoder(fd).Decode(key); err != nil {
		return nil, err
	}
	return keyToAddr(key)
}

// Store stores iotxaddress in keystore filesystem
func (ks *plainKeyStore) Store(rawAddr string, address *iotxaddress.Address) error {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return errors.Wrap(ErrAddr, "Address format is invalid")
	}
	filePath := filepath.Join(ks.directory, rawAddr)

	_, err := os.Stat(filePath)
	if err == nil {
		return ErrExist
	}
	if !os.IsNotExist(err) {
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	key, err := addrToKey(address)
	if err != nil {
		return err
	}
	sKey, err := json.Marshal(key)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, bytes.NewReader(sKey))
	return err
}

// Remove removes the iotxaddress from keystore filesystem given raw address
func (ks *plainKeyStore) Remove(rawAddr string) error {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return errors.Wrap(ErrAddr, "Address format is invalid")
	}

	filePath := filepath.Join(ks.directory, rawAddr)
	err := os.Remove(filePath)
	if os.IsNotExist(err) {
		return ErrNotExist
	}
	return err
}

// NewMemKeyStore creates a new instance of MemKeyStore
func NewMemKeyStore() KeyStore {
	return &MemKeyStore{
		accounts: make(map[string]*iotxaddress.Address),
	}
}

// Has returns whether the given raw address already exists in map
func (ks *MemKeyStore) Has(rawAddr string) (bool, error) {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return false, errors.Wrap(ErrAddr, "Address format is invalid")
	}
	_, ok := ks.accounts[rawAddr]
	return ok, nil
}

// Get returns iotxaddress stored in map given raw address of the account
func (ks *MemKeyStore) Get(rawAddr string) (*iotxaddress.Address, error) {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return nil, errors.Wrap(ErrAddr, "Address format is invalid")
	}
	addr, ok := ks.accounts[rawAddr]
	if !ok {
		return nil, ErrNotExist
	}
	return addr, nil
}

// Store stores iotxaddress in map
func (ks *MemKeyStore) Store(rawAddr string, addr *iotxaddress.Address) error {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return errors.Wrap(ErrAddr, "Address format is invalid")
	}
	// check if the key already exists in map
	if _, ok := ks.accounts[rawAddr]; ok {
		return ErrExist
	}

	ks.accounts[rawAddr] = addr
	return nil
}

// Remove removes the entry corresponding to the given raw address from map if exists
func (ks *MemKeyStore) Remove(rawAddr string) error {
	// check if sender's address is valid
	pkhash := iotxaddress.GetPubkeyHash(rawAddr)
	if pkhash == nil {
		return errors.Wrap(ErrAddr, "Address format is invalid")
	}
	_, ok := ks.accounts[rawAddr]
	if !ok {
		return ErrNotExist
	}
	delete(ks.accounts, rawAddr)
	return nil
}

//======================================
// private functions
//======================================
func keyToAddr(key *Key) (*iotxaddress.Address, error) {
	if key == nil {
		return nil, errors.Wrapf(ErrKey, "key must not be nil")
	}
	publicKey, err := keypair.DecodePublicKey(key.PublicKey)
	if err != nil {
		return nil, err
	}
	privateKey, err := keypair.DecodePrivateKey(key.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &iotxaddress.Address{PublicKey: publicKey, PrivateKey: privateKey, RawAddress: key.RawAddress}, nil
}

func addrToKey(addr *iotxaddress.Address) (*Key, error) {
	if addr == nil {
		return nil, errors.Wrapf(ErrAddr, "address must not be nil")
	}
	publicKey := keypair.EncodePublicKey(addr.PublicKey)
	privateKey := keypair.EncodePrivateKey(addr.PrivateKey)
	return &Key{PublicKey: publicKey, PrivateKey: privateKey, RawAddress: addr.RawAddress}, nil
}
