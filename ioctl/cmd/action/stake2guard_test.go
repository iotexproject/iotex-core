// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func testGuardedOp() *bucketguard.GuardedBucketOp {
	alg := pqcrypto.AlgSLHDSA192s
	return &bucketguard.GuardedBucketOp{
		BucketIndex: 7, Generation: 3, Nonce: 5, Deadline: 1100, Op: bucketguard.OpTransfer,
		NewOwner: identityset.Address(3).Bytes(),
		NewGuard: &bucketguard.NewGuard{
			AlgID: alg, PublicKey: bytes.Repeat([]byte{1}, alg.PublicKeySize()),
			AcceptSig: bytes.Repeat([]byte{2}, alg.SignatureSize()),
		},
		Sig: bytes.Repeat([]byte{3}, alg.SignatureSize()),
	}
}

func newGasCmd(t *testing.T, args ...string) *cobra.Command {
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	RegisterWriteCommand(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func TestGuardGasLimit(t *testing.T) {
	r := require.New(t)
	a := action.NewGuardedBucketOp(testGuardedOp())
	intrinsic, err := a.IntrinsicGas()
	r.NoError(err)

	got, err := guardGasLimit(newGasCmd(t, "-l", "123456"), a)
	r.NoError(err)
	r.EqualValues(123456, got)
	// Without -l the 20M default of the flag is not used.
	got, err = guardGasLimit(newGasCmd(t), a)
	r.NoError(err)
	r.Equal(intrinsic+_guardGasMargin, got)

	b := action.NewBindBucketGuard(&bucketguard.BindBucketGuard{
		BucketIndex: 1, AlgID: pqcrypto.AlgSLHDSA192s,
		PublicKey: make([]byte, 48), AcceptSig: make([]byte, pqcrypto.AlgSLHDSA192s.SignatureSize()),
	})
	bi, _ := b.IntrinsicGas()
	got, err = guardGasLimit(newGasCmd(t), b)
	r.NoError(err)
	r.Equal(bi+_guardGasMargin, got)
}

func TestGuardDeadline(t *testing.T) {
	r := require.New(t)
	newCmd := func(args ...string) *cobra.Command {
		cmd := &cobra.Command{Use: "x"}
		cmd.Flags().Uint64Var(&_guardDeadlineBlocks, "deadline-blocks", _defaultDeadlineBlocks, "")
		cmd.Flags().Uint64Var(&_guardDeadline, "deadline", 0, "")
		r.NoError(cmd.ParseFlags(args))
		return cmd
	}
	d, err := guardDeadline(newCmd(), 1000, false)
	r.NoError(err)
	r.EqualValues(1100, d)
	d, err = guardDeadline(newCmd(), 1000, true)
	r.NoError(err)
	r.EqualValues(1720, d)
	d, err = guardDeadline(newCmd("--deadline-blocks", "5"), 1000, true)
	r.NoError(err)
	r.EqualValues(1005, d)
	d, err = guardDeadline(newCmd("--deadline", "2000"), 1000, false)
	r.NoError(err)
	r.EqualValues(2000, d)
	_, err = guardDeadline(newCmd("--deadline", "1000"), 1000, false)
	r.Error(err)
	_, err = guardDeadline(newCmd("--deadline", "2000", "--deadline-blocks", "5"), 1000, false)
	r.Error(err)
	_, err = guardDeadline(newCmd("--deadline-blocks", "0"), 1000, false)
	r.Error(err)
}

func TestPrintGuardActionProto(t *testing.T) {
	r := require.New(t)
	sk := identityset.PrivateKey(1)
	for name, payload := range map[string]interface {
		IntrinsicGas() (uint64, error)
		SanityCheck() error
		FillAction(*iotextypes.ActionCore)
	}{
		"op": action.NewGuardedBucketOp(testGuardedOp()),
		"bind": action.NewBindBucketGuard(&bucketguard.BindBucketGuard{
			BucketIndex: 9, AlgID: pqcrypto.AlgSLHDSA192s,
			PublicKey: make([]byte, 48), AcceptSig: bytes.Repeat([]byte{4}, pqcrypto.AlgSLHDSA192s.SignatureSize()),
		}),
	} {
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(100000).SetChainID(2).SetAction(payload).Build()
		selp, err := action.Sign(elp, sk)
		r.NoError(err)
		out, err := printActionProto(selp.Proto())
		r.NoError(err, name)
		// The 16 KB signatures are summarized, not dumped.
		r.Less(len(out), 2500, name)
		r.Contains(out, "<16224 bytes, sha256 ", name)
		if name == "op" {
			for _, want := range []string{"guardedBucketOp: <", "op: 3 (TRANSFER)", "generation: 3", "nonce: 5",
				"deadline: 1100", "newOwner: " + identityset.Address(3).String(), "newGuard: <", "SLH-DSA-SHA2-192s"} {
				r.Contains(out, want)
			}
			r.Equal(2, strings.Count(out, "<16224 bytes"))
		} else {
			r.Contains(out, "bindBucketGuard: <")
			r.Contains(out, "bucketIndex: 9")
		}
	}
}

func TestMatchGuardStatuses(t *testing.T) {
	r := require.New(t)
	for s := 216; s <= 221; s++ {
		m := Match(strconv.Itoa(s), "status")
		r.True(strings.HasPrefix(m, "(Failure : "), "%d: %q", s, m)
	}
	r.Contains(Match("218", "status"), "mismatch")
	r.Contains(Match("219", "status"), "signature")
	r.Equal("", Match("222", "status"))
}

const testGuardMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

func testKeyring(t *testing.T, passphrase string) *qsb.Keyring {
	seed, err := qsb.Seed(testGuardMnemonic, passphrase)
	require.NoError(t, err)
	return qsb.NewKeyring(seed, 0)
}

// TestSignRequest signs a ROTATE prepared with a pending key the way the
// one-shot command does: ACCEPT by --new-key-index, then CALL by the key
// found in the scan window.
func TestSignRequest(t *testing.T) {
	r := require.New(t)
	kr := testKeyring(t, "")
	cur, err := kr.Key(2)
	r.NoError(err)
	s := &qsb.ChainState{
		ChainID: 2, Height: 1000, BucketIndex: 7, Owner: identityset.Address(1),
		Guard: &bucketguard.Guard{AlgID: pqcrypto.AlgSLHDSA192s, PublicKey: cur.PublicKey().Bytes(), Generation: 1},
	}
	req, err := qsb.NewOpRequest(s, &qsb.OpParams{Op: bucketguard.OpRotate, Deadline: 1100})
	r.NoError(err)
	r.Contains(nextSteps(req, "f.json"), "sign f.json --new-key-index N")

	// The current key is outside a window of 2.
	two, high := uint32(3), uint32(5000)
	err = signRequest(req, kr, &guardSignOptions{scanWindow: 2, newKeyIndex: &two, signCall: true})
	r.ErrorContains(err, "not among key indices 0..1")
	r.True(req.AcceptSigned())
	r.False(req.CallSigned())
	// A wrong explicit index is refused before signing.
	wrong := uint32(0)
	r.ErrorContains(signRequest(req, kr, &guardSignOptions{keyIndex: &wrong, signCall: true}), "is not the bucket's guard key")
	r.NoError(signRequest(req, kr, &guardSignOptions{scanWindow: 4, signCall: true}))
	r.True(req.Complete())
	r.NoError(qsb.Verify(req))
	r.Contains(nextSteps(req, "f.json"), "valid until block 1100")

	// New keys beyond the recovery window need --allow-high-index.
	r.ErrorContains(checkKeyIndex(high), "--allow-high-index")
	r.NoError(checkKeyIndex(1023))
}
