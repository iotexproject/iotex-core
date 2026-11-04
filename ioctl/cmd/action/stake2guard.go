// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_stake2GuardCmdShorts = map[config.Language]string{
		config.English: "Quantum-safe staking buckets (IIP-65): guard vault, bind and guarded operations",
		config.Chinese: "抗量子质押桶（IIP-65）：守护密钥库、绑定与受守护操作",
	}
	_stake2GuardCmdLongs = map[config.Language]string{
		config.English: `Quantum-safe staking buckets (IIP-65).

A guard is a post-quantum (SLH-DSA) key bound to a native bucket. Once bound,
only a CALL signed by the guard key can unstake, withdraw, transfer, restake,
change the candidate of the bucket or rotate the key; any account may submit
it and pay the gas (the sponsor, -s).

Guard keys are derived from a dedicated BIP-39 mnemonic, the guard vault,
stored encrypted in --vault-file. Keep it separate from your account keys and
back the mnemonic up: a lost guard key locks the bucket for good.

Operations run online in one step, or offline in several: prepare with
--out FILE, sign (and accept, for a new key) on the device holding the vault,
then submit from any account.`,
		config.Chinese: `抗量子质押桶（IIP-65）。守护密钥由独立的 BIP-39 助记词派生，加密保存在 --vault-file。`,
	}
	_stake2GuardFlagVaultFileUsages = map[config.Language]string{
		config.English: "guard vault file (default <wallet>/guardvault)",
		config.Chinese: "守护密钥库文件（默认 <wallet>/guardvault）",
	}
	_stake2GuardFlagVaultIndexUsages = map[config.Language]string{
		config.English: "QSB-KDF vault index of the guard keys",
		config.Chinese: "守护密钥的 QSB-KDF 库索引",
	}
	_stake2GuardFlagVaultPasswordUsages = map[config.Language]string{
		config.English: "guard vault password (for scripts; prompted if empty)",
		config.Chinese: "守护密钥库密码（脚本用；为空时提示输入）",
	}
)

var (
	_guardVaultFile     string
	_guardVaultIndex    uint32
	_guardVaultPassword string
)

// _stake2GuardCmd is the stake2 guard command group.
var _stake2GuardCmd = &cobra.Command{
	Use:   "guard",
	Short: config.TranslateInLang(_stake2GuardCmdShorts, config.UILanguage),
	Long:  config.TranslateInLang(_stake2GuardCmdLongs, config.UILanguage),
}

func init() {
	_stake2GuardCmd.AddCommand(_guardVaultCmd)
	pf := _stake2GuardCmd.PersistentFlags()
	pf.StringVar(&_guardVaultFile, "vault-file", "", config.TranslateInLang(_stake2GuardFlagVaultFileUsages, config.UILanguage))
	pf.Uint32Var(&_guardVaultIndex, "vault-index", 0, config.TranslateInLang(_stake2GuardFlagVaultIndexUsages, config.UILanguage))
	pf.StringVar(&_guardVaultPassword, "vault-password", "", config.TranslateInLang(_stake2GuardFlagVaultPasswordUsages, config.UILanguage))
}

// vaultFilePath resolves --vault-file lazily: the wallet directory comes from
// the config, which flags may change.
func vaultFilePath() string {
	if _guardVaultFile != "" {
		return _guardVaultFile
	}
	return filepath.Join(config.ReadConfig.Wallet, "guardvault")
}

func readSecret(prompt string) (string, error) {
	output.PrintQuery(prompt)
	s, err := util.ReadSecretFromStdin()
	if err != nil {
		return "", output.NewError(output.InputError, "failed to read input", err)
	}
	return s, nil
}

// readNewSecret reads a secret twice. flagValue, when set, is used as is.
func readNewSecret(what, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	s, err := readSecret(fmt.Sprintf("Set %s:", what))
	if err != nil {
		return "", err
	}
	again, err := readSecret(fmt.Sprintf("Enter %s again:", what))
	if err != nil {
		return "", err
	}
	if s != again {
		return "", output.NewError(output.ValidationError, what+" does not match", nil)
	}
	return s, nil
}

// unlockVault decrypts the vault at path and derives the keyring of
// vaultIndex. It prompts for the password unless password is set, and for
// the BIP-39 passphrase if the vault was created with one.
func unlockVault(path, password string, vaultIndex uint32) (*qsb.Keyring, error) {
	v, err := openVault(path, password)
	if err != nil {
		return nil, err
	}
	passphrase := ""
	if v.HasPassphrase {
		if passphrase, err = readSecret("Enter the BIP-39 passphrase of the guard vault:"); err != nil {
			return nil, err
		}
	}
	k, err := v.Unlock(passphrase, vaultIndex)
	if err != nil {
		return nil, output.NewError(output.ValidationError, "failed to unlock guard vault", err)
	}
	return k, nil
}

func openVault(path, password string) (*qsb.Vault, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, output.NewError(output.ReadFileError,
				fmt.Sprintf("no guard vault at %s; run 'ioctl stake2 guard vault create' or 'vault import'", path), nil)
		}
		return nil, output.NewError(output.ReadFileError, "failed to read guard vault", err)
	}
	if password == "" {
		if password, err = readSecret(fmt.Sprintf("Enter the password of guard vault %s:", path)); err != nil {
			return nil, err
		}
	}
	v, err := qsb.DecryptVault(data, password)
	if err != nil {
		return nil, output.NewError(output.KeystoreError, "failed to decrypt guard vault", err)
	}
	return v, nil
}

// rawMessage prints any JSON-marshalable value as an output message.
type rawMessage struct{ v any }

func (m rawMessage) String() string               { return "" }
func (m rawMessage) MarshalJSON() ([]byte, error) { return json.Marshal(m.v) }

// printGuardResult prints text, or data as a JSON result with -o json.
func printGuardResult(text string, data any) {
	if output.Format == "" {
		fmt.Println(text)
		return
	}
	fmt.Println(output.FormatString(output.Result, rawMessage{data}))
}
