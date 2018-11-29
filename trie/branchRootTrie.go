// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type branchRootTrie struct {
	tc   SameKeyLenTrieContext
	root *branchNode
}

func newBranchRootTrie(tc SameKeyLenTrieContext, rootHash hash.Hash32B) (*branchRootTrie, error) {
	tr := &branchRootTrie{tc: tc}
	if err := tr.SetRoot(rootHash); err != nil {
		return nil, err
	}
	return tr, nil
}

func (tr *branchRootTrie) RootHash() hash.Hash32B {
	h, err := nodeHash(tr.root)
	if err != nil {
		return hash.ZeroHash32B
	}
	return h
}

func (tr *branchRootTrie) SetRoot(rootHash hash.Hash32B) error {
	logger.Debug().Hex("hash", rootHash[:]).Msg("reset root hash")
	root, err := tr.getRootByHash(rootHash)
	if err != nil {
		return err
	}
	tr.root = root
	return nil
}

func (tr *branchRootTrie) getRootByHash(rootHash hash.Hash32B) (*branchNode, error) {
	var root *branchNode
	switch rootHash {
	case EmptyRoot:
		var err error
		root, err = newBranchNodeAndSave(tr.tc, nil)
		if err != nil {
			return nil, err
		}
	case hash.ZeroHash32B:
		return nil, errors.New("zero hash is an invalid hash")
	default:
		node, err := loadNodeFromDB(tr.tc, rootHash)
		if err != nil {
			return nil, err
		}
		if root = node.(*branchNode); root == nil {
			return nil, errors.Wrapf(ErrInvalidTrie, "root should be a branch")
		}
	}
	return root, nil
}

func (tr *branchRootTrie) Get(key []byte) ([]byte, error) {
	logger.Debug().Hex("key", key).Msg("Get value by key")
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return nil, err
	}
	t := tr.root.search(tr.tc, kt, 0)
	if t == nil {
		return nil, ErrNotExist
	}
	if l := t.(*leafNode); l != nil {
		return l.Value(), nil
	}
	return nil, ErrInvalidTrie
}

func (tr *branchRootTrie) Delete(key []byte) error {
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	child, err := tr.root.child(tr.tc, kt[0])
	if err != nil {
		return errors.Wrapf(ErrNotExist, "key %x does not exist", kt)
	}
	newChild, err := child.delete(tr.tc, kt, 1)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.updateChild(tr.tc, kt[0], newChild)
	if err != nil {
		return err
	}
	tr.root = newRoot
	return nil
}

func (tr *branchRootTrie) Upsert(key []byte, value []byte) error {
	logger.Debug().Hex("key", key).Hex("value", value).Msg("Upsert into branch root trie")
	kt, err := tr.checkKeyType(key)
	if err != nil {
		return err
	}
	newRoot, err := tr.root.upsert(tr.tc, kt, 0, value)
	if err != nil {
		return err
	}
	if bn, ok := newRoot.(*branchNode); ok {
		tr.root = bn
		return nil
	}
	return errors.Errorf("unexpected new root %x", newRoot)
}

func (tr *branchRootTrie) checkKeyType(key []byte) (keyType, error) {
	if len(key) != tr.tc.KeyLength {
		return nil, errors.Errorf("invalid key length %d", len(key))
	}
	kt := make([]byte, tr.tc.KeyLength)
	copy(kt, key)

	return kt, nil
}
