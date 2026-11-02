// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package qsb is the wallet side of IIP-65 quantum-safe staking buckets for
// ioctl: the encrypted guard vault (a dedicated BIP-39 mnemonic, IIP §12.2),
// request files for offline signing (IIP §12.3), signing, verification and
// summaries. It has no network or CLI code.
package qsb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/text/unicode/norm"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

// Vault file envelope.
const (
	VaultType    = "iotex-qsb-guard-vault"
	VaultVersion = 1
)

// DefaultWords is the default mnemonic length of a new vault.
const DefaultWords = 24

var (
	// ErrWrongPassword is returned when the vault password does not decrypt the vault.
	ErrWrongPassword = errors.New("wrong vault password")
	// ErrWrongPassphrase is returned when the BIP-39 passphrase does not
	// reproduce the key the vault was created with.
	ErrWrongPassphrase = errors.New("wrong BIP-39 passphrase")
)

// scrypt parameters of new vaults; tests lower them.
var scryptN, scryptP = keystore.StandardScryptN, keystore.StandardScryptP

// vaultPayload is the plaintext of a vault. The BIP-39 passphrase is never
// stored; Check, the first 8 bytes of SHA-256 of the vault-0 key-0 public
// key, detects a wrong one.
type vaultPayload struct {
	Mnemonic      string `json:"mnemonic"`
	HasPassphrase bool   `json:"hasPassphrase"`
	Check         string `json:"check"`
}

type vaultFile struct {
	Type    string              `json:"type"`
	Version int                 `json:"version"`
	Crypto  keystore.CryptoJSON `json:"crypto"`
}

// Vault is a decrypted vault. Unlock it to derive keys.
type Vault struct {
	Mnemonic      string
	HasPassphrase bool
	check         []byte
}

// NewMnemonic returns a new random BIP-39 mnemonic of 12, 15, 18, 21 or 24 words.
func NewMnemonic(words int) (string, error) {
	if words < 12 || words > 24 || words%3 != 0 {
		return "", fmt.Errorf("mnemonic must have 12, 15, 18, 21 or 24 words, got %d", words)
	}
	entropy, err := bip39.NewEntropy(words * 32 / 3)
	if err != nil {
		return "", err
	}
	return bip39.NewMnemonic(entropy)
}

// NormalizeMnemonic returns m in NFKD with words separated by single spaces.
func NormalizeMnemonic(m string) string {
	return strings.Join(strings.Fields(norm.NFKD.String(m)), " ")
}

// Seed returns the 64-byte BIP-39 seed of mnemonic and passphrase, both NFKD
// normalized as BIP-39 requires. The mnemonic's checksum is verified.
func Seed(mnemonic, passphrase string) ([]byte, error) {
	m := NormalizeMnemonic(mnemonic)
	seed, err := bip39.NewSeedWithErrorChecking(m, norm.NFKD.String(passphrase))
	if err != nil {
		return nil, fmt.Errorf("invalid mnemonic: %w", err)
	}
	return seed, nil
}

// checkValue identifies the seed without revealing a usable key.
func checkValue(seed []byte) ([]byte, error) {
	k, err := signer.DeriveQSBKey(seed, 0, 0)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(k.PublicKey().Bytes())
	return h[:8], nil
}

// EncryptVault encrypts mnemonic under password (scrypt, keystore v3 cipher
// and MAC). passphrase is the BIP-39 passphrase; it is not stored, only a
// check value derived from the seed it produces.
func EncryptVault(mnemonic, passphrase, password string) ([]byte, error) {
	if password == "" {
		return nil, errors.New("vault password must not be empty")
	}
	seed, err := Seed(mnemonic, passphrase)
	if err != nil {
		return nil, err
	}
	defer zero(seed)
	check, err := checkValue(seed)
	if err != nil {
		return nil, err
	}
	plain, err := json.Marshal(vaultPayload{
		Mnemonic:      NormalizeMnemonic(mnemonic),
		HasPassphrase: passphrase != "",
		Check:         hex.EncodeToString(check),
	})
	if err != nil {
		return nil, err
	}
	defer zero(plain)
	c, err := keystore.EncryptDataV3(plain, []byte(password), scryptN, scryptP)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(vaultFile{Type: VaultType, Version: VaultVersion, Crypto: c}, "", "  ")
}

// DecryptVault decrypts a vault written by EncryptVault.
func DecryptVault(data []byte, password string) (*Vault, error) {
	var f vaultFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("not a guard vault: %w", err)
	}
	if f.Type != VaultType || f.Version != VaultVersion {
		return nil, fmt.Errorf("not a guard vault (type %q, version %d)", f.Type, f.Version)
	}
	plain, err := keystore.DecryptDataV3(f.Crypto, password)
	if err != nil {
		if errors.Is(err, keystore.ErrDecrypt) {
			return nil, ErrWrongPassword
		}
		return nil, err
	}
	defer zero(plain)
	var p vaultPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return nil, fmt.Errorf("corrupt vault: %w", err)
	}
	check, err := hex.DecodeString(p.Check)
	if err != nil || len(check) != 8 {
		return nil, errors.New("corrupt vault: bad check value")
	}
	if !bip39.IsMnemonicValid(p.Mnemonic) {
		return nil, errors.New("corrupt vault: invalid mnemonic")
	}
	return &Vault{Mnemonic: p.Mnemonic, HasPassphrase: p.HasPassphrase, check: check}, nil
}

// Unlock derives the seed with passphrase, checks it against the vault, and
// returns a keyring for vaultIndex.
func (v *Vault) Unlock(passphrase string, vaultIndex uint32) (*Keyring, error) {
	if !v.HasPassphrase && passphrase != "" {
		return nil, ErrWrongPassphrase
	}
	seed, err := Seed(v.Mnemonic, passphrase)
	if err != nil {
		return nil, err
	}
	check, err := checkValue(seed)
	if err != nil {
		zero(seed)
		return nil, err
	}
	if !bytes.Equal(check, v.check) {
		zero(seed)
		return nil, ErrWrongPassphrase
	}
	return &Keyring{seed: seed, vault: vaultIndex}, nil
}

// Keyring derives the guard keys of one vault index (QSB-KDF-v1).
type Keyring struct {
	seed  []byte
	vault uint32
}

// NewKeyring returns a keyring over a BIP-39 seed.
func NewKeyring(seed []byte, vaultIndex uint32) *Keyring {
	return &Keyring{seed: bytes.Clone(seed), vault: vaultIndex}
}

// VaultIndex returns the vault index of k.
func (k *Keyring) VaultIndex() uint32 { return k.vault }

// Key returns the guard key at keyIndex.
func (k *Keyring) Key(keyIndex uint32) (*signer.PrivateKey, error) {
	if k.seed == nil {
		return nil, errors.New("keyring is closed")
	}
	return signer.DeriveQSBKey(k.seed, k.vault, keyIndex)
}

// FindWorkers is the number of goroutines Find derives keys with.
var FindWorkers = runtime.NumCPU()

// Find scans key indices first..last for the key whose public key is pub.
// Keys are derived in parallel (about 35 ms each on one core). It returns
// false if none matches.
func (k *Keyring) Find(ctx context.Context, pub *pqcrypto.PublicKey, first, last uint32) (uint32, bool, error) {
	if pub == nil || first > last {
		return 0, false, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		next    = uint64(first)
		found   atomic.Bool
		index   uint32
		mu      sync.Mutex
		firstEr error
		wg      sync.WaitGroup
	)
	workers := FindWorkers
	if n := uint64(last) - uint64(first) + 1; uint64(workers) > n {
		workers = int(n)
	}
	if workers < 1 {
		workers = 1
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := atomic.AddUint64(&next, 1) - 1
				if i > uint64(last) || i > math.MaxUint32 {
					return
				}
				key, err := k.Key(uint32(i))
				mu.Lock()
				switch {
				case err != nil:
					if firstEr == nil {
						firstEr = err
					}
					cancel()
				case key.PublicKey().Equal(pub):
					if !found.Load() || uint32(i) < index {
						index = uint32(i)
					}
					found.Store(true)
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if firstEr != nil {
		return 0, false, firstEr
	}
	if found.Load() {
		return index, true, nil
	}
	return 0, false, ctx.Err()
}

// Zero overwrites the seed. The keyring cannot be used afterwards.
func (k *Keyring) Zero() {
	zero(k.seed)
	k.seed = nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
