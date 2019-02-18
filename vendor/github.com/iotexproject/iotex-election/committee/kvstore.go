// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package committee

// KVStore defines the db interface using in committee
type KVStore interface {
	Get([]byte) ([]byte, error)
	Put([]byte, []byte) error
}

type store struct {
	kv map[string][]byte
}

func (s *store) Get([]byte) ([]byte, error) {
	return nil, nil
}

func (s *store) Put([]byte, []byte) error {
	return nil
}
