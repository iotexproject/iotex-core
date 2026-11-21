package bucketguard_test

import (
	"bytes"
	"fmt"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

// Example shows the calls a staking handler makes for a bind and a guarded
// withdraw. Storage, the bucket effect and receipts are the handler's own.
func Example() {
	const bucket = 7
	// The node reads the chain ID and the genesis hash from its context; a
	// wallet gets the domain from the node or from published vectors.
	domain := bucketguard.NewDomain(2, [32]byte{0x9e})
	p := bucketguard.ParamsV1()
	alg := pqcrypto.AlgSLHDSA192s
	owner, _ := address.FromBytes(bytes.Repeat([]byte{0xa1}, 20))
	recipient, _ := address.FromBytes(bytes.Repeat([]byte{0xb0}, 20))
	key, _ := signer.NewKeyFromSeeds(alg, bytes.Repeat([]byte{1}, signer.SeedSize(alg)))

	// Wallet: bind key, proving possession with ACCEPT (no CALL, zero hash).
	pub := key.PublicKey().Bytes()
	acc, _ := key.Sign([]byte(bucketguard.AcceptContext), bucketguard.EncodeAccept(domain, bucket, 1, owner, alg, pub, [bucketguard.CallHashLen]byte{}))
	bind := &bucketguard.BindBucketGuard{BucketIndex: bucket, AlgID: alg, PublicKey: pub, AcceptSig: acc}

	// Node: stateless check, then (after bucket/native/owner checks) authorize.
	fmt.Println("bind valid:", bind.Validate(p) == nil, "gas:", bind.IntrinsicGas(p))
	guard, err := bucketguard.AuthorizeBind(p, domain, owner, bucketguard.BindScope{}, nil, bind, 100)
	fmt.Println("bound:", err == nil, "generation:", guard.Generation)

	// The chain the node executes on; the wallet reads the anchor block from it.
	blockHash := func(h uint64) ([32]byte, error) { return [32]byte{byte(h)}, nil }
	anchor, _ := blockHash(140)

	// Wallet: sign a withdraw to a fresh address, anchored to the recent block
	// 140; anyone may submit it.
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: bucket, Generation: guard.Generation, Nonce: guard.Nonce,
		Deadline: 200, AnchorHeight: 140, AnchorHash: anchor[:],
		Op: bucketguard.OpWithdraw, Recipient: recipient.Bytes(),
	}
	op.Sig, _ = key.Sign([]byte(bucketguard.CallContext), bucketguard.EncodeCall(domain, owner, op))

	// Node: IIP §9.2 order — guard, sponsor, counters and anchor, staking
	// preconditions, signatures, effect.
	fmt.Println("valid:", op.Validate(p) == nil)
	fmt.Println("check:", bucketguard.CheckOp(p, guard, op, bucketguard.OpEnv{Height: 150, Sender: recipient, BlockHash: blockHash}))
	grant, err := bucketguard.VerifyOp(p, domain, owner, guard, op)
	fmt.Println("verify:", err, "grant for:", grant.Op())
	_, deleted := bucketguard.Next(guard, op, 150)
	fmt.Println("guard deleted:", deleted)

	// Output:
	// bind valid: true gas: 1788100
	// bound: true generation: 1
	// valid: true
	// check: <nil>
	// verify: <nil> grant for: WITHDRAW
	// guard deleted: true
}
