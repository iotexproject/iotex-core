// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// blsPoPFlags holds the user's BLS proof-of-possession choices for
// stake2 register / update. The three-option matrix:
//
//   Option 1 (default, register only): all flags empty.
//     Tool derives a BLS keypair deterministically from the signer's
//     ECDSA private key (same scheme as `ioctl account blskey`),
//     computes the PoP, and prompts the user to confirm before
//     proceeding. --yes suppresses the prompt for scripted use.
//
//   Option 2: --bls-priv-key (hex) supplied.
//     Tool uses the provided BLS key, derives its pubkey, computes
//     the PoP. No prompt.
//
//   Option 3: --bls-pubkey and --bls-pop both supplied.
//     Tool sends as-is. If --candidate-id is also supplied the PoP is
//     verified locally before submission so a malformed PoP fails
//     fast without burning gas.
//
// For update, the matrix is augmented with one extra Option 0:
//
//   Option 0 (update only): all BLS flags empty.
//     No BLS field is sent — the update touches only the non-BLS
//     fields (name / operator / reward).
//
// Update Option 1 is opt-in via --bls-from-signer because the
// implicit default for "update with no BLS flag" is "don't touch BLS",
// not "rotate to the derived key."
type blsPoPFlags struct {
	pubKeyHex      string // --bls-pubkey
	popHex         string // --bls-pop
	privKeyHex     string // --bls-priv-key
	keystorePath   string // --bls-keystore (placeholder, see error message)
	fromSigner     bool   // --bls-from-signer (update path Option 1 opt-in)
	autoConfirm    bool   // --yes / -y
	candidateIDStr string // --candidate-id (for update; optional local verify in register Option 3)
}

// derivedBLSFromSigner decrypts the signer's ECDSA keystore and derives
// a BLS private key from it via the same algorithm as
// `ioctl account blskey` (ECDSA bytes used as IKM for
// crypto.GenerateBLS12381PrivateKey). Returns the typed BLS private
// key so the caller can both publish its pubkey and sign the PoP.
func derivedBLSFromSigner(signer, password string) (*crypto.BLS12381PrivateKey, error) {
	ecdsaSk, err := account.PrivateKeyFromSigner(signer, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt signer keystore")
	}
	defer ecdsaSk.Zero()
	bls, err := crypto.GenerateBLS12381PrivateKey(ecdsaSk.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive BLS key from ECDSA")
	}
	return bls, nil
}

// confirmDerivedBLS prints the auto-derived pubkey and coupling warning
// and waits for the user to type y/yes. Bypassed by --yes. The text is
// intentionally explicit about the coupling cost — the user is opting
// into "ECDSA leak compromises BLS identity too" and they should know.
func confirmDerivedBLS(signerAddr string, blsPubKey []byte, autoConfirm bool) error {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "No BLS credential flags provided. The tool will derive a BLS keypair from the")
	fmt.Fprintln(os.Stderr, "signer's ECDSA private key (same mechanism as `ioctl account blskey`).")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "  Signer:      %s\n", signerAddr)
	fmt.Fprintf(os.Stderr, "  BLS pubkey:  0x%s\n", hex.EncodeToString(blsPubKey))
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Coupling: a leak of your signer ECDSA private key would also expose this BLS key,")
	fmt.Fprintln(os.Stderr, "since anyone holding the ECDSA key can re-derive the BLS key. For separate")
	fmt.Fprintln(os.Stderr, "security domains use --bls-priv-key with a standalone BLS keypair instead.")
	fmt.Fprintln(os.Stderr, "")
	if autoConfirm {
		fmt.Fprintln(os.Stderr, "--yes supplied; proceeding with derived BLS key.")
		return nil
	}
	fmt.Fprint(os.Stderr, "Proceed with derived BLS key? [y/N]: ")
	resp, err := util.ReadSecretFromStdin()
	if err != nil {
		return errors.Wrap(err, "failed to read confirmation")
	}
	switch strings.ToLower(strings.TrimSpace(resp)) {
	case "y", "yes":
		return nil
	default:
		return errors.New("aborted: user declined to use derived BLS key")
	}
}

// resolveBLSForRegister returns (blsPubKey, blsPop) for the register
// command per the three-option matrix. ownerAddr is the candidateID
// at registration time (which becomes c.Identifier verbatim in the
// non-collision case via generateCandidateID's owner-first fast path).
func resolveBLSForRegister(flags *blsPoPFlags, signer, password string, ownerAddr address.Address) ([]byte, []byte, error) {
	if flags.keystorePath != "" {
		return nil, nil, output.NewError(output.FlagError,
			"--bls-keystore is not yet supported; use --bls-priv-key for now", nil)
	}
	mode, err := flags.classifyForRegister()
	if err != nil {
		return nil, nil, err
	}
	switch mode {
	case blsModeAutoDerive:
		sk, err := derivedBLSFromSigner(signer, password)
		if err != nil {
			return nil, nil, err
		}
		defer sk.Zero()
		pk := sk.PublicKey().Bytes()
		if err := confirmDerivedBLS(signer, pk, flags.autoConfirm); err != nil {
			return nil, nil, err
		}
		pop, err := staking.SignBLSPop(sk, ownerAddr)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign BLS PoP")
		}
		return pk, pop, nil

	case blsModeExplicitKey:
		sk, err := loadBLSPrivKey(flags.privKeyHex)
		if err != nil {
			return nil, nil, err
		}
		defer sk.Zero()
		pk := sk.PublicKey().Bytes()
		pop, err := staking.SignBLSPop(sk, ownerAddr)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign BLS PoP")
		}
		return pk, pop, nil

	case blsModeExplicitPoP:
		pk, pop, err := flags.decodeExplicitPubKeyAndPoP()
		if err != nil {
			return nil, nil, err
		}
		// Local fail-fast verification. ownerAddr is the canonical
		// candidateID for register so we can verify immediately
		// without an extra --candidate-id flag.
		if err := staking.VerifyBLSPop(pk, pop, ownerAddr); err != nil {
			return nil, nil, errors.Wrap(err, "supplied PoP fails local verification against ownerAddress")
		}
		return pk, pop, nil
	}
	return nil, nil, errors.New("unreachable: unhandled BLS mode")
}

// resolveBLSForUpdate returns (blsPubKey, blsPop) for the update
// command, or (nil, nil) if the user supplied no BLS flags at all
// (Option 0 — leave Candidate.BLSPubKey unchanged). candidateID is
// required for any non-zero option; the caller is expected to supply
// it via --candidate-id (a future improvement would resolve via RPC
// from the signer).
func resolveBLSForUpdate(flags *blsPoPFlags, signer, password string) ([]byte, []byte, error) {
	if flags.keystorePath != "" {
		return nil, nil, output.NewError(output.FlagError,
			"--bls-keystore is not yet supported; use --bls-priv-key for now", nil)
	}
	mode, err := flags.classifyForUpdate()
	if err != nil {
		return nil, nil, err
	}
	if mode == blsModeNone {
		return nil, nil, nil
	}
	if flags.candidateIDStr == "" {
		return nil, nil, output.NewError(output.FlagError,
			"BLS update requires --candidate-id (the candidate's current identifier); "+
				"future versions will resolve this via RPC from the signer", nil)
	}
	candidateID, err := address.FromString(flags.candidateIDStr)
	if err != nil {
		return nil, nil, output.NewError(output.AddressError, "invalid --candidate-id", err)
	}
	switch mode {
	case blsModeAutoDerive:
		sk, err := derivedBLSFromSigner(signer, password)
		if err != nil {
			return nil, nil, err
		}
		defer sk.Zero()
		pk := sk.PublicKey().Bytes()
		if err := confirmDerivedBLS(signer, pk, flags.autoConfirm); err != nil {
			return nil, nil, err
		}
		pop, err := staking.SignBLSPop(sk, candidateID)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign BLS PoP")
		}
		return pk, pop, nil

	case blsModeExplicitKey:
		sk, err := loadBLSPrivKey(flags.privKeyHex)
		if err != nil {
			return nil, nil, err
		}
		defer sk.Zero()
		pk := sk.PublicKey().Bytes()
		pop, err := staking.SignBLSPop(sk, candidateID)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to sign BLS PoP")
		}
		return pk, pop, nil

	case blsModeExplicitPoP:
		pk, pop, err := flags.decodeExplicitPubKeyAndPoP()
		if err != nil {
			return nil, nil, err
		}
		if err := staking.VerifyBLSPop(pk, pop, candidateID); err != nil {
			return nil, nil, errors.Wrap(err, "supplied PoP fails local verification against --candidate-id")
		}
		return pk, pop, nil
	}
	return nil, nil, errors.New("unreachable: unhandled BLS mode")
}

// loadBLSPrivKey parses a hex BLS private key. Reusable across
// register / update / bls-sign-pop.
func loadBLSPrivKey(hexStr string) (*crypto.BLS12381PrivateKey, error) {
	if hexStr == "" {
		return nil, errors.New("empty --bls-priv-key")
	}
	hexStr = strings.TrimPrefix(hexStr, "0x")
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid --bls-priv-key hex")
	}
	sk, err := crypto.BLS12381PrivateKeyFromBytes(b)
	if err != nil {
		return nil, errors.Wrap(err, "invalid BLS private key bytes")
	}
	return sk, nil
}

// blsMode is the resolved option selected by the user's flags.
type blsMode int

const (
	blsModeNone         blsMode = iota // update-only Option 0: don't touch BLS
	blsModeAutoDerive                  // Option 1: derive BLS from signer
	blsModeExplicitKey                 // Option 2: --bls-priv-key (or --bls-keystore in the future)
	blsModeExplicitPoP                 // Option 3: --bls-pubkey + --bls-pop
)

func (f *blsPoPFlags) classifyForRegister() (blsMode, error) {
	switch {
	case f.pubKeyHex != "" && f.popHex != "":
		if f.privKeyHex != "" {
			return 0, errMutex("--bls-pubkey/--bls-pop", "--bls-priv-key")
		}
		return blsModeExplicitPoP, nil
	case f.pubKeyHex != "" || f.popHex != "":
		return 0, output.NewError(output.FlagError,
			"--bls-pubkey and --bls-pop must be specified together, or use --bls-priv-key, "+
				"or supply neither for auto-derive from signer", nil)
	case f.privKeyHex != "":
		return blsModeExplicitKey, nil
	default:
		return blsModeAutoDerive, nil
	}
}

func (f *blsPoPFlags) classifyForUpdate() (blsMode, error) {
	switch {
	case f.pubKeyHex != "" && f.popHex != "":
		if f.privKeyHex != "" || f.fromSigner {
			return 0, errMutex("--bls-pubkey/--bls-pop", "--bls-priv-key/--bls-from-signer")
		}
		return blsModeExplicitPoP, nil
	case f.pubKeyHex != "" || f.popHex != "":
		return 0, output.NewError(output.FlagError,
			"--bls-pubkey and --bls-pop must be specified together; use --bls-priv-key or "+
				"--bls-from-signer to have the tool compute the PoP", nil)
	case f.privKeyHex != "":
		if f.fromSigner {
			return 0, errMutex("--bls-priv-key", "--bls-from-signer")
		}
		return blsModeExplicitKey, nil
	case f.fromSigner:
		return blsModeAutoDerive, nil
	default:
		return blsModeNone, nil
	}
}

func (f *blsPoPFlags) decodeExplicitPubKeyAndPoP() ([]byte, []byte, error) {
	pk, err := hex.DecodeString(strings.TrimPrefix(f.pubKeyHex, "0x"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid --bls-pubkey hex")
	}
	if _, err := crypto.BLS12381PublicKeyFromBytes(pk); err != nil {
		return nil, nil, errors.Wrap(err, "invalid BLS pubkey bytes")
	}
	pop, err := hex.DecodeString(strings.TrimPrefix(f.popHex, "0x"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid --bls-pop hex")
	}
	if len(pop) != crypto.BLSAggregateSignatureLength {
		return nil, nil, errors.Errorf("invalid --bls-pop length: got %d, want %d",
			len(pop), crypto.BLSAggregateSignatureLength)
	}
	return pk, pop, nil
}

func errMutex(a, b string) error {
	return output.NewError(output.FlagError,
		fmt.Sprintf("%s and %s are mutually exclusive", a, b), nil)
}
