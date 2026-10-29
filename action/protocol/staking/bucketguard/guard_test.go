package bucketguard

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// Expected bytes are derived by hand from the BucketGuard message definition.
func TestGuardSerializeVectors(t *testing.T) {
	for _, tc := range []struct {
		name string
		g    Guard
		want string
	}{
		{"guarded", Guard{AlgID: alg, PublicKey: bytes.Repeat([]byte{7}, 48), Generation: 1, Nonce: 2, BoundHeight: 100},
			"0812" + "1230" + strings.Repeat("07", 48) + "1801" + "2002" + "2864"},
		{"tombstone", Guard{Generation: 3, Nonce: 9, BoundHeight: 100}, "1803" + "2009" + "2864"},
		{"tombstone at height 0", Guard{Generation: 3, Nonce: 9}, "1803" + "2009"},
		{"empty", Guard{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.g.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(b); got != tc.want {
				t.Fatalf("Serialize = %s, want %s", got, tc.want)
			}
			var back Guard
			if err := back.Deserialize(b); err != nil {
				t.Fatal(err)
			}
			if back.AlgID != tc.g.AlgID || !bytes.Equal(back.PublicKey, tc.g.PublicKey) || back.Generation != tc.g.Generation ||
				back.Nonce != tc.g.Nonce || back.BoundHeight != tc.g.BoundHeight {
				t.Fatalf("round trip: got %+v, want %+v", back, tc.g)
			}
		})
	}
}

func TestGuardRejects(t *testing.T) {
	if _, err := (&Guard{AlgID: alg}).Serialize(); err == nil {
		t.Error("tombstone with algID serialized")
	}
	if _, err := (&Guard{AlgID: alg, PublicKey: make([]byte, 47)}).Serialize(); err == nil {
		t.Error("short key serialized")
	}
	key := "1230" + strings.Repeat("07", 48)
	for name, h := range map[string]string{
		"unknown field":   "3001",
		"repeated field":  "18011802",
		"out of order":    "20021801",
		"zero encoded":    "1800",
		"long varint":     "188100",
		"wrong wire type": "1a0101",
		"truncated":       "12",
		"key without alg": key,
		"tombstone alg":   "0812",
	} {
		var g Guard
		if err := g.Deserialize(mustHex(t, h)); err == nil {
			t.Errorf("%s: %s accepted", name, h)
		}
	}
}

func FuzzGuardDeserialize(f *testing.F) {
	g := Guard{AlgID: alg, PublicKey: bytes.Repeat([]byte{7}, 48), Generation: 1, Nonce: 2, BoundHeight: 100}
	seed, _ := g.Serialize()
	f.Add(seed)
	f.Add([]byte{0x18, 0x03, 0x20, 0x09})
	f.Fuzz(func(t *testing.T, b []byte) {
		var g Guard
		if g.Deserialize(b) != nil {
			return
		}
		out, err := g.Serialize()
		if err != nil || !bytes.Equal(out, b) {
			t.Fatalf("accepted %x but re-serialized to %x (%v)", b, out, err)
		}
	})
}
