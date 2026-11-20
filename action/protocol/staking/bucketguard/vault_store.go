package bucketguard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Serialize returns the canonical proto3 layout of VaultBucket in §13.2.
func (v *VaultBucket) Serialize() ([]byte, error) {
	if v == nil || !validAddress(v.Owner) || (len(v.Amount) > 0 && v.Amount[0] == 0) || len(v.Amount) > 32 {
		return nil, ErrMalformed
	}
	b := appendVarintField(nil, 1, v.VaultIndex)
	b = appendBytesField(b, 2, v.Owner)
	return appendBytesField(b, 3, v.Amount), nil
}

// Serialize returns the new BucketGuardV2 proto3 layout. It does not reuse or
// change the deployed Draft 4 Guard serializer.
func (g *BucketGuardV2) Serialize() ([]byte, error) {
	if g == nil || (g.BucketClass != ClassAVault && g.BucketClass != ClassBStaking) {
		return nil, ErrClassMismatch
	}
	if err := (&Guard{AlgID: g.AlgID, PublicKey: g.PublicKey}).check(); err != nil {
		return nil, err
	}
	b := appendVarintField(nil, 1, uint64(g.AlgID))
	b = appendBytesField(b, 2, g.PublicKey)
	b = appendVarintField(b, 3, uint64(g.BucketClass))
	b = appendVarintField(b, 4, g.Generation)
	b = appendVarintField(b, 5, g.GuardNonce)
	b = appendVarintField(b, 6, g.NextRequestID)
	b = appendVarintField(b, 7, g.BoundHeight)
	b = appendVarintField(b, 8, g.UnbindDelay)
	b = appendVarintField(b, 9, g.CancelCooldown)
	if g.Pending != nil {
		p := appendVarintField(nil, 1, g.Pending.RequestID)
		p = appendVarintField(p, 2, g.Pending.StartHeight)
		p = appendVarintField(p, 3, g.Pending.ExpiresAt)
		b = appendBytesField(b, 10, p)
	}
	return b, nil
}

// vaultFields reads strictly ordered, minimally encoded proto3 fields. The
// final Serialize comparison in each decoder also rejects explicit defaults.
func vaultFields(b []byte, max protowire.Number, apply func(protowire.Number, uint64, []byte) error) error {
	last := protowire.Number(0)
	for len(b) != 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 || num <= last || num > max {
			return fmt.Errorf("vault: invalid field tag")
		}
		last, b = num, b[n:]
		var v uint64
		var data []byte
		switch typ {
		case protowire.VarintType:
			v, n = protowire.ConsumeVarint(b)
		case protowire.BytesType:
			data, n = protowire.ConsumeBytes(b)
		default:
			return fmt.Errorf("vault: invalid wire type %d", typ)
		}
		if n < 0 {
			return protowire.ParseError(n)
		}
		if err := apply(num, v, data); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

// Deserialize decodes a canonical VaultBucket without changing v on failure.
func (v *VaultBucket) Deserialize(b []byte) error {
	var out VaultBucket
	err := vaultFields(b, 3, func(num protowire.Number, n uint64, data []byte) error {
		if (num == 1) != (data == nil) {
			return ErrMalformed
		}
		switch num {
		case 1:
			out.VaultIndex = n
		case 2:
			out.Owner = bytes.Clone(data)
		case 3:
			out.Amount = bytes.Clone(data)
		}
		return nil
	})
	if err != nil {
		return err
	}
	enc, err := out.Serialize()
	if err != nil {
		return err
	}
	if !bytes.Equal(enc, b) {
		return ErrMalformed
	}
	*v = out
	return nil
}

// Deserialize decodes a canonical V2 record, including retained tombstones.
func (g *BucketGuardV2) Deserialize(b []byte) error {
	var out BucketGuardV2
	err := vaultFields(b, 10, func(num protowire.Number, n uint64, data []byte) error {
		if (num == 2 || num == 10) != (data != nil) {
			return ErrMalformed
		}
		switch num {
		case 1:
			if n > 0xffffffff {
				return ErrMalformed
			}
			out.AlgID = pqcrypto.AlgID(n)
		case 2:
			out.PublicKey = bytes.Clone(data)
		case 3:
			if n > 0xffffffff {
				return ErrMalformed
			}
			out.BucketClass = uint32(n)
		case 4:
			out.Generation = n
		case 5:
			out.GuardNonce = n
		case 6:
			out.NextRequestID = n
		case 7:
			out.BoundHeight = n
		case 8:
			out.UnbindDelay = n
		case 9:
			out.CancelCooldown = n
		case 10:
			p := new(PendingUnbind)
			if err := vaultFields(data, 3, func(field protowire.Number, value uint64, raw []byte) error {
				if raw != nil {
					return ErrMalformed
				}
				switch field {
				case 1:
					p.RequestID = value
				case 2:
					p.StartHeight = value
				case 3:
					p.ExpiresAt = value
				}
				return nil
			}); err != nil {
				return err
			}
			out.Pending = p
		}
		return nil
	})
	if err != nil {
		return err
	}
	enc, err := out.Serialize()
	if err != nil {
		return err
	}
	if !bytes.Equal(enc, b) {
		return ErrMalformed
	}
	*g = out
	return nil
}

// State reads simulated native storage. Meta key {12} holds a big-endian
// uint64 totalVaultCount, including the initial value zero. Missing keys are nil.
func (e *VaultEngine) State(key []byte) []byte {
	if bytes.Equal(key, []byte{VaultMetaPrefix}) {
		return binary.BigEndian.AppendUint64(nil, e.totalVaultCount)
	}
	if len(key) != 9 {
		return nil
	}
	var k [9]byte
	copy(k[:], key)
	if v := e.vaults[k]; v != nil {
		b, _ := v.Serialize()
		return b
	}
	if g := e.guards[k]; g != nil {
		b, _ := g.Serialize()
		return b
	}
	return nil
}

// VaultStateEntry is a copy of one native storage entry.
type VaultStateEntry struct{ Key, Value []byte }

// StateEntries sorts keys before producing a snapshot; Go map order never
// influences serialized state. Liquid accounts and PoS fixtures are separate.
func (e *VaultEngine) StateEntries() []VaultStateEntry {
	keys := [][]byte{{VaultMetaPrefix}}
	for k := range e.vaults {
		keys = append(keys, bytes.Clone(k[:]))
	}
	for k := range e.guards {
		keys = append(keys, bytes.Clone(k[:]))
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i], keys[j]) < 0 })
	entries := make([]VaultStateEntry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, VaultStateEntry{k, e.State(k)})
	}
	return entries
}
