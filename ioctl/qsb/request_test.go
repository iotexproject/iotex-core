// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package qsb

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

const (
	mainnetGenesis = "b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b"
	testnetGenesis = "5e31a9f95ca4de82bd6f5ed9b465c6474bba27f1c5d31747393a555ce1f41607"
)

func genesis(t testing.TB, s string) [32]byte {
	var h [32]byte
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	copy(h[:], b)
	return h
}

// Test keys: vault A is the test mnemonic without a passphrase, vault B the
// same mnemonic with "TREZOR" (a different seed).
var testKeys struct {
	once       sync.Once
	a0, a1, b0 *signer.PrivateKey
	err        error
}

func keys(t testing.TB) (a0, a1, b0 *signer.PrivateKey) {
	testKeys.once.Do(func() {
		var sa, sb []byte
		if sa, testKeys.err = Seed(testMnemonic, ""); testKeys.err != nil {
			return
		}
		if sb, testKeys.err = Seed(testMnemonic, "TREZOR"); testKeys.err != nil {
			return
		}
		if testKeys.a0, testKeys.err = NewKeyring(sa, 0).Key(0); testKeys.err != nil {
			return
		}
		if testKeys.a1, testKeys.err = NewKeyring(sa, 0).Key(1); testKeys.err != nil {
			return
		}
		testKeys.b0, testKeys.err = NewKeyring(sb, 0).Key(0)
	})
	require.NoError(t, testKeys.err)
	return testKeys.a0, testKeys.a1, testKeys.b0
}

func testState(t testing.TB) *ChainState {
	a0, _, _ := keys(t)
	return &ChainState{
		ChainID:     2,
		GenesisHash: genesis(t, testnetGenesis),
		Height:      1000,
		HeightHash:  [32]byte{0xa1, 0x2b},
		BucketIndex: 7,
		Owner:       identityset.Address(1),
		Candidate:   identityset.Address(5).String(),
		Amount:      "1234500000000000000000",
		Guard: &bucketguard.Guard{
			AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: a0.PublicKey().Bytes(), Generation: 3, Nonce: 5, BoundHeight: 900,
		},
	}
}

func testParams(t testing.TB, o bucketguard.Op) *OpParams {
	_, a1, b0 := keys(t)
	p := &OpParams{Op: o, Deadline: 1100}
	switch o {
	case bucketguard.OpWithdraw:
		p.Recipient = identityset.Address(2)
	case bucketguard.OpTransfer:
		p.NewOwner, p.NewGuardKey = identityset.Address(3), b0.PublicKey()
	case bucketguard.OpChangeCandidate:
		p.CandidateName, p.ExpectedCandidateID = "bot4", identityset.Address(4)
	case bucketguard.OpRestake:
		p.Duration, p.AutoStake = 91, true
	case bucketguard.OpRotate:
		p.NewGuardKey = a1.PublicKey()
	}
	return p
}

func roundTrip(t testing.TB, r *Request) *Request {
	b, err := json.Marshal(r)
	require.NoError(t, err)
	var out Request
	require.NoError(t, json.Unmarshal(b, &out))
	return &out
}

func TestDomainVectors(t *testing.T) {
	r := require.New(t)
	main := bucketguard.NewDomain(1, genesis(t, mainnetGenesis))
	r.Equal("49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871", hex.EncodeToString(main[:]))
	r.Equal("mainnet", KnownNetwork(main))
	test := bucketguard.NewDomain(2, genesis(t, testnetGenesis))
	r.Equal("5f917f80a9bc0a0ed98df4b02f74ebe111f8289f4e467a92e48fb3a2ae9f997d", hex.EncodeToString(test[:]))
	r.Equal("testnet", KnownNetwork(test))
	r.Equal("unknown network", KnownNetwork(bucketguard.NewDomain(2, genesis(t, mainnetGenesis))))
}

func TestParseOp(t *testing.T) {
	r := require.New(t)
	for _, o := range []bucketguard.Op{bucketguard.OpUnstake, bucketguard.OpWithdraw, bucketguard.OpTransfer,
		bucketguard.OpChangeCandidate, bucketguard.OpRestake, bucketguard.OpRotate} {
		got, err := ParseOp(CommandName(o))
		r.NoError(err)
		r.Equal(o, got)
		got, err = ParseOp(o.String())
		r.NoError(err)
		r.Equal(o, got)
	}
	_, err := ParseOp("bind")
	r.Error(err)
}

func TestRequestRoundTrip(t *testing.T) {
	r := require.New(t)
	s := testState(t)
	cases := map[string]*OpParams{}
	for _, o := range []bucketguard.Op{bucketguard.OpUnstake, bucketguard.OpWithdraw, bucketguard.OpTransfer,
		bucketguard.OpChangeCandidate, bucketguard.OpRestake, bucketguard.OpRotate} {
		cases[o.String()] = testParams(t, o)
	}
	cases["TRANSFER clear"] = &OpParams{Op: bucketguard.OpTransfer, Deadline: 1100, NewOwner: identityset.Address(3), ClearGuard: true}
	cases["ROTATE pending"] = &OpParams{Op: bucketguard.OpRotate, Deadline: 1100}
	cases["UNSTAKE sponsored"] = &OpParams{Op: bucketguard.OpUnstake, Deadline: 1100, Sponsor: identityset.Address(9)}
	for name, p := range cases {
		req, err := NewOpRequest(s, p)
		r.NoError(err, name)
		out := roundTrip(t, req)
		r.Equal(req, out, name)
		r.Equal(bucketguard.EncodeCall(req.Domain(), req.Owner(), req.Op), bucketguard.EncodeCall(out.Domain(), out.Owner(), out.Op), name)
		// Anchored to the block the state was read at.
		r.EqualValues(1000, out.Op.AnchorHeight, name)
		r.Equal(s.HeightHash[:], out.Op.AnchorHash, name)
		if p.Sponsor != nil {
			r.Equal(p.Sponsor.Bytes(), out.Op.Sponsor, name)
			r.Equal(p.Sponsor.String(), out.Sponsor().String(), name)
		} else {
			r.Empty(out.Op.Sponsor, name)
			r.Nil(out.Sponsor(), name)
		}
		_, ok := out.CallHash()
		r.Equal(name != "ROTATE pending", ok, name)
	}

	// A file is written atomically with mode 0600 and read back.
	req, err := NewOpRequest(s, testParams(t, bucketguard.OpWithdraw))
	r.NoError(err)
	path := filepath.Join(t.TempDir(), "op.json")
	r.NoError(WriteRequestFile(path, req))
	fi, err := os.Stat(path)
	r.NoError(err)
	r.Equal(os.FileMode(0600), fi.Mode().Perm())
	b, err := os.ReadFile(path)
	r.NoError(err)
	for _, want := range []string{`"type": "iotex-qsb-request"`, `"version": 2`, `"index": "7"`, `"generation": "3"`, `"nonce": "5"`,
		`"deadline": "1100"`, `"anchorHeight": "1000"`, `"anchorHash": "a12b000000`, `"op": "WITHDRAW"`, `"chainID": 2`, `"recipient": "io1`} {
		r.Contains(string(b), want)
	}
	got, err := ReadRequest(path)
	r.NoError(err)
	r.Equal(req, got)

	// Bind, pending and with a key.
	s.Guard = &bucketguard.Guard{Generation: 4, Nonce: 6}
	for _, k := range []*pqcrypto.PublicKey{nil, keysPub(t)} {
		req, err := NewBindRequest(s, k)
		r.NoError(err)
		r.Equal(req, roundTrip(t, req))
	}
	s.Guard = nil
	req, err = NewBindRequest(s, nil)
	r.NoError(err)
	r.Equal(req, roundTrip(t, req))
}

func keysPub(t testing.TB) *pqcrypto.PublicKey {
	a0, _, _ := keys(t)
	return a0.PublicKey()
}

func TestRequestRejects(t *testing.T) {
	r := require.New(t)
	s := testState(t)
	req, err := NewOpRequest(s, testParams(t, bucketguard.OpTransfer))
	r.NoError(err)
	b, err := json.Marshal(req)
	r.NoError(err)
	good := string(b)
	var out Request
	r.NoError(json.Unmarshal(b, &out))

	h, _ := req.CallHash()
	d := req.Domain()
	for name, bad := range map[string]string{
		"unknown field": strings.Replace(good, `"kind":`, `"extra":1,"kind":`, 1),
		"zero sponsor":  strings.Replace(good, `"deadline":`, `"sponsor":"io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7","deadline":`, 1),
		"bad sponsor":   strings.Replace(good, `"deadline":`, `"sponsor":"0x01","deadline":`, 1),
		// The CALL covers the sponsor, so adding one changes the callHash.
		"added sponsor":   strings.Replace(good, `"deadline":`, `"sponsor":"`+identityset.Address(9).String()+`","deadline":`, 1),
		"anchor changed":  strings.Replace(good, `"anchorHeight":"1000"`, `"anchorHeight":"999"`, 1),
		"no anchor":       strings.Replace(good, `"anchorHeight":"1000"`, `"anchorHeight":"0"`, 1),
		"short anchor":    strings.Replace(good, `"anchorHash":"a12b`, `"anchorHash":"`, 1),
		"anchor>deadline": strings.Replace(good, `"anchorHeight":"1000"`, `"anchorHeight":"1101"`, 1),
		"bad callHash":    strings.Replace(good, hex.EncodeToString(h[:]), strings.Repeat("00", 32), 1),
		"bad domain":      strings.Replace(good, hex.EncodeToString(d[:]), strings.Repeat("00", 32), 1),
		"other chain":     strings.Replace(good, `"chainID":2`, `"chainID":1`, 1),
		"uppercase hex":   strings.Replace(good, hex.EncodeToString(h[:]), strings.ToUpper(hex.EncodeToString(h[:])), 1),
		"number uint64":   strings.Replace(good, `"index":"7"`, `"index":7`, 1),
		"leading zero":    strings.Replace(good, `"index":"7"`, `"index":"07"`, 1),
		"command op name": strings.Replace(good, `"op":"TRANSFER"`, `"op":"transfer"`, 1),
		"trailing data":   good + "{}",
		"wrong type":      strings.Replace(good, RequestType, "other", 1),
		"wrong version":   strings.Replace(good, `"version":2`, `"version":3`, 1),
		"bind kind":       strings.Replace(good, `"kind":"op"`, `"kind":"bind"`, 1),
		"missing owner":   strings.Replace(good, `"newOwner":"`+identityset.Address(3).String()+`",`, "", 1),
		"guard cleared":   strings.Replace(good, hex.EncodeToString(req.Guard.PublicKey), "", 1),
		"pending signed":  strings.Replace(strings.Replace(good, hex.EncodeToString(req.Op.NewGuard.PublicKey), "", 1), `"sig":""`, `"sig":"00"`, 1),
	} {
		r.NotEqual(good, bad, name)
		var out Request
		r.Error(json.Unmarshal([]byte(bad), &out), name)
	}
	// A candidate name is checked before a summary prints it.
	cc, err := NewOpRequest(s, testParams(t, bucketguard.OpChangeCandidate))
	r.NoError(err)
	cb, err := json.Marshal(cc)
	r.NoError(err)
	badName := strings.Replace(string(cb), `"candidateName":"`+cc.Op.CandidateName+`"`, `"candidateName":"Bad\u001bName"`, 1)
	r.NotEqual(string(cb), badName)
	r.ErrorContains(json.Unmarshal([]byte(badName), &out), "invalid candidate name")
	// Version 1 files predate the anchor; the message says so.
	var o2 Request
	err = json.Unmarshal([]byte(strings.Replace(good, `"version":2`, `"version":1`, 1)), &o2)
	r.ErrorContains(err, "version 1 files predate IIP-65 Draft 4")
	err = json.Unmarshal([]byte(strings.Replace(strings.Replace(good, `"version":2`, `"version":1`, 1), `"kind":`, `"extra":1,"kind":`, 1)), &o2)
	r.ErrorContains(err, "version 1 files predate IIP-65 Draft 4")

	// A bind of a guarded bucket, and an op on an unguarded one, are refused.
	_, err = NewBindRequest(s, nil)
	r.ErrorContains(err, "already guarded")
	s.Guard = &bucketguard.Guard{Generation: 1}
	_, err = NewOpRequest(s, testParams(t, bucketguard.OpUnstake))
	r.ErrorContains(err, "not guarded")
	// Fields an op must not carry are refused by the action's checks.
	s = testState(t)
	p := testParams(t, bucketguard.OpUnstake)
	p.Recipient = identityset.Address(2)
	_, err = NewOpRequest(s, p)
	r.ErrorIs(err, bucketguard.ErrMalformed)
	p = testParams(t, bucketguard.OpUnstake)
	p.Deadline = s.Height
	_, err = NewOpRequest(s, p)
	r.ErrorContains(err, "deadline")
	// The deadline may be at most MAX_DEADLINE_WINDOW after the anchor.
	p.Deadline = s.Height + bucketguard.ParamsV1().MaxDeadlineWindow
	_, err = NewOpRequest(s, p)
	r.NoError(err)
	p.Deadline++
	_, err = NewOpRequest(s, p)
	r.ErrorContains(err, "more than 241920 blocks after the anchor block 1000")
	p = testParams(t, bucketguard.OpUnstake)
	zero, err := address.FromBytes(make([]byte, 20))
	r.NoError(err)
	p.Sponsor = zero
	_, err = NewOpRequest(s, p)
	r.ErrorContains(err, "sponsor must not be the zero address")
}
