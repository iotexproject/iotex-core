package action

import (
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
)

// Bundle represents a collection of actions or items grouped together.
type Bundle struct {
	items             []*SealedEnvelope
	targetBlockHeight uint64
}

// NewBundle creates a new Bundle
func NewBundle() *Bundle {
	return &Bundle{}
}

func (b *Bundle) validateItem(item *SealedEnvelope) error {
	if item == nil {
		return ErrNilAction
	}
	/*
		switch act := item.Action().(type) {
		case *Transfer, *Execution:
			// Transfer and Execution actions are allowed in bundles
		case *txContainer:
			// act
		default:
			return ErrInvalidAct
		}
	*/
	return nil
}

// Add appends an item to the bundle.
func (b *Bundle) Add(items ...*SealedEnvelope) error {
	for _, item := range items {
		if err := b.validateItem(item); err != nil {
			return err
		}
	}
	b.items = append(b.items, items...)
	return nil
}

// SetTargetBlockHeight sets the target block height for the bundle.
func (b *Bundle) SetTargetBlockHeight(height uint64) {
	b.targetBlockHeight = height
}

// TargetBlockHeight returns the target block height for the bundle.
func (b *Bundle) TargetBlockHeight() uint64 {
	return b.targetBlockHeight
}

// Len returns the number of items in the bundle.
func (b *Bundle) Len() int {
	return len(b.items)
}

// Proto converts the Bundle to its protobuf representation.
func (b *Bundle) Proto() *iotextypes.Bundle {
	if b == nil {
		return nil
	}
	pbBundle := &iotextypes.Bundle{
		Actions: make([]*iotextypes.Action, 0, len(b.items)),
	}
	for _, item := range b.items {
		pbBundle.Actions = append(pbBundle.Actions, item.Proto())
	}
	pbBundle.TargetBlockNumber = b.targetBlockHeight
	return pbBundle
}

// LoadProto populates the Bundle from its protobuf representation.
func (b *Bundle) LoadProto(pbBundle *iotextypes.Bundle, d *Deserializer) error {
	if pbBundle == nil || len(pbBundle.Actions) == 0 {
		return ErrNilProto
	}
	if b == nil {
		return ErrNilAction
	}
	selps := make([]*SealedEnvelope, 0, len(pbBundle.Actions))
	for _, pbAction := range pbBundle.Actions {
		selp, err := d.ActionToSealedEnvelope(pbAction)
		if err != nil {
			return err
		}
		if err := b.validateItem(selp); err != nil {
			return err
		}
		selps = append(selps, selp)
	}
	b.items = selps
	b.targetBlockHeight = pbBundle.TargetBlockNumber
	return nil
}

// Serialize converts the Bundle to its byte representation.
func (b *Bundle) Serialize() []byte {
	return byteutil.Must(proto.Marshal(b.Proto()))
}

func (b *Bundle) Hash() hash.Hash256 {
	if b == nil || len(b.items) == 0 {
		return hash.ZeroHash256
	}

	return hash.BytesToHash256(b.Serialize())
}

// Gas calculates the total gas of all items in the bundle.
func (b *Bundle) Gas() uint64 {
	if b == nil || len(b.items) == 0 {
		return 0
	}

	var totalGas uint64
	for _, item := range b.items {
		totalGas += item.Gas()
	}
	return totalGas
}

// ForEach executes the given function for each item in the bundle.
func (b *Bundle) ForEach(f func(item *SealedEnvelope) error) error {
	for _, item := range b.items {
		if err := f(item); err != nil {
			return err
		}
	}
	return nil
}
