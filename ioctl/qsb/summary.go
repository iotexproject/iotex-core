// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
)

// BlockTime is the block interval used to estimate deadlines in wall-clock time.
const BlockTime = 2500 * time.Millisecond

// MaxSafeDeadlineBlocks is the deadline distance (about one day) beyond which
// the CLI warns: a signed op is a bearer authorization until its deadline.
const MaxSafeDeadlineBlocks = 17_280

var knownNetworks = map[string]string{
	// chain 1, genesis b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b
	"49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871": "mainnet",
	// chain 2, genesis 5e31a9f95ca4de82bd6f5ed9b465c6474bba27f1c5d31747393a555ce1f41607
	"5f917f80a9bc0a0ed98df4b02f74ebe111f8289f4e467a92e48fb3a2ae9f997d": "testnet",
}

// KnownNetwork returns "mainnet" or "testnet" for their domains and
// "unknown network" otherwise.
func KnownNetwork(d bucketguard.Domain) string {
	if n, ok := knownNetworks[hex.EncodeToString(d[:])]; ok {
		return n
	}
	return "unknown network"
}

// SummaryMessage is what a signer confirms, rendered from the request that
// builds the transcripts. It is also the JSON output of -o json.
type SummaryMessage struct {
	Network        string            `json:"network"`
	ChainID        uint32            `json:"chainID"`
	Domain         string            `json:"domain"`
	Kind           Kind              `json:"kind"`
	Op             string            `json:"op"`
	Bucket         uint64            `json:"bucket"`
	Owner          string            `json:"owner"`
	ResultingOwner string            `json:"resultingOwner"`
	Candidate      string            `json:"candidate,omitempty"`
	Amount         string            `json:"amount,omitempty"`
	ReadHeight     uint64            `json:"readHeight"`
	Generation     uint64            `json:"generation"`
	Nonce          uint64            `json:"nonce"`
	GuardKey       string            `json:"guardKey,omitempty"`
	AnchorHeight   uint64            `json:"anchorHeight,omitempty"`
	AnchorHash     string            `json:"anchorHash,omitempty"`
	Sponsor        string            `json:"sponsor,omitempty"`
	Deadline       uint64            `json:"deadline,omitempty"`
	DeadlineBlocks uint64            `json:"deadlineBlocks,omitempty"`
	Fields         map[string]string `json:"fields,omitempty"`
	IncomingKey    string            `json:"incomingKey,omitempty"`
	CallHash       string            `json:"callHash,omitempty"`
	Call           SigStatus         `json:"call"`
	Accept         SigStatus         `json:"accept"`
	Complete       bool              `json:"complete"`
	Warnings       []string          `json:"warnings,omitempty"`
	fieldOrder     []string
}

// Summary describes r and verifies its signatures.
func Summary(r *Request) *SummaryMessage {
	d := r.Domain()
	m := &SummaryMessage{
		Network:    KnownNetwork(d),
		ChainID:    r.ChainID,
		Domain:     hex.EncodeToString(d[:]),
		Kind:       r.Kind,
		Bucket:     r.Bucket.Index,
		Owner:      r.Bucket.Owner.String(),
		Candidate:  r.Bucket.Candidate,
		Amount:     r.Bucket.Amount,
		ReadHeight: r.Bucket.ReadHeight,
		Generation: r.Guard.Generation,
		Nonce:      r.Guard.Nonce,
		Complete:   r.Complete(),
	}
	m.ResultingOwner = m.Owner
	if o, err := r.ResultingOwner(); err == nil {
		m.ResultingOwner = o.String()
	}
	m.Call, m.Accept = Signatures(r)
	if r.NeedsAccept() {
		m.IncomingKey = "pending"
		if _, pub := r.incoming(); len(pub) > 0 {
			m.IncomingKey = hex.EncodeToString(pub)
		}
	}
	if r.Kind == KindBind {
		m.Op = "BIND"
		m.Warnings = append(m.Warnings,
			"binding is irreversible for the owner key: afterwards only the guard key can unstake, withdraw, transfer, "+
				"restake or change the candidate of this bucket; back up the vault mnemonic (and passphrase) first")
		return m
	}
	op := r.Op
	m.Op = op.Op.String()
	m.GuardKey = hex.EncodeToString(r.Guard.PublicKey)
	m.AnchorHeight = op.AnchorHeight
	m.AnchorHash = hex.EncodeToString(op.AnchorHash)
	if s := r.Sponsor(); s != nil {
		m.Sponsor = s.String()
	}
	m.Deadline = op.Deadline
	if op.Deadline > r.Bucket.ReadHeight {
		m.DeadlineBlocks = op.Deadline - r.Bucket.ReadHeight
	}
	if h, ok := r.CallHash(); ok {
		m.CallHash = hex.EncodeToString(h[:])
	}
	add := func(k, v string) {
		if m.Fields == nil {
			m.Fields = map[string]string{}
		}
		m.Fields[k] = v
		m.fieldOrder = append(m.fieldOrder, k)
	}
	addr := func(b []byte) string {
		a, err := address.FromBytes(b)
		if err != nil {
			return hex.EncodeToString(b)
		}
		return a.String()
	}
	switch op.Op {
	case bucketguard.OpWithdraw:
		add("recipient", addr(op.Recipient))
	case bucketguard.OpTransfer:
		add("newOwner", addr(op.NewOwner))
		if op.ClearGuard {
			add("clearGuard", "true")
			m.Warnings = append(m.Warnings, "clearGuard removes the guard: the new owner's ECDSA key alone will control the bucket")
		}
	case bucketguard.OpChangeCandidate:
		add("candidateName", op.CandidateName)
		add("expectedCandidateID", addr(op.ExpectedCandidateID))
	case bucketguard.OpRestake:
		add("duration", fmt.Sprintf("%d days", op.Duration))
		add("autoStake", fmt.Sprintf("%v", op.AutoStake))
	}
	if m.Sponsor == "" {
		m.Warnings = append(m.Warnings, fmt.Sprintf(
			"no sponsor: the signed op is a bearer authorization, anyone holding it can submit it until block %d", op.Deadline))
	}
	if m.DeadlineBlocks > MaxSafeDeadlineBlocks {
		m.Warnings = append(m.Warnings, fmt.Sprintf("the deadline is %d blocks (%s) after the read height",
			m.DeadlineBlocks, approx(m.DeadlineBlocks)))
	}
	return m
}

func approx(blocks uint64) string {
	return "≈ " + (time.Duration(blocks) * BlockTime).Round(time.Minute).String()
}

func shortHex(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:16] + "…" + s[len(s)-8:]
}

// FormatRau formats an amount in rau as IOTX.
func FormatRau(rau string) string {
	v, ok := new(big.Int).SetString(rau, 10)
	if !ok {
		return rau
	}
	q, r := new(big.Int).QuoRem(v, big.NewInt(1e18), new(big.Int))
	if r.Sign() == 0 {
		return q.String() + " IOTX"
	}
	frac := strings.TrimRight(fmt.Sprintf("%018s", r.String()), "0")
	return q.String() + "." + frac + " IOTX"
}

// String renders the summary for a terminal.
func (m *SummaryMessage) String() string {
	var b strings.Builder
	line := func(k, format string, a ...any) {
		fmt.Fprintf(&b, "  %-20s "+format+"\n", append([]any{k + ":"}, a...)...)
	}
	fmt.Fprintf(&b, "%s on bucket %d\n", m.Op, m.Bucket)
	line("network", "%s (chain %d, domain %s)", m.Network, m.ChainID, shortHex(m.Domain))
	line("owner", "%s", m.Owner)
	if m.ResultingOwner != m.Owner {
		line("resulting owner", "%s", m.ResultingOwner)
	}
	if m.Candidate != "" {
		line("candidate", "%s", m.Candidate)
	}
	if m.Amount != "" {
		line("amount", "%s (informational; the op does not commit to it)", FormatRau(m.Amount))
	}
	line("read at height", "%d", m.ReadHeight)
	if m.Kind == KindOp {
		line("guard key", "%s", m.GuardKey)
	}
	line("generation/nonce", "%d/%d", m.Generation, m.Nonce)
	for _, k := range m.fieldOrder {
		line(k, "%s", m.Fields[k])
	}
	if m.Kind == KindOp {
		line("anchor", "block %d, hash %s", m.AnchorHeight, m.AnchorHash)
		if m.Sponsor != "" {
			line("sponsor", "%s (the only account that can submit it)", m.Sponsor)
		} else {
			line("sponsor", "none (anyone holding the signed op can submit it)")
		}
	}
	if m.Deadline > 0 {
		line("deadline", "block %d (%d blocks after the read height, %s)", m.Deadline, m.DeadlineBlocks, approx(m.DeadlineBlocks))
	}
	if m.IncomingKey != "" {
		// in full: the holder compares it with the key it expects
		line("incoming key", "%s", m.IncomingKey)
	}
	if m.CallHash != "" {
		line("call hash", "%s", m.CallHash)
	}
	if m.Kind == KindOp {
		line("CALL", "%s", m.Call)
	}
	if m.Accept != SigNotNeeded {
		line("ACCEPT", "%s", m.Accept)
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(&b, "  warning: %s\n", w)
	}
	return strings.TrimRight(b.String(), "\n")
}
