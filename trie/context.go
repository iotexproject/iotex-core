// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/iotexproject/iotex-core/db"
)

type sameKeyLenTrieContextKey struct{}

// SameKeyLenTrieContext provides the trie operation with auxiliary information.
type SameKeyLenTrieContext struct {
	// Bucket defines the bucket to store in db
	Bucket string
	// DB refers to the db to store the key-value pairs
	DB db.KVStore
	// CB is a collection of db operations
	CB db.CachedBatch
	// KeyLength defines the length of the key to be used in the trie
	KeyLength int
}

// WithSameKeyLenTrieContext add SameKeyLenTrieContext into context.
func WithSameKeyLenTrieContext(ctx context.Context, tc SameKeyLenTrieContext) context.Context {
	return context.WithValue(ctx, sameKeyLenTrieContextKey{}, tc)
}

func getSameKeyLenTrieContext(ctx context.Context) (SameKeyLenTrieContext, bool) {
	tc, ok := ctx.Value(sameKeyLenTrieContextKey{}).(SameKeyLenTrieContext)

	return tc, ok
}
