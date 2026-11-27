// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

// BenchmarkGuardedBucketOpDecode times what a node does with a guarded op
// before execution: decode it from the wire, run the stateless checks and
// price it. "transfer+guard" is the largest op (two keys and signatures).
func BenchmarkGuardedBucketOpDecode(b *testing.B) {
	for _, name := range []string{"unstake", "transfer+guard"} {
		core := &iotextypes.ActionCore{}
		NewGuardedBucketOp(testGuardedOps()[name]).FillAction(core)
		wire, err := proto.Marshal(core)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(wire)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				decoded := &iotextypes.ActionCore{}
				if err := proto.Unmarshal(wire, decoded); err != nil {
					b.Fatal(err)
				}
				elp := &envelope{}
				if err := elp.loadProtoActionPayload(decoded); err != nil {
					b.Fatal(err)
				}
				act := elp.payload.(*GuardedBucketOp)
				if err := act.SanityCheck(); err != nil {
					b.Fatal(err)
				}
				if _, err := act.IntrinsicGas(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
