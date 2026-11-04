// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Request file envelope. Version 2 (IIP-65 Draft 4) adds the anchor and the
// optional sponsor of an op; version 1 files predate the anchor and are
// rejected.
const (
	RequestType    = "iotex-qsb-request"
	RequestVersion = 2
)

// Kind is the kind of a request.
type Kind string

// Request kinds.
const (
	KindOp   Kind = "op"
	KindBind Kind = "bind"
)

// Request is an IIP-65 action being prepared, signed and submitted: a guarded
// op (CALL by the current guard key, plus ACCEPT by an incoming key) or a bind
// (ACCEPT by the new key). The same struct builds the transcripts and the
// summary a signer confirms.
//
// An op request without a sponsor is a bearer authorization: anyone holding
// the complete request can submit it until its deadline. With a sponsor, only
// that account can.
type Request struct {
	Kind        Kind
	ChainID     uint32
	GenesisHash [32]byte
	Bucket      BucketInfo
	// Guard is the bucket's guard record at Bucket.ReadHeight: the current
	// key and counters of an op; the counters (tombstone or none) of a bind.
	Guard bucketguard.Guard
	// Op is set for KindOp. BucketIndex, Generation and Nonce mirror Bucket
	// and Guard; AnchorHeight and AnchorHash name the block the request was
	// prepared at. A NewGuard with an empty PublicKey is pending: the
	// incoming key is not chosen yet, so CALL cannot be signed.
	Op *bucketguard.GuardedBucketOp
	// Bind is set for KindBind. An empty PublicKey is pending.
	Bind *bucketguard.BindBucketGuard
}

// BucketInfo describes the bucket. Owner is signed (CALL, ACCEPT); the other
// fields are informational.
type BucketInfo struct {
	Index      uint64
	Owner      address.Address
	Candidate  string
	Amount     string // rau
	ReadHeight uint64
}

// Domain returns the transcript domain of r's network.
func (r *Request) Domain() bucketguard.Domain {
	return bucketguard.NewDomain(r.ChainID, r.GenesisHash)
}

// Owner returns the bucket owner CALL is signed for.
func (r *Request) Owner() address.Address { return r.Bucket.Owner }

// ResultingOwner returns the bucket owner after the action, the owner an
// incoming key accepts.
func (r *Request) ResultingOwner() (address.Address, error) {
	if r.Kind == KindOp && r.Op.Op == bucketguard.OpTransfer {
		return address.FromBytes(r.Op.NewOwner)
	}
	return r.Bucket.Owner, nil
}

// CurrentGuard returns the current guard key of an op request.
func (r *Request) CurrentGuard() (*pqcrypto.PublicKey, error) {
	return pqcrypto.ParsePublicKey(r.Guard.AlgID, r.Guard.PublicKey)
}

// IncomingKey returns the key a bind installs or an op's NewGuard; nil if the
// request installs none or the key is pending.
func (r *Request) IncomingKey() (*pqcrypto.PublicKey, error) {
	alg, pub := r.incoming()
	if len(pub) == 0 {
		return nil, nil
	}
	return pqcrypto.ParsePublicKey(alg, pub)
}

func (r *Request) incoming() (pqcrypto.AlgID, []byte) {
	switch {
	case r.Kind == KindBind:
		return r.Bind.AlgID, r.Bind.PublicKey
	case r.Op.NewGuard != nil:
		return r.Op.NewGuard.AlgID, r.Op.NewGuard.PublicKey
	}
	return 0, nil
}

func (r *Request) acceptSig() []byte {
	switch {
	case r.Kind == KindBind:
		return r.Bind.AcceptSig
	case r.Op.NewGuard != nil:
		return r.Op.NewGuard.AcceptSig
	}
	return nil
}

// NeedsAccept reports whether the request carries an ACCEPT.
func (r *Request) NeedsAccept() bool {
	return r.Kind == KindBind || r.Op.NewGuard != nil
}

// IncomingPending reports whether the incoming key is still to be chosen.
func (r *Request) IncomingPending() bool {
	_, pub := r.incoming()
	return r.NeedsAccept() && len(pub) == 0
}

// CallSigned reports whether CALL is signed (ops only).
func (r *Request) CallSigned() bool { return r.Kind == KindOp && len(r.Op.Sig) > 0 }

// AcceptSigned reports whether ACCEPT is signed.
func (r *Request) AcceptSigned() bool { return len(r.acceptSig()) > 0 }

// Complete reports whether every signature the action needs is present.
func (r *Request) Complete() bool {
	if r.NeedsAccept() && !r.AcceptSigned() {
		return false
	}
	return r.Kind == KindBind || r.CallSigned()
}

// CallHash returns SHA-512 of the CALL transcript of an op. ok is false for a
// bind, or while the incoming key is pending.
func (r *Request) CallHash() (h [bucketguard.CallHashLen]byte, ok bool) {
	if r.Kind != KindOp || r.IncomingPending() {
		return h, false
	}
	return bucketguard.CallHash(r.Domain(), r.Bucket.Owner, r.Op), true
}

// GuardedOp returns the op.
func (r *Request) GuardedOp() *bucketguard.GuardedBucketOp { return r.Op }

// Sponsor returns the only account that may submit an op request, or nil if
// anyone may.
func (r *Request) Sponsor() address.Address {
	if r.Kind != KindOp || len(r.Op.Sponsor) == 0 {
		return nil
	}
	a, err := address.FromBytes(r.Op.Sponsor)
	if err != nil {
		return nil
	}
	return a
}

// BindOp returns the bind.
func (r *Request) BindOp() *bucketguard.BindBucketGuard { return r.Bind }

// ---- op names ----

var opNames = map[string]bucketguard.Op{
	"unstake":          bucketguard.OpUnstake,
	"withdraw":         bucketguard.OpWithdraw,
	"transfer":         bucketguard.OpTransfer,
	"change-candidate": bucketguard.OpChangeCandidate,
	"restake":          bucketguard.OpRestake,
	"rotate":           bucketguard.OpRotate,
}

// ParseOp parses an op by its command name (change-candidate) or IIP name
// (CHANGE_CANDIDATE).
func ParseOp(s string) (bucketguard.Op, error) {
	if o, ok := opNames[strings.ToLower(strings.ReplaceAll(s, "_", "-"))]; ok {
		return o, nil
	}
	return 0, fmt.Errorf("unknown guarded op %q", s)
}

// CommandName returns the command name of op (change-candidate).
func CommandName(o bucketguard.Op) string {
	for n, v := range opNames {
		if v == o {
			return n
		}
	}
	return o.String()
}

// ---- JSON ----

// u64 is a uint64 written as a decimal string, so JSON readers without 64-bit
// integers keep the exact value.
type u64 uint64

func (v u64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(v), 10))
}

func (v *u64) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("uint64 must be a decimal string: %w", err)
	}
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return fmt.Errorf("uint64 %q is not canonical", s)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*v = u64(n)
	return nil
}

type wireRequest struct {
	Type     string      `json:"type"`
	Version  int         `json:"version"`
	Kind     Kind        `json:"kind"`
	Network  wireNetwork `json:"network"`
	Bucket   wireBucket  `json:"bucket"`
	Guard    wireGuard   `json:"guard"`
	Op       *wireOp     `json:"op,omitempty"`
	Bind     *wireKey    `json:"bind,omitempty"`
	CallHash string      `json:"callHash,omitempty"`
}

type wireNetwork struct {
	ChainID     uint32 `json:"chainID"`
	GenesisHash string `json:"genesisHash"`
	Domain      string `json:"domain"`
}

type wireBucket struct {
	Index      u64    `json:"index"`
	Owner      string `json:"owner"`
	Candidate  string `json:"candidate,omitempty"`
	Amount     string `json:"amount,omitempty"`
	ReadHeight u64    `json:"readHeight"`
}

type wireGuard struct {
	AlgID      uint32 `json:"algID"`
	PublicKey  string `json:"publicKey"`
	Generation u64    `json:"generation"`
	Nonce      u64    `json:"nonce"`
}

type wireKey struct {
	AlgID     uint32 `json:"algID"`
	PublicKey string `json:"publicKey"`
	AcceptSig string `json:"acceptSig"`
}

type wireOp struct {
	Op                  string   `json:"op"`
	Deadline            u64      `json:"deadline"`
	AnchorHeight        u64      `json:"anchorHeight"`
	AnchorHash          string   `json:"anchorHash"`
	Sponsor             string   `json:"sponsor,omitempty"`
	Recipient           string   `json:"recipient,omitempty"`
	NewOwner            string   `json:"newOwner,omitempty"`
	CandidateName       string   `json:"candidateName,omitempty"`
	ExpectedCandidateID string   `json:"expectedCandidateID,omitempty"`
	Duration            uint32   `json:"duration,omitempty"`
	AutoStake           bool     `json:"autoStake,omitempty"`
	NewGuard            *wireKey `json:"newGuard,omitempty"`
	ClearGuard          bool     `json:"clearGuard,omitempty"`
	Sig                 string   `json:"sig"`
}

func encodeAddr(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	a, err := address.FromBytes(b)
	if err != nil {
		return "", err
	}
	return a.String(), nil
}

func decodeAddr(field, s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	a, err := address.FromString(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return a.Bytes(), nil
}

func decodeHex(field, s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if s != strings.ToLower(s) {
		return nil, fmt.Errorf("%s must be lowercase hex", field)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return b, nil
}

// MarshalJSON writes request file v2.
func (r *Request) MarshalJSON() ([]byte, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	d := r.Domain()
	w := wireRequest{
		Type:    RequestType,
		Version: RequestVersion,
		Kind:    r.Kind,
		Network: wireNetwork{
			ChainID:     r.ChainID,
			GenesisHash: hex.EncodeToString(r.GenesisHash[:]),
			Domain:      hex.EncodeToString(d[:]),
		},
		Bucket: wireBucket{
			Index:      u64(r.Bucket.Index),
			Owner:      r.Bucket.Owner.String(),
			Candidate:  r.Bucket.Candidate,
			Amount:     r.Bucket.Amount,
			ReadHeight: u64(r.Bucket.ReadHeight),
		},
		Guard: wireGuard{
			AlgID:      uint32(r.Guard.AlgID),
			PublicKey:  hex.EncodeToString(r.Guard.PublicKey),
			Generation: u64(r.Guard.Generation),
			Nonce:      u64(r.Guard.Nonce),
		},
	}
	switch r.Kind {
	case KindBind:
		w.Bind = &wireKey{
			AlgID:     uint32(r.Bind.AlgID),
			PublicKey: hex.EncodeToString(r.Bind.PublicKey),
			AcceptSig: hex.EncodeToString(r.Bind.AcceptSig),
		}
	case KindOp:
		op := r.Op
		wo := &wireOp{
			Op:            op.Op.String(),
			Deadline:      u64(op.Deadline),
			AnchorHeight:  u64(op.AnchorHeight),
			AnchorHash:    hex.EncodeToString(op.AnchorHash),
			CandidateName: op.CandidateName,
			Duration:      op.Duration,
			AutoStake:     op.AutoStake,
			ClearGuard:    op.ClearGuard,
			Sig:           hex.EncodeToString(op.Sig),
		}
		var err error
		if wo.Recipient, err = encodeAddr(op.Recipient); err != nil {
			return nil, err
		}
		if wo.NewOwner, err = encodeAddr(op.NewOwner); err != nil {
			return nil, err
		}
		if wo.ExpectedCandidateID, err = encodeAddr(op.ExpectedCandidateID); err != nil {
			return nil, err
		}
		if wo.Sponsor, err = encodeAddr(op.Sponsor); err != nil {
			return nil, err
		}
		if ng := op.NewGuard; ng != nil {
			wo.NewGuard = &wireKey{
				AlgID:     uint32(ng.AlgID),
				PublicKey: hex.EncodeToString(ng.PublicKey),
				AcceptSig: hex.EncodeToString(ng.AcceptSig),
			}
		}
		w.Op = wo
		if h, ok := r.CallHash(); ok {
			w.CallHash = hex.EncodeToString(h[:])
		}
	}
	return json.MarshalIndent(w, "", "  ")
}

// UnmarshalJSON reads request file v2 strictly: other versions, unknown
// fields, a domain that does not match the chain ID and genesis hash, and a
// callHash that does not match the CALL transcript are rejected.
func (r *Request) UnmarshalJSON(b []byte) error {
	// The envelope first, so a file of another version gets a clear message
	// rather than an unknown-field error.
	var env struct {
		Type    string `json:"type"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	switch {
	case env.Type != RequestType:
		return fmt.Errorf("request: unsupported type %q", env.Type)
	case env.Version == 1:
		return errors.New("request: version 1 files predate IIP-65 Draft 4 and carry no anchor block; prepare the request again")
	case env.Version != RequestVersion:
		return fmt.Errorf("request: unsupported version %d (this ioctl reads version %d)", env.Version, RequestVersion)
	}
	var w wireRequest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("request: trailing data after the JSON object")
	}
	var out Request
	out.Kind = w.Kind
	out.ChainID = w.Network.ChainID
	gh, err := decodeHex("network.genesisHash", w.Network.GenesisHash)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if len(gh) != 32 {
		return errors.New("request: network.genesisHash must be 32 bytes")
	}
	copy(out.GenesisHash[:], gh)
	d := out.Domain()
	if w.Network.Domain != hex.EncodeToString(d[:]) {
		return errors.New("request: network.domain does not match chainID and genesisHash")
	}
	owner, err := address.FromString(w.Bucket.Owner)
	if err != nil {
		return fmt.Errorf("request: bucket.owner: %w", err)
	}
	out.Bucket = BucketInfo{
		Index:      uint64(w.Bucket.Index),
		Owner:      owner,
		Candidate:  w.Bucket.Candidate,
		Amount:     w.Bucket.Amount,
		ReadHeight: uint64(w.Bucket.ReadHeight),
	}
	gpub, err := decodeHex("guard.publicKey", w.Guard.PublicKey)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	out.Guard = bucketguard.Guard{
		AlgID:      pqcrypto.AlgID(w.Guard.AlgID),
		PublicKey:  gpub,
		Generation: uint64(w.Guard.Generation),
		Nonce:      uint64(w.Guard.Nonce),
	}
	switch w.Kind {
	case KindBind:
		if w.Bind == nil || w.Op != nil || w.CallHash != "" {
			return errors.New("request: a bind carries bind and no op or callHash")
		}
		k, err := decodeKey("bind", w.Bind)
		if err != nil {
			return err
		}
		out.Bind = &bucketguard.BindBucketGuard{
			BucketIndex: out.Bucket.Index, AlgID: k.AlgID, PublicKey: k.PublicKey, AcceptSig: k.AcceptSig,
		}
	case KindOp:
		if w.Op == nil || w.Bind != nil {
			return errors.New("request: an op carries op and no bind")
		}
		if out.Op, err = decodeOp(&w, out.Bucket.Index, out.Guard); err != nil {
			return err
		}
	default:
		return fmt.Errorf("request: unknown kind %q", w.Kind)
	}
	if err := out.check(); err != nil {
		return err
	}
	h, ok := out.CallHash()
	switch {
	case ok && w.CallHash != hex.EncodeToString(h[:]):
		return errors.New("request: callHash does not match the CALL transcript")
	case !ok && w.CallHash != "":
		return errors.New("request: callHash given while the incoming key is pending")
	}
	*r = out
	return nil
}

func decodeKey(field string, k *wireKey) (*bucketguard.NewGuard, error) {
	pub, err := decodeHex(field+".publicKey", k.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	sig, err := decodeHex(field+".acceptSig", k.AcceptSig)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return &bucketguard.NewGuard{AlgID: pqcrypto.AlgID(k.AlgID), PublicKey: pub, AcceptSig: sig}, nil
}

func decodeOp(w *wireRequest, index uint64, g bucketguard.Guard) (*bucketguard.GuardedBucketOp, error) {
	wo := w.Op
	o, err := ParseOp(wo.Op)
	if err != nil || wo.Op != o.String() {
		return nil, fmt.Errorf("request: unknown op %q", wo.Op)
	}
	op := &bucketguard.GuardedBucketOp{
		BucketIndex:   index,
		Generation:    g.Generation,
		Nonce:         g.Nonce,
		Deadline:      uint64(wo.Deadline),
		AnchorHeight:  uint64(wo.AnchorHeight),
		Op:            o,
		CandidateName: wo.CandidateName,
		Duration:      wo.Duration,
		AutoStake:     wo.AutoStake,
		ClearGuard:    wo.ClearGuard,
	}
	if op.Recipient, err = decodeAddr("op.recipient", wo.Recipient); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if op.NewOwner, err = decodeAddr("op.newOwner", wo.NewOwner); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if op.ExpectedCandidateID, err = decodeAddr("op.expectedCandidateID", wo.ExpectedCandidateID); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if op.Sponsor, err = decodeAddr("op.sponsor", wo.Sponsor); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if op.AnchorHash, err = decodeHex("op.anchorHash", wo.AnchorHash); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if op.Sig, err = decodeHex("op.sig", wo.Sig); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if wo.NewGuard != nil {
		if op.NewGuard, err = decodeKey("op.newGuard", wo.NewGuard); err != nil {
			return nil, err
		}
	}
	return op, nil
}

// check validates the shape of r. Missing signatures are allowed; with
// stand-ins of the right size, the request must pass the stateless checks of
// the action (bucketguard Validate with ParamsV1).
func (r *Request) check() error {
	p := bucketguard.ParamsV1()
	if r.Bucket.Owner == nil {
		return errors.New("request: missing bucket owner")
	}
	switch r.Kind {
	case KindBind:
		if r.Bind == nil || r.Op != nil {
			return errors.New("request: a bind carries bind and no op")
		}
		if r.Guard.IsGuarded() || r.Guard.AlgID != 0 {
			return errors.New("request: bind of a bucket that is already guarded")
		}
		if r.Bind.BucketIndex != r.Bucket.Index {
			return errors.New("request: bind bucket index mismatch")
		}
		m := *r.Bind
		if len(m.PublicKey) == 0 {
			m.PublicKey = make([]byte, m.AlgID.PublicKeySize())
		}
		if len(m.AcceptSig) == 0 {
			m.AcceptSig = make([]byte, m.AlgID.SignatureSize())
		}
		if err := m.Validate(p); err != nil {
			return fmt.Errorf("request: %w", err)
		}
	case KindOp:
		if r.Op == nil || r.Bind != nil {
			return errors.New("request: an op carries op and no bind")
		}
		if _, err := r.CurrentGuard(); err != nil {
			return fmt.Errorf("request: guard: %w", err)
		}
		op := *r.Op
		if op.BucketIndex != r.Bucket.Index || op.Generation != r.Guard.Generation || op.Nonce != r.Guard.Nonce {
			return errors.New("request: op index, generation or nonce mismatch")
		}
		if len(op.Sig) == 0 {
			op.Sig = make([]byte, r.Guard.AlgID.SignatureSize())
		} else if len(op.Sig) != r.Guard.AlgID.SignatureSize() {
			return fmt.Errorf("request: CALL signature has %d bytes, %s needs %d", len(op.Sig), r.Guard.AlgID, r.Guard.AlgID.SignatureSize())
		}
		if ng := op.NewGuard; ng != nil {
			if len(ng.PublicKey) == 0 && len(r.Op.Sig) > 0 {
				return errors.New("request: CALL is signed but the incoming key is pending")
			}
			if len(ng.PublicKey) == 0 && len(ng.AcceptSig) > 0 {
				return errors.New("request: ACCEPT is signed but the incoming key is pending")
			}
			c := *ng
			if len(c.PublicKey) == 0 {
				c.PublicKey = make([]byte, c.AlgID.PublicKeySize())
			}
			if len(c.AcceptSig) == 0 {
				c.AcceptSig = make([]byte, c.AlgID.SignatureSize())
			}
			op.NewGuard = &c
		}
		if err := op.Validate(p); err != nil {
			return fmt.Errorf("request: %w", err)
		}
	default:
		return fmt.Errorf("request: unknown kind %q", r.Kind)
	}
	if _, pub := r.incoming(); len(pub) > 0 {
		if _, err := r.IncomingKey(); err != nil {
			return fmt.Errorf("request: incoming key: %w", err)
		}
	}
	return nil
}

// ReadRequest reads and validates a request file.
func ReadRequest(path string) (*Request, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var r Request
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}

// WriteRequestFile writes r to path atomically with mode 0600.
func WriteRequestFile(path string, r *Request) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	// json.Marshal compacts the indented output of MarshalJSON; re-indent.
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	return WriteFileAtomic(path, buf.Bytes())
}

// WriteFileAtomic writes data to a temporary file next to path with mode
// 0600 and renames it over path.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
