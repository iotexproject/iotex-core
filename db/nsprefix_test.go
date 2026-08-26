// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
)

// PebbleDB keys are Hash160b(ns)[:8] || key, and decodeKey() strips those 8 bytes
// without checking which namespace produced them. Two namespaces sharing a prefix
// would therefore be one logical bucket in pebble and two separate buckets in
// bolt: the same query would return different states on the two engines, which is
// a chain fork.
//
// Why a unit test rather than a startup assertion:
//   - the fixed namespaces are compile-time constants, so a test catches a bad new
//     namespace at the moment it is introduced, on CI, instead of after a node has
//     already written data with it. A startup check would fail a node in the field
//     for something that can only ever be a source-code mistake.
//   - the only dynamic family is ContractStaking{Bucket,BucketType}NamespacePrefix +
//     hex(contractAddr), and the contract set comes from genesis config, i.e. it is
//     enumerable at build time. The addresses below are the ones the node actually
//     configures (see chainservice/builder.go, which builds one indexer per
//     configured system staking contract).
//   - db.CheckNamespacePrefixCollision is exported, so if the contract family ever
//     becomes genuinely open-ended (e.g. contracts registered on-chain at runtime),
//     the same check can be wired into node startup or into contract registration
//     without any further work.

func fixedNamespaces() []string {
	return []string{
		state.SystemNamespace,
		state.AccountKVNamespace,
		state.RewardingNamespace,
		state.StakingNamespace,
		state.StakingViewNamespace,
		state.StakingContractMetaNamespace,
		state.CandidateNamespace,
		state.CandsMapNamespace,
		state.CodeKVNameSpace,
		state.ContractKVNameSpace,
		state.PreimageKVNameSpace,
		// bare prefixes are not namespaces themselves, but include them so that a
		// future namespace equal to a prefix is caught too
		state.ContractStakingBucketNamespacePrefix,
		state.ContractStakingBucketTypeNamespacePrefix,
	}
}

func contractStakingNamespaces(t *testing.T, addrs []string) []string {
	var out []string
	for _, a := range addrs {
		if len(a) == 0 {
			continue
		}
		addr, err := address.FromString(a)
		require.NoError(t, err, "bad configured contract address %s", a)
		out = append(out,
			fmt.Sprintf("%s%x", state.ContractStakingBucketNamespacePrefix, addr.Bytes()),
			fmt.Sprintf("%s%x", state.ContractStakingBucketTypeNamespacePrefix, addr.Bytes()),
		)
	}
	return out
}

func TestNamespacePrefixCollision(t *testing.T) {
	r := require.New(t)

	g := genesis.Default
	namespaces := fixedNamespaces()
	namespaces = append(namespaces, contractStakingNamespaces(t, []string{
		g.SystemStakingContractAddress,
		g.SystemStakingContractV2Address,
		g.SystemStakingContractV3Address,
		g.NativeStakingContractAddress,
	})...)

	r.NoError(db.CheckNamespacePrefixCollision(namespaces))
	// listing the same namespace twice is not a collision
	r.NoError(db.CheckNamespacePrefixCollision([]string{"Account", "Account"}))
	// (the detection path itself is exercised in nsprefix_internal_test.go, where a
	// collision can be forced -- constructing a real Hash160b[:8] collision by hand
	// would be ~2^32 work.)
}

// TestNamespacePrefixCollisionDynamicFamily widens the check over a large,
// deterministic sample of synthetic contract addresses. A pass is weak evidence on
// its own (an 8-byte prefix gives a birthday bound around 2^32 namespaces before a
// random collision is likely, and we are nowhere near that), but it does catch a
// systematic bug -- e.g. a namespace builder that truncates or lower-cases the
// address so that distinct contracts map to the same string.
//
// Note on adversarial collisions: contract addresses are derived from the deployer
// and nonce, so grinding an address whose Hash160b[:8] matches an existing
// namespace is ~2^64 work. This is a correctness guard, not a security boundary.
func TestNamespacePrefixCollisionDynamicFamily(t *testing.T) {
	r := require.New(t)
	rnd := rand.New(rand.NewSource(1))
	namespaces := fixedNamespaces()
	for i := 0; i < 5000; i++ {
		var b [20]byte
		_, err := rnd.Read(b[:])
		r.NoError(err)
		addr, err := address.FromBytes(b[:])
		r.NoError(err)
		namespaces = append(namespaces,
			fmt.Sprintf("%s%x", state.ContractStakingBucketNamespacePrefix, addr.Bytes()),
			fmt.Sprintf("%s%x", state.ContractStakingBucketTypeNamespacePrefix, addr.Bytes()),
		)
	}
	r.NoError(db.CheckNamespacePrefixCollision(namespaces))
}
