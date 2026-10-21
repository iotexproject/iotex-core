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
	const chainID, bucket = 2, 7
	p := bucketguard.ParamsV1()
	alg := pqcrypto.AlgSLHDSA192s
	owner, _ := address.FromBytes(bytes.Repeat([]byte{0xa1}, 20))
	recipient, _ := address.FromBytes(bytes.Repeat([]byte{0xb0}, 20))
	key, _ := signer.NewKeyFromSeeds(alg, bytes.Repeat([]byte{1}, signer.SeedSize(alg)))
	ctx := []byte(bucketguard.Context)

	// Wallet: bind key, proving possession with ACCEPT.
	pub := key.PublicKey().Bytes()
	acc, _ := key.Sign(ctx, bucketguard.EncodeAccept(chainID, bucket, 1, owner, alg, pub, 0))
	bind := &bucketguard.BindBucketGuard{BucketIndex: bucket, AlgID: alg, PublicKey: pub, AcceptSig: acc}

	// Node: stateless check, then (after bucket/native/owner checks) authorize.
	fmt.Println("bind valid:", bind.Validate(p) == nil, "gas:", bind.IntrinsicGas(p))
	guard, err := bucketguard.AuthorizeBind(p, chainID, owner, bucketguard.BindScope{}, nil, bind, 100)
	fmt.Println("bound:", err == nil, "generation:", guard.Generation)

	// Wallet: sign a withdraw to a fresh address; anyone may submit it.
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: bucket, Generation: guard.Generation, Nonce: guard.Nonce,
		Deadline: 200, Op: bucketguard.OpWithdraw, Recipient: recipient.Bytes(),
	}
	op.Sig, _ = key.Sign(ctx, bucketguard.EncodeCall(chainID, owner, op))

	// Node: IIP §9.2 order — counters, staking preconditions, signatures, effect.
	fmt.Println("valid:", op.Validate(p) == nil)
	fmt.Println("check:", bucketguard.CheckOp(guard, op, 150))
	fmt.Println("verify:", bucketguard.VerifyOp(p, chainID, owner, guard, op))
	_, deleted := bucketguard.Next(guard, op, 150)
	fmt.Println("guard deleted:", deleted)

	// Output:
	// bind valid: true gas: 300496
	// bound: true generation: 1
	// valid: true
	// check: <nil>
	// verify: <nil>
	// guard deleted: true
}
