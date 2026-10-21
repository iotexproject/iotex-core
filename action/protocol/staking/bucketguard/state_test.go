package bucketguard

import (
	"bytes"
	"testing"
)

func TestGuardStateRoundTrip(t *testing.T) {
	pub := bytes.Repeat([]byte{0x11}, PubKeySize)
	g := &Guard{
		AlgID:       AlgSLHDSA192s,
		PublicKey:   pub,
		Generation:  7,
		Nonce:       9,
		BoundHeight: 11,
	}
	b, err := g.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got Guard
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if !got.Guarded || got.AlgID != AlgSLHDSA192s || !bytes.Equal(got.PublicKey, pub) ||
		got.Generation != 7 || got.Nonce != 9 || got.BoundHeight != 11 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestGuardStateTombstone(t *testing.T) {
	g := &Guard{Generation: 5, Nonce: 7}
	b, err := g.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x18, 0x05, 0x20, 0x07}
	if !bytes.Equal(b, want) {
		t.Fatalf("tombstone encoding = %x, want %x", b, want)
	}
	var got Guard
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	if got.Guarded || got.AlgID != 0 || len(got.PublicKey) != 0 ||
		got.Generation != 5 || got.Nonce != 7 || got.BoundHeight != 0 {
		t.Fatalf("tombstone round trip mismatch: %+v", got)
	}
}

func TestGuardStateRejectsMalformed(t *testing.T) {
	cases := map[string][]byte{
		"emptyTag":       {0x00},
		"unknownField":   {0x30, 0x01},
		"duplicateField": {0x18, 0x01, 0x18, 0x02},
		"nonCanonical":   {0x18, 0x81, 0x00},
		"truncated":      {0x12, 0x30},
		"wrongWire":      {0x0a, 0x01, 0x00},
		"algOverflow":    {0x08, 0xff, 0xff, 0xff, 0xff, 0x1f},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			var g Guard
			if err := g.UnmarshalBinary(data); err == nil {
				t.Fatal("malformed guard accepted")
			}
		})
	}
}
