// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// VaultBucketRecord stores Class A liquid principal, outside the staking pool
// and every vote-bucket index. It never contributes to totalStaked or votes.
type VaultBucketRecord struct{ bucketguard.VaultBucket }

// BucketGuardV2Record stores the append-only Draft 5 schema independently of
// BucketGuardRecord. A keyless record is a permanent Class A tombstone.
type BucketGuardV2Record struct{ bucketguard.BucketGuardV2 }

// VaultMetaRecord is the monotonic allocator. Zero is a valid initial count.
type VaultMetaRecord struct{ TotalVaultCount uint64 }

// Serialize implements state.Serializer.
func (r *VaultBucketRecord) Serialize() ([]byte, error) { return r.VaultBucket.Serialize() }

// Deserialize implements state.Deserializer.
func (r *VaultBucketRecord) Deserialize(b []byte) error { return r.VaultBucket.Deserialize(b) }

// Serialize implements state.Serializer.
func (r *BucketGuardV2Record) Serialize() ([]byte, error) { return r.BucketGuardV2.Serialize() }

// Deserialize implements state.Deserializer.
func (r *BucketGuardV2Record) Deserialize(b []byte) error { return r.BucketGuardV2.Deserialize(b) }

// Serialize encodes message VaultMeta { uint64 totalVaultCount = 1; }.
func (r *VaultMetaRecord) Serialize() ([]byte, error) {
	if r.TotalVaultCount == 0 {
		return nil, nil
	}
	b := protowire.AppendTag(nil, 1, protowire.VarintType)
	return protowire.AppendVarint(b, r.TotalVaultCount), nil
}

// Deserialize accepts only the canonical protobuf form and is atomic on error.
func (r *VaultMetaRecord) Deserialize(b []byte) error {
	var out VaultMetaRecord
	if len(b) != 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 || num != 1 || typ != protowire.VarintType {
			return fmt.Errorf("vault meta: invalid tag")
		}
		v, size := protowire.ConsumeVarint(b[n:])
		if size < 0 || n+size != len(b) {
			return fmt.Errorf("vault meta: invalid value")
		}
		out.TotalVaultCount = v
	}
	enc, _ := out.Serialize()
	if !bytes.Equal(enc, b) {
		return fmt.Errorf("vault meta: non-canonical encoding")
	}
	*r = out
	return nil
}

func vaultGenericValue(r interface{ Serialize() ([]byte, error) }) (systemcontracts.GenericValue, error) {
	b, err := r.Serialize()
	return systemcontracts.GenericValue{PrimaryData: b}, err
}

// Encode implements systemcontracts.GenericValueContainer.
func (r *VaultBucketRecord) Encode() (systemcontracts.GenericValue, error) {
	return vaultGenericValue(r)
}

// Decode implements systemcontracts.GenericValueContainer.
func (r *VaultBucketRecord) Decode(v systemcontracts.GenericValue) error {
	return r.Deserialize(v.PrimaryData)
}

// Encode implements systemcontracts.GenericValueContainer.
func (r *BucketGuardV2Record) Encode() (systemcontracts.GenericValue, error) {
	return vaultGenericValue(r)
}

// Decode implements systemcontracts.GenericValueContainer.
func (r *BucketGuardV2Record) Decode(v systemcontracts.GenericValue) error {
	return r.Deserialize(v.PrimaryData)
}

// Encode implements systemcontracts.GenericValueContainer.
func (r *VaultMetaRecord) Encode() (systemcontracts.GenericValue, error) { return vaultGenericValue(r) }

// Decode implements systemcontracts.GenericValueContainer.
func (r *VaultMetaRecord) Decode(v systemcontracts.GenericValue) error {
	return r.Deserialize(v.PrimaryData)
}

func vaultKey(prefix byte, index uint64) []byte {
	return binary.BigEndian.AppendUint64([]byte{prefix}, index)
}

// ReadVaultBucket returns a Class A record. Absence, including a closed vault,
// is state.ErrStateNotExist; storage failures remain errors.
func ReadVaultBucket(sr protocol.StateReader, index uint64) (*VaultBucketRecord, error) {
	var r VaultBucketRecord
	_, err := sr.State(&r, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(vaultKey(_vaultBucket, index)))
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ReadVaultGuard includes the tombstone retained after a Class A vault closes.
func ReadVaultGuard(sr protocol.StateReader, index uint64) (*BucketGuardV2Record, error) {
	var r BucketGuardV2Record
	_, err := sr.State(&r, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(vaultKey(_vaultGuard, index)))
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// _stakingGuardV2 is isolated from the deployed Draft 4 guard schema (tag 9).
// The tag is declared with the other isolated record tags in protocol.go.

// ReadStakingGuardV2 reads recovery metadata without interpreting Draft 4 bytes.
func ReadStakingGuardV2(sr protocol.StateReader, index uint64) (*BucketGuardV2Record, error) {
	var r BucketGuardV2Record
	_, err := sr.State(&r, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(vaultKey(_stakingGuardV2, index)))
	return &r, err
}

func vaultStateOptions(key []byte) []protocol.StateOption {
	return []protocol.StateOption{protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(key)}
}

func (p *Protocol) handleVaultAction(ctx context.Context, act *action.VaultAction, csm CandidateStateManager) (log *receiptLog, txlogs []*action.TransactionLog, result error) {
	return p.checkVaultAction(ctx, act, csm, false)
}

func (p *Protocol) checkVaultAction(ctx context.Context, act *action.VaultAction, csm CandidateStateManager, preflight bool) (log *receiptLog, txlogs []*action.TransactionLog, result error) {
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil, nil, errors.New("bucket guard disabled")
	}
	m, err := act.Vault()
	if err != nil {
		return nil, nil, err
	}
	sm := csm.SM()
	sid := -1
	if !preflight {
		sid = sm.Snapshot()
	}
	defer func() {
		if result != nil && !preflight {
			if err := sm.Revert(sid); err != nil {
				result = errors.Wrap(err, "vault revert")
			}
		}
	}()
	ac := protocol.MustGetActionCtx(ctx)
	env := bucketguard.OpEnv{Height: protocol.MustGetBlockCtx(ctx).BlockHeight, Sender: ac.Caller, BlockHash: anchorBlockHash(ctx)}
	class := bucketguard.ClassAVault
	if strings.HasPrefix(m.Method, "guardUnbind") || m.Method == "bindBucketGuardV2" {
		class = bucketguard.ClassBStaking
	}
	var v *bucketguard.VaultBucket
	var g *bucketguard.BucketGuardV2
	var meta VaultMetaRecord
	if _, err = sm.State(&meta, vaultStateOptions([]byte{_vaultMeta})...); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, nil, err
	}
	index := m.Index
	if m.Method == "vaultCreate" {
		index = meta.TotalVaultCount
	} else if class == bucketguard.ClassAVault {
		vr, e := ReadVaultBucket(sm, index)
		if e != nil {
			if errors.Cause(e) == state.ErrStateNotExist {
				return nil, nil, vaultReceiptError(bucketguard.ErrVaultClosed)
			}
			return nil, nil, e
		}
		v = &vr.VaultBucket
		gr, e := ReadVaultGuard(sm, index)
		if e != nil {
			return nil, nil, e
		}
		g = &gr.BucketGuardV2
	} else {
		b, e := fetchNativeBucket(csm, index, bucketguard.ErrClassMismatch)
		if e != nil {
			return nil, nil, e
		}
		v = &bucketguard.VaultBucket{VaultIndex: index, Owner: b.Owner.Bytes(), Amount: b.StakedAmount.Bytes()}
		gr, e := ReadStakingGuardV2(sm, index)
		if e == nil {
			g = &gr.BucketGuardV2
		} else if errors.Cause(e) != state.ErrStateNotExist {
			return nil, nil, e
		}
		if m.Method == "bindBucketGuardV2" {
			old, e := getBucketGuard(sm, index)
			if e != nil {
				return nil, nil, e
			}
			if old.IsGuarded() {
				return nil, nil, guardReceiptError(bucketguard.ErrBucketGuarded)
			}
			if old != nil && (g == nil || g.Generation < old.Generation) {
				if g == nil {
					g = &bucketguard.BucketGuardV2{BucketClass: bucketguard.ClassBStaking, NextRequestID: 1}
				}
				g.Generation = old.Generation
				g.GuardNonce = old.Nonce
			}
			if csm.ContainsSelfStakingBucket(index) {
				return nil, nil, guardReceiptError(bucketguard.ErrGuardScope)
			}
			if _, e := NewEndorsementStateReader(sm).Get(index); e == nil {
				return nil, nil, guardReceiptError(bucketguard.ErrGuardScope)
			} else if errors.Cause(e) != state.ErrStateNotExist {
				return nil, nil, e
			}
		}
	}
	owner := ac.Caller.Bytes()
	if v != nil {
		owner = v.Owner
	}
	ownerAddr, err := address.FromBytes(owner)
	if err != nil {
		return nil, nil, err
	}
	ownerAcct, err := accountutil.LoadAccount(sm, ownerAddr)
	if err != nil {
		return nil, nil, err
	}
	if m.Method == "vaultCreate" || m.Method == "vaultAddDeposit" {
		fee := new(big.Int).Mul(new(big.Int).SetUint64(ac.IntrinsicGas), ac.GasPrice)
		required := new(big.Int).Add(m.Amount, fee)
		if ownerAcct.Balance.Cmp(required) < 0 {
			return nil, nil, vaultReceiptError(bucketguard.ErrInsufficientBalance)
		}
	}
	pool := bucketguard.VaultPool()
	poolAddr, _ := address.FromBytes(pool[:])
	poolAcct, err := accountutil.LoadAccount(sm, poolAddr)
	if err != nil {
		return nil, nil, err
	}
	engine, err := bucketguard.LoadVaultTransition(guardDomain(ctx), bucketguard.ParamsV2(), meta.TotalVaultCount, class, index, v, g, ownerAcct.Balance, poolAcct.Balance)
	if err != nil {
		return nil, nil, err
	}
	if m.Method == "vaultCreate" {
		if err = engine.Fund(owner, ownerAcct.Balance); err != nil {
			return nil, nil, err
		}
	}
	if preflight && m.Method == "guardUnbindCancel" && g != nil {
		switch {
		case m.RequestID >= g.NextRequestID:
			return nil, nil, bucketguard.ErrFutureRequestId
		case g.Pending != nil && g.Pending.RequestID != m.RequestID:
			return nil, nil, bucketguard.ErrStaleRequestId
		case g.Pending != nil && env.Height >= g.Pending.ExpiresAt:
			return nil, nil, bucketguard.ErrUnbindElapsed
		case g.Pending != nil && env.Height > math.MaxUint64-bucketguard.CancelCooldownBlocks:
			return nil, nil, bucketguard.ErrGuardMismatch
		}
	}
	if preflight && m.Method == "vaultRotateOwner" {
		newOwner, e := address.FromBytes(m.Call.NewOwner)
		if e != nil {
			return nil, nil, bucketguard.ErrMalformed
		}
		acct, e := accountutil.LoadAccount(sm, newOwner)
		if e != nil {
			return nil, nil, e
		}
		if acct.IsContract() {
			return nil, nil, errors.Wrap(bucketguard.ErrGuardScope, "vault owner cannot be contract")
		}
	}
	switch m.Method {
	case "vaultCreate":
		_, err = engine.VaultCreate(bucketguard.VaultEnv{OpEnv: env, OwnerTxNonce: ac.Nonce}, &bucketguard.VaultCreateRequest{Amount: m.Amount, OwnerTxNonce: m.OwnerTxNonce, Guard: m.Guard})
	case "vaultAddDeposit":
		err = engine.VaultAddDeposit(env, index, m.Amount)
	case "vaultWithdraw", "vaultRotateOwner", "vaultRotateGuard", "vaultRevokeSignatures":
		err = engine.VaultExecute(env, &m.Call)
	case "vaultInitiateExit":
		err = engine.VaultInitiateExit(env, index)
	case "vaultCancelExit":
		err = engine.VaultCancelExit(env, index)
	case "vaultFinalizeExit":
		err = engine.VaultFinalizeExit(env, index)
	case "bindBucketGuardV2":
		err = engine.BindGuardBV2(env, index, m.UnbindDelay, &m.Guard)
	case "guardUnbindInitiate":
		err = engine.GuardUnbindInitiate(env, index)
	case "guardUnbindCancel":
		err = engine.GuardUnbindCancel(env, &bucketguard.GuardCancel{BucketIndex: index, Generation: m.Call.Generation, RequestID: m.RequestID, Deadline: m.Call.Deadline, AnchorHeight: m.Call.AnchorHeight, AnchorHash: m.Call.AnchorHash, Sponsor: m.Call.Sponsor, Sig: m.Call.Sig})
	case "guardUnbindFinalize":
		err = engine.GuardUnbindFinalize(env, index)
	default:
		err = bucketguard.ErrMalformed
	}
	if err != nil {
		return nil, nil, vaultReceiptError(err)
	}
	if m.Call.Op == bucketguard.VaultOpRotateOwner && m.Method == "vaultRotateOwner" {
		newOwner, err := address.FromBytes(m.Call.NewOwner)
		if err != nil {
			return nil, nil, vaultReceiptError(bucketguard.ErrMalformed)
		}
		acct, err := accountutil.LoadAccount(sm, newOwner)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to load new vault owner %s", newOwner)
		}
		if acct.IsContract() {
			return nil, nil, vaultReceiptError(errors.Wrapf(bucketguard.ErrGuardScope, "vault owner cannot be contract %s", newOwner))
		}
	}
	if preflight {
		return nil, nil, nil
	}
	// Verify the entire transition before touching accounts or native records.
	next := engine.Vault(index)
	ng := engine.GuardV2(class, index)
	if class == bucketguard.ClassAVault {
		before := new(big.Int)
		if v != nil {
			before.SetBytes(v.Amount)
		}
		after := new(big.Int)
		if next != nil {
			after.SetBytes(next.Amount)
		}
		delta := new(big.Int).Sub(after, before)
		if delta.Sign() != 0 {
			from, to := ownerAddr, poolAddr
			n := new(big.Int).Set(delta)
			if delta.Sign() < 0 {
				n.Neg(n)
				from = poolAddr
				to = ownerAddr
				if m.Method == "vaultWithdraw" {
					to, err = address.FromBytes(m.Call.Recipient)
					if err != nil {
						return nil, nil, err
					}
				}
			}
			if err = vaultTransfer(sm, from, to, n); err != nil {
				return nil, nil, err
			}
			txlogs = append(txlogs, &action.TransactionLog{Type: iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, Sender: from.String(), Recipient: to.String(), Amount: new(big.Int).Set(n)})
		}
		if next == nil {
			_, err = sm.DelState(append(vaultStateOptions(vaultKey(_vaultBucket, index)), protocol.ObjectOption(&VaultBucketRecord{}))...)
		} else {
			_, err = sm.PutState(&VaultBucketRecord{*next}, vaultStateOptions(vaultKey(_vaultBucket, index))...)
		}
		if err != nil {
			return nil, nil, err
		}
		if _, err = sm.PutState(&VaultMetaRecord{engine.TotalVaultCount()}, vaultStateOptions([]byte{_vaultMeta})...); err != nil {
			return nil, nil, err
		}
	} else {
		// The immutable Draft 4 record remains the native write firewall.
		if err = putBucketGuard(sm, index, &bucketguard.Guard{AlgID: ng.AlgID, PublicKey: ng.PublicKey, Generation: ng.Generation, Nonce: ng.GuardNonce, BoundHeight: ng.BoundHeight}); err != nil {
			return nil, nil, err
		}
	}
	prefix := byte(_vaultGuard)
	if class == bucketguard.ClassBStaking {
		prefix = _stakingGuardV2
	}
	if _, err = sm.PutState(&BucketGuardV2Record{*ng}, vaultStateOptions(vaultKey(prefix, index))...); err != nil {
		return nil, nil, err
	}
	log = newReceiptLog(p.addr.String())
	event := bucketguard.Draft5Events.Events["VaultTransition"]
	data, err := event.Inputs.NonIndexed().Pack(m.Method, index, class, ng.Generation, ng.GuardNonce, ng.NextRequestID)
	if err != nil {
		return nil, nil, err
	}
	log.AddEvent(action.Topics{hash.BytesToHash256(event.ID.Bytes())}, data)
	return log, txlogs, nil
}

func vaultTransfer(sm protocol.StateManager, from, to address.Address, n *big.Int) error {
	if address.Equal(from, to) {
		return nil
	}
	a, err := accountutil.LoadAccount(sm, from)
	if err != nil {
		return err
	}
	a = a.Clone()
	b, err := accountutil.LoadAccount(sm, to)
	if err != nil {
		return err
	}
	b = b.Clone()
	if err = a.SubBalance(n); err != nil {
		return vaultReceiptError(bucketguard.ErrInsufficientBalance)
	}
	if err = b.AddBalance(n); err != nil {
		return err
	}
	if err = accountutil.StoreAccount(sm, from, a); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, to, b)
}

// vaultHandleError preserves error identity while carrying the existing
// protobuf failure status. No new receipt status allocation is required.
type vaultHandleError struct{ *handleError }

func (e *vaultHandleError) Unwrap() error { return e.err }

func vaultReceiptError(err error) error {
	if mapped := guardReceiptError(err); mapped != err {
		return mapped
	}
	for _, target := range []error{bucketguard.ErrClassMismatch, bucketguard.ErrVaultUnbinding, bucketguard.ErrVaultClosed, bucketguard.ErrAlreadyUnbinding, bucketguard.ErrUnbindNotElapsed, bucketguard.ErrUnbindElapsed, bucketguard.ErrCancelCooldown, bucketguard.ErrStaleRequestId, bucketguard.ErrFutureRequestId, bucketguard.ErrBelowMinBalance, bucketguard.ErrUnauthorizedOperator, bucketguard.ErrInsufficientBalance} {
		if errors.Is(err, target) {
			return &vaultHandleError{&handleError{err: err, failureStatus: iotextypes.ReceiptStatus_Failure}}
		}
	}
	return err
}

// readVaultState serves both ReadState and eth_call using ABI-packed data;
// existing protobuf method numbers and schemas remain immutable.
func (p *Protocol) readVaultState(ctx context.Context, sr protocol.StateReader, method string, args ...[]byte) ([]byte, uint64, error) {
	height, err := sr.Height()
	if err != nil {
		return nil, 0, err
	}
	if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
		return nil, height, errors.New("bucket guard disabled")
	}
	if len(args) != 1 || len(args[0]) != 32 {
		return nil, height, bucketguard.ErrMalformed
	}
	n := new(big.Int).SetBytes(args[0])
	if !n.IsUint64() {
		return nil, height, bucketguard.ErrMalformed
	}
	index := n.Uint64()
	bytesType, _ := abi.NewType("bytes", "", nil)
	if method == "stakingGuardV2" {
		g, e := ReadStakingGuardV2(sr, index)
		var raw []byte
		if e == nil {
			raw, e = g.Serialize()
		} else if errors.Cause(e) == state.ErrStateNotExist {
			e = nil
		}
		if e != nil {
			return nil, height, e
		}
		b, e := (abi.Arguments{{Type: bytesType}}).Pack(raw)
		return b, height, e
	}
	var owner common.Address
	amount := new(big.Int)
	var raw []byte
	v, e := ReadVaultBucket(sr, index)
	if e == nil {
		owner = common.BytesToAddress(v.Owner)
		amount.SetBytes(v.Amount)
	} else if errors.Cause(e) != state.ErrStateNotExist {
		return nil, height, e
	}
	g, e := ReadVaultGuard(sr, index)
	if e == nil {
		raw, e = g.Serialize()
	} else if errors.Cause(e) == state.ErrStateNotExist {
		e = nil
	}
	if e != nil {
		return nil, height, e
	}
	addressType, _ := abi.NewType("address", "", nil)
	uintType, _ := abi.NewType("uint256", "", nil)
	b, e := (abi.Arguments{{Type: addressType}, {Type: uintType}, {Type: bytesType}}).Pack(owner, amount, raw)
	return b, height, e
}
