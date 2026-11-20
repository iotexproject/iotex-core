package bucketguard

import (
	"bytes"
	"encoding/hex"
	"math"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Pin the wire layout independently of the encoder/decoder round trip.
func TestVaultStateWireLayout(t *testing.T) {
	for _, tc := range []struct {
		value interface{ Serialize() ([]byte, error) }
		hex   string
	}{
		{&VaultBucket{VaultIndex: 1, Owner: bytes.Repeat([]byte{1}, 20), Amount: []byte{100}}, "0801121401010101010101010101010101010101010101011a0164"},
		{&BucketGuardV2{BucketClass: ClassAVault, Generation: 2, GuardNonce: 3, NextRequestID: 4, BoundHeight: 5}, "18012002280330043805"},
	} {
		b, err := tc.value.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(b); got != tc.hex {
			t.Fatalf("wire layout: got %s, want %s", got, tc.hex)
		}
	}
}

func TestVaultStateDecode(t *testing.T) {
	type record interface {
		Serialize() ([]byte, error)
		Deserialize([]byte) error
	}
	for _, tc := range []struct {
		name  string
		value record
		fresh func() record
	}{
		{"vault", &VaultBucket{VaultIndex: math.MaxUint64, Owner: bytes.Repeat([]byte{1}, 20), Amount: bytes.Repeat([]byte{0xff}, 32)}, func() record { return &VaultBucket{} }},
		{"guard", &BucketGuardV2{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: bytes.Repeat([]byte{2}, 48), BucketClass: ClassAVault, Generation: 1, GuardNonce: 2, NextRequestID: 3, BoundHeight: 4, UnbindDelay: UnbindDelayBlocks, CancelCooldown: 5, Pending: &PendingUnbind{2, 4, 4 + UnbindDelayBlocks}}, func() record { return &BucketGuardV2{} }},
		{"tombstone", &BucketGuardV2{BucketClass: ClassAVault, Generation: 7, GuardNonce: 8, NextRequestID: 9, BoundHeight: 10}, func() record { return &BucketGuardV2{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.value.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			got := tc.fresh()
			if err := got.Deserialize(b); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.value, got) {
				t.Fatalf("roundtrip: %+v", got)
			}
			bad := [][]byte{{0x80}, append(bytes.Clone(b), 0x78, 1), append(bytes.Clone(b), b...), b[:len(b)-1], append([]byte{8, 0}, b...)}
			for _, raw := range bad {
				if got.Deserialize(raw) == nil {
					t.Fatalf("accepted malformed state %x", raw)
				}
				if !reflect.DeepEqual(tc.value, got) {
					t.Fatal("failed decode changed destination")
				}
			}
			// The key and amount must never alias the input backend buffer.
			for i := range b {
				b[i] = 0
			}
			if !reflect.DeepEqual(tc.value, got) {
				t.Fatal("decoded state aliases input")
			}
		})
	}
}

func FuzzVaultStateDecode(f *testing.F) {
	v := &VaultBucket{Owner: bytes.Repeat([]byte{1}, 20), Amount: []byte{1}}
	b, _ := v.Serialize()
	f.Add(b)
	g := &BucketGuardV2{BucketClass: ClassAVault, Generation: 1}
	b, _ = g.Serialize()
	f.Add(b)
	f.Add([]byte{0x80})
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, r := range []interface {
			Serialize() ([]byte, error)
			Deserialize([]byte) error
		}{new(VaultBucket), new(BucketGuardV2)} {
			if err := r.Deserialize(b); err == nil {
				enc, err := r.Serialize()
				if err != nil || !bytes.Equal(enc, b) {
					t.Fatal("accepted noncanonical state")
				}
			}
		}
	})
}
