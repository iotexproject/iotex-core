// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

const (
	// _guardGasMargin is added to the intrinsic gas of a guard action when
	// -l is not given.
	_guardGasMargin = 10_000
	// _defaultOpScanWindow is the number of key indices searched for the
	// current guard key when --key-index is not given (~0.3 s on 10 cores).
	_defaultOpScanWindow = 64
	// default deadlines, in blocks after the read height
	_defaultDeadlineBlocks        = 100
	_defaultOfflineDeadlineBlocks = 720
)

// guardGasLimit returns -l if it was given, else the intrinsic gas of the
// action plus a margin. The flag's own default (20M) is far too high.
func guardGasLimit(cmd *cobra.Command, a interface{ IntrinsicGas() (uint64, error) }) (uint64, error) {
	if f := cmd.Flags().Lookup(_gasLimitFlag.Label()); f != nil && f.Changed {
		return _gasLimitFlag.Value().(uint64), nil
	}
	g, err := a.IntrinsicGas()
	if err != nil {
		return 0, err
	}
	return g + _guardGasMargin, nil
}

// guardFromProto converts an on-chain record; an empty record (no guard ever
// bound) has zero counters.
func guardFromProto(g *iotextypes.BucketGuard) *bucketguard.Guard {
	return &bucketguard.Guard{
		AlgID:       pqcrypto.AlgID(g.GetAlgID()),
		PublicKey:   bytes.Clone(g.GetPublicKey()),
		Generation:  g.GetGeneration(),
		Nonce:       g.GetNonce(),
		BoundHeight: g.GetBoundHeight(),
	}
}

// loadChainState reads the network identity and a bucket with its guard
// record, both at the tip of the same block, and that block's hash, the
// anchor of an op prepared from the state.
func loadChainState(index uint64) (*qsb.ChainState, error) {
	meta, err := bc.GetChainMeta()
	if err != nil {
		return nil, output.NewError(0, "failed to get chain meta", err)
	}
	gh, err := bc.GetGenesisHash()
	if err != nil {
		return nil, output.NewError(0, "failed to get the genesis hash", err)
	}
	for attempt := 0; ; attempt++ {
		g, gh1, err := bc.GetBucketGuard(index, 0)
		if err != nil {
			return nil, output.NewError(0, "failed to read the bucket guard", err)
		}
		b, gh2, err := bc.GetBucketByIndex(index, 0)
		if err != nil {
			return nil, output.NewError(0, "failed to read the bucket", err)
		}
		if gh1 != gh2 && attempt < 3 {
			continue // a block landed between the reads
		}
		owner, err := address.FromString(b.GetOwner())
		if err != nil {
			return nil, output.NewError(output.AddressError, "invalid bucket owner", err)
		}
		anchor, err := bc.GetBlockHash(gh2)
		if err != nil {
			return nil, output.NewError(0, fmt.Sprintf("failed to read the hash of block %d", gh2), err)
		}
		return &qsb.ChainState{
			ChainID:     meta.GetChainID(),
			GenesisHash: gh,
			Height:      gh2,
			HeightHash:  anchor,
			BucketIndex: index,
			Owner:       owner,
			Candidate:   b.GetCandidateAddress(),
			Amount:      b.GetStakedAmount(),
			Guard:       guardFromProto(g),
		}, nil
	}
}

// guardDeadline returns --deadline, or the read height plus
// --deadline-blocks (default 100, or 720 for a file to sign offline). The
// read height is the op's anchor, so the deadline may be at most
// MAX_DEADLINE_WINDOW blocks after it.
func guardDeadline(cmd *cobra.Command, height uint64, offline bool) (uint64, error) {
	abs, rel := cmd.Flags().Changed("deadline"), cmd.Flags().Changed("deadline-blocks")
	window := bucketguard.ParamsV1().MaxDeadlineWindow
	switch {
	case abs && rel:
		return 0, output.NewError(output.FlagError, "give --deadline or --deadline-blocks, not both", nil)
	case abs:
		if _guardDeadline <= height {
			return 0, output.NewError(output.FlagError, fmt.Sprintf("--deadline %d is not after the tip %d", _guardDeadline, height), nil)
		}
		if _guardDeadline-height > window {
			return 0, output.NewError(output.FlagError, fmt.Sprintf(
				"--deadline %d is more than %d blocks after the tip %d, the op's anchor", _guardDeadline, window, height), nil)
		}
		return _guardDeadline, nil
	}
	blocks := _guardDeadlineBlocks
	if !rel {
		blocks = _defaultDeadlineBlocks
		if offline {
			blocks = _defaultOfflineDeadlineBlocks
		}
	}
	if blocks == 0 || blocks > window {
		return 0, output.NewError(output.FlagError, fmt.Sprintf("--deadline-blocks must be in 1..%d", window), nil)
	}
	return height + blocks, nil
}

// checkKeyIndex refuses a new guard key beyond the default recovery window
// unless asked: a wallet restoring from the mnemonic would not find it.
func checkKeyIndex(i uint32) error {
	if i >= signer.DefaultQSBScanWindow && !_guardAllowHighIndex {
		return output.NewError(output.FlagError, fmt.Sprintf(
			"key index %d is beyond the default recovery window 0..%d; pass --allow-high-index if you will remember it",
			i, signer.DefaultQSBScanWindow-1), nil)
	}
	return nil
}

// guardSignOptions selects the keys signRequest uses.
type guardSignOptions struct {
	keyIndex      *uint32 // current guard key, or the bind key; nil = scan
	scanWindow    uint32
	newKeyIndex   *uint32 // incoming key of ROTATE/TRANSFER, from the same vault unless newVault is set
	newVault      string
	newVaultIndex uint32
	newVaultPass  string
	signCall      bool // sign CALL (sign, one-shot); false for accept
}

// signRequest signs what kr (and the new vault) can sign: ACCEPT by the
// incoming key when its index is given, then CALL by the current key.
// Every signature is verified before it is stored.
func signRequest(r *qsb.Request, kr *qsb.Keyring, o *guardSignOptions) error {
	if r.Kind == qsb.KindBind {
		return signAcceptWith(r, kr, o.keyIndex, o.scanWindow)
	}
	if r.NeedsAccept() && !r.AcceptSigned() && o.newKeyIndex != nil {
		nkr := kr
		if o.newVault != "" {
			var err error
			if nkr, err = unlockVault(o.newVault, o.newVaultPass, o.newVaultIndex); err != nil {
				return err
			}
			defer nkr.Zero()
		}
		if err := signAcceptWith(r, nkr, o.newKeyIndex, 0); err != nil {
			return err
		}
	}
	if !o.signCall || r.CallSigned() {
		return nil
	}
	if r.IncomingPending() {
		return output.NewError(output.InputError,
			"the new guard key is not chosen yet: pass --new-key-index, or let the new key's holder run accept first", nil)
	}
	cur, err := r.CurrentGuard()
	if err != nil {
		return output.NewError(output.ValidationError, "", err)
	}
	idx, err := keyIndexOf(kr, cur, o.keyIndex, o.scanWindow)
	if err != nil {
		return err
	}
	k, err := kr.Key(idx)
	if err != nil {
		return output.NewError(output.CryptoError, "", err)
	}
	start := time.Now()
	if err := qsb.SignCall(r, k); err != nil {
		return output.NewError(output.CryptoError, "failed to sign CALL", err)
	}
	output.PrintInfo(fmt.Sprintf("Signed CALL with guard key %d of vault index %d in %v",
		idx, kr.VaultIndex(), time.Since(start).Round(time.Millisecond)))
	return nil
}

// signAcceptWith signs ACCEPT with the key at index (or, if nil, the key of
// the request found by a scan of window indices).
func signAcceptWith(r *qsb.Request, kr *qsb.Keyring, index *uint32, window uint32) error {
	if r.AcceptSigned() {
		return nil
	}
	var idx uint32
	if index != nil {
		if err := checkKeyIndex(*index); err != nil {
			return err
		}
		idx = *index
	} else {
		pub, err := r.IncomingKey()
		if err != nil {
			return output.NewError(output.ValidationError, "", err)
		}
		if pub == nil {
			return output.NewError(output.InputError, "the new guard key is not chosen yet: pass its key index", nil)
		}
		if idx, err = keyIndexOf(kr, pub, nil, window); err != nil {
			return err
		}
	}
	k, err := kr.Key(idx)
	if err != nil {
		return output.NewError(output.CryptoError, "", err)
	}
	start := time.Now()
	if err := qsb.SignAccept(r, k); err != nil {
		return output.NewError(output.CryptoError, "failed to sign ACCEPT", err)
	}
	output.PrintInfo(fmt.Sprintf("Signed ACCEPT with guard key %d of vault index %d in %v",
		idx, kr.VaultIndex(), time.Since(start).Round(time.Millisecond)))
	return nil
}

// keyIndexOf returns *index after checking it derives pub, or scans
// 0..window-1 for pub.
func keyIndexOf(kr *qsb.Keyring, pub *pqcrypto.PublicKey, index *uint32, window uint32) (uint32, error) {
	if index != nil {
		k, err := kr.Key(*index)
		if err != nil {
			return 0, output.NewError(output.CryptoError, "", err)
		}
		if !k.PublicKey().Equal(pub) {
			return 0, output.NewError(output.ValidationError, fmt.Sprintf(
				"key %d of vault index %d is not the bucket's guard key", *index, kr.VaultIndex()), nil)
		}
		return *index, nil
	}
	if window == 0 {
		window = _defaultOpScanWindow
	}
	idx, ok, err := scanKeyring(kr, pub, 0, window)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, output.NewError(output.ValidationError, fmt.Sprintf(
			"the guard key is not among key indices 0..%d of vault index %d; pass --key-index, --scan-window "+
				"or --vault-index (ioctl stake2 guard vault find can search further)", window-1, kr.VaultIndex()), nil)
	}
	return idx, nil
}

// precheckRequest compares a request with the chain before gas is spent.
func precheckRequest(r *qsb.Request, sender string) error {
	s, err := loadChainState(r.Bucket.Index)
	if err != nil {
		return err
	}
	if d := s.Domain(); d != r.Domain() {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the request is for %s (chain %d) but the endpoint serves %s (chain %d)",
			qsb.KnownNetwork(r.Domain()), r.ChainID, qsb.KnownNetwork(d), s.ChainID), nil)
	}
	if !address.Equal(s.Owner, r.Bucket.Owner) {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"bucket %d is now owned by %s, the request is for %s", r.Bucket.Index, s.Owner, r.Bucket.Owner), nil)
	}
	if r.Kind == qsb.KindBind {
		switch {
		case s.Guard.IsGuarded():
			return output.NewError(output.ValidationError, fmt.Sprintf("bucket %d is already guarded", r.Bucket.Index), nil)
		case s.Guard.Generation != r.Guard.Generation:
			return output.NewError(output.ValidationError, fmt.Sprintf(
				"the guard generation is now %d, the request is for %d; prepare it again", s.Guard.Generation, r.Guard.Generation), nil)
		case !util.AliasIsHdwalletKey(sender) && sender != r.Bucket.Owner.String():
			return output.NewError(output.ValidationError, fmt.Sprintf(
				"a bind must be sent by the bucket owner %s (-s), not %s", r.Bucket.Owner, sender), nil)
		}
		return nil
	}
	g := s.Guard
	switch {
	case !g.IsGuarded():
		return output.NewError(output.ValidationError, fmt.Sprintf("bucket %d is not guarded", r.Bucket.Index), nil)
	case g.AlgID != r.Guard.AlgID || !bytes.Equal(g.PublicKey, r.Guard.PublicKey):
		return output.NewError(output.ValidationError, "the bucket's guard key has changed since the request was prepared", nil)
	case g.Generation != r.Guard.Generation || g.Nonce != r.Guard.Nonce:
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the guard is at generation/nonce %d/%d, the request is for %d/%d; it would fail with status 218",
			g.Generation, g.Nonce, r.Guard.Generation, r.Guard.Nonce), nil)
	case s.Height+1 > r.Op.Deadline:
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the deadline %d has passed (tip %d)", r.Op.Deadline, s.Height), nil)
	}
	if err := checkAnchor(r, s.Height, bc.GetBlockHash); err != nil {
		return err
	}
	if err := checkSponsor(r, sender); err != nil {
		return err
	}
	if r.Op.Op == bucketguard.OpChangeCandidate {
		id, err := candidateID(r.Op.CandidateName)
		if err != nil {
			return err
		}
		if !bytes.Equal(id.Bytes(), r.Op.ExpectedCandidateID) {
			return output.NewError(output.ValidationError, fmt.Sprintf(
				"candidate %q is now %s, not the signed one; it would fail with status 221", r.Op.CandidateName, id), nil)
		}
	}
	return nil
}

// checkAnchor checks that the request's anchor block is on the chain the
// endpoint serves: the block at anchorHeight must have hash anchorHash.
func checkAnchor(r *qsb.Request, tip uint64, blockHash func(uint64) ([32]byte, error)) error {
	op := r.Op
	if op.AnchorHeight > tip {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the anchor block %d is above the tip %d; is the endpoint on the request's chain?", op.AnchorHeight, tip), nil)
	}
	h, err := blockHash(op.AnchorHeight)
	if err != nil {
		return output.NewError(0, fmt.Sprintf("failed to read the hash of the anchor block %d", op.AnchorHeight), err)
	}
	if !bytes.Equal(h[:], op.AnchorHash) {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"block %d has hash %x on this chain, the request is anchored to %x; it would fail with status 218",
			op.AnchorHeight, h, op.AnchorHash), nil)
	}
	return nil
}

// checkSponsor checks that a sponsor-bound op is sent by its sponsor.
func checkSponsor(r *qsb.Request, sender string) error {
	sp := r.Sponsor()
	switch {
	case sp == nil:
		return nil
	case util.AliasIsHdwalletKey(sender):
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the op is bound to sponsor %s and an hdwallet signer cannot be checked against it; "+
				"send it from that account's alias or address (or --skip-checks)", sp), nil)
	case sender != sp.String():
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the op is bound to sponsor %s and must be sent by it (-s), not %s; it would fail with status 203", sp, sender), nil)
	}
	return nil
}

// candidateID resolves a candidate name to the identifier a guarded
// CHANGE_CANDIDATE commits to.
func candidateID(name string) (address.Address, error) {
	c, err := bc.GetCandidateByName(name)
	if err != nil {
		return nil, output.NewError(0, fmt.Sprintf("failed to read candidate %q", name), err)
	}
	id := c.GetId()
	if id == "" {
		id = c.GetOwnerAddress()
	}
	a, err := address.FromString(id)
	if err != nil {
		return nil, output.NewError(output.AddressError, fmt.Sprintf("candidate %q has no identifier", name), err)
	}
	return a, nil
}

// submitRequest sends a complete request from -s: the bound sponsor of an
// op, any account for an op without one, the owner for a bind. Unless
// skipChecks, the request is first compared with the chain.
func submitRequest(cmd *cobra.Command, r *qsb.Request, skipChecks bool) error {
	if !r.Complete() {
		var missing []string
		if r.Kind == qsb.KindOp && !r.CallSigned() {
			missing = append(missing, "CALL (run sign)")
		}
		if r.NeedsAccept() && !r.AcceptSigned() {
			missing = append(missing, "ACCEPT (run accept, or sign --new-key-index)")
		}
		return output.NewError(output.ValidationError, "the request is not fully signed: missing "+strings.Join(missing, ", "), nil)
	}
	if err := qsb.Verify(r); err != nil {
		return output.NewError(output.ValidationError, "the request does not verify", err)
	}
	payload, err := qsb.Payload(r)
	if err != nil {
		return output.NewError(output.ValidationError, "", err)
	}
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	if skipChecks {
		output.PrintInfo("Skipping the chain checks (--skip-checks)")
	} else if err := precheckRequest(r, sender); err != nil {
		return err
	}
	gasLimit, err := guardGasLimit(cmd, payload)
	if err != nil {
		return output.NewError(0, "failed to compute the gas limit", err)
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	resp, err := SendActionAndResponse(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(payload).Build(),
		sender)
	if err != nil || resp == nil {
		return err
	}
	if err := outputActionInfo(resp.ActionHash); err != nil {
		return err
	}
	if _waitFlag.Value() == true && output.Format == "" {
		return showGuard(r.Bucket.Index, 0)
	}
	return nil
}
