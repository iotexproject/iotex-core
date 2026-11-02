// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/fileutil"
)

// Multi-language support
var (
	_guardVaultCmdShorts = map[config.Language]string{
		config.English: "Manage the guard vault, the mnemonic guard keys are derived from",
		config.Chinese: "管理守护密钥库（派生守护密钥的助记词）",
	}
	_guardVaultCreateCmdShorts = map[config.Language]string{
		config.English: "Create a guard vault with a new mnemonic",
		config.Chinese: "用新助记词创建守护密钥库",
	}
	_guardVaultImportCmdShorts = map[config.Language]string{
		config.English: "Import a guard vault from a mnemonic",
		config.Chinese: "从助记词导入守护密钥库",
	}
	_guardVaultExportCmdShorts = map[config.Language]string{
		config.English: "Print the mnemonic of the guard vault",
		config.Chinese: "显示守护密钥库的助记词",
	}
	_guardVaultDeleteCmdShorts = map[config.Language]string{
		config.English: "Delete the guard vault file",
		config.Chinese: "删除守护密钥库文件",
	}
	_guardVaultKeyCmdShorts = map[config.Language]string{
		config.English: "Print guard public keys (give one to the owner transferring a bucket to you)",
		config.Chinese: "显示守护公钥",
	}
	_guardVaultFindCmdUses = map[config.Language]string{
		config.English: "find (PUBLIC_KEY | --bucket BUCKET_INDEX) [--scan-window N]",
		config.Chinese: "find (公钥 | --bucket 票索引) [--scan-window N]",
	}
	_guardVaultFindCmdShorts = map[config.Language]string{
		config.English: "Find the key index of a guard public key or of a bucket's guard",
		config.Chinese: "查找守护公钥或质押桶守护密钥的索引",
	}
)

var (
	_guardVaultWords      int
	_guardVaultPassphrase bool
	_guardKeyIndex        uint32
	_guardKeyCount        uint32
	_guardFindBucket      uint64
	_guardFindFirst       uint32
	_guardFindWindow      uint32
)

var _guardVaultCmd = &cobra.Command{
	Use:   "vault",
	Short: config.TranslateInLang(_guardVaultCmdShorts, config.UILanguage),
}

var _guardVaultCreateCmd = &cobra.Command{
	Use:   "create [--words 24] [--passphrase]",
	Short: config.TranslateInLang(_guardVaultCreateCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardVaultCreate(""))
	},
}

var _guardVaultImportCmd = &cobra.Command{
	Use:   "import [--passphrase]",
	Short: config.TranslateInLang(_guardVaultImportCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := func() error {
			m, err := readSecret("Enter the mnemonic of the guard vault:")
			if err != nil {
				return err
			}
			return guardVaultCreate(m)
		}()
		return output.PrintError(err)
	},
}

var _guardVaultExportCmd = &cobra.Command{
	Use:   "export",
	Short: config.TranslateInLang(_guardVaultExportCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := func() error {
			v, err := openVault(vaultFilePath(), _guardVaultPassword)
			if err != nil {
				return err
			}
			text := "Mnemonic: " + v.Mnemonic
			if v.HasPassphrase {
				text += "\nThe vault also needs its BIP-39 passphrase, which is not stored."
			}
			printGuardResult(text, map[string]any{"mnemonic": v.Mnemonic, "hasPassphrase": v.HasPassphrase})
			return nil
		}()
		return output.PrintError(err)
	},
}

var _guardVaultDeleteCmd = &cobra.Command{
	Use:   "delete [-y]",
	Short: config.TranslateInLang(_guardVaultDeleteCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := func() error {
			path := vaultFilePath()
			if !fileutil.FileExists(path) {
				return output.NewError(output.ReadFileError, "no guard vault at "+path, nil)
			}
			if _yesFlag.Value() == false {
				ok, err := output.Confirm(fmt.Sprintf("Delete guard vault %s? Buckets guarded by its keys are "+
					"locked for good unless the mnemonic is backed up.", path))
				if err != nil {
					return output.NewError(output.InputError, "use -y to skip confirmation", err)
				}
				if !ok {
					return nil
				}
			}
			if err := os.Remove(path); err != nil {
				return output.NewError(output.WriteFileError, "failed to delete guard vault", err)
			}
			output.PrintResult("Deleted guard vault " + path)
			return nil
		}()
		return output.PrintError(err)
	},
}

var _guardVaultKeyCmd = &cobra.Command{
	Use:   "key [--key-index N] [--count K]",
	Short: config.TranslateInLang(_guardVaultKeyCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardVaultKey())
	},
}

var _guardVaultFindCmd = &cobra.Command{
	Use:   config.TranslateInLang(_guardVaultFindCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_guardVaultFindCmdShorts, config.UILanguage),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardVaultFind(cmd, args))
	},
}

func init() {
	_guardVaultCmd.AddCommand(_guardVaultCreateCmd, _guardVaultImportCmd, _guardVaultExportCmd,
		_guardVaultDeleteCmd, _guardVaultKeyCmd, _guardVaultFindCmd)
	_guardVaultCreateCmd.Flags().IntVar(&_guardVaultWords, "words", qsb.DefaultWords, "number of mnemonic words (12, 15, 18, 21 or 24)")
	for _, c := range []*cobra.Command{_guardVaultCreateCmd, _guardVaultImportCmd} {
		c.Flags().BoolVar(&_guardVaultPassphrase, "passphrase", false,
			"protect the seed with a BIP-39 passphrase (prompted; not stored, needed with every use)")
	}
	_yesFlag.RegisterCommand(_guardVaultDeleteCmd)
	_guardVaultKeyCmd.Flags().Uint32Var(&_guardKeyIndex, "key-index", 0, "first key index")
	_guardVaultKeyCmd.Flags().Uint32Var(&_guardKeyCount, "count", 1, "number of keys")
	_guardVaultFindCmd.Flags().Uint64Var(&_guardFindBucket, "bucket", 0, "find the key of this bucket's guard (reads the chain)")
	_guardVaultFindCmd.Flags().Uint32Var(&_guardFindFirst, "first", 0, "first key index to scan")
	_guardVaultFindCmd.Flags().Uint32Var(&_guardFindWindow, "scan-window", signer.DefaultQSBScanWindow, "number of key indices to scan")
}

// guardVaultCreate writes a new vault, from mnemonic or, if empty, a new one.
func guardVaultCreate(mnemonic string) error {
	path := vaultFilePath()
	if fileutil.FileExists(path) {
		return output.NewError(output.ValidationError,
			fmt.Sprintf("a guard vault already exists at %s; delete it or use --vault-file", path), nil)
	}
	created := mnemonic == ""
	if created {
		var err error
		if mnemonic, err = qsb.NewMnemonic(_guardVaultWords); err != nil {
			return output.NewError(output.InputError, "", err)
		}
	} else if _, err := qsb.Seed(mnemonic, ""); err != nil {
		return output.NewError(output.ValidationError, "", err)
	}
	passphrase := ""
	if _guardVaultPassphrase {
		var err error
		if passphrase, err = readNewSecret("the BIP-39 passphrase", ""); err != nil {
			return err
		}
		if passphrase == "" {
			return output.NewError(output.ValidationError, "the BIP-39 passphrase must not be empty", nil)
		}
	}
	password, err := readNewSecret("the guard vault password", _guardVaultPassword)
	if err != nil {
		return err
	}
	data, err := qsb.EncryptVault(mnemonic, passphrase, password)
	if err != nil {
		return output.NewError(output.CryptoError, "failed to encrypt guard vault", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return output.NewError(output.WriteFileError, "", err)
	}
	if err := qsb.WriteFileAtomic(path, data); err != nil {
		return output.NewError(output.WriteFileError, "failed to write guard vault", err)
	}
	seed, err := qsb.Seed(mnemonic, passphrase)
	if err != nil {
		return output.NewError(output.CryptoError, "", err)
	}
	kr := qsb.NewKeyring(seed, _guardVaultIndex)
	defer kr.Zero()
	k, err := kr.Key(0)
	if err != nil {
		return output.NewError(output.CryptoError, "", err)
	}
	pub := hex.EncodeToString(k.PublicKey().Bytes())
	data2 := map[string]any{"vaultFile": path, "vaultIndex": _guardVaultIndex, "key0": pub}
	text := fmt.Sprintf("Guard vault written to %s\nVault index %d, key 0: %s", path, _guardVaultIndex, pub)
	if created {
		data2["mnemonic"] = mnemonic
		text += "\n\nMnemonic: " + qsb.NormalizeMnemonic(mnemonic) +
			"\nWrite it down and keep it offline, apart from your account keys. Without it (and the passphrase," +
			" if set) a guarded bucket cannot be recovered."
	}
	printGuardResult(text, data2)
	return nil
}

type guardKeyInfo struct {
	VaultIndex uint32 `json:"vaultIndex"`
	KeyIndex   uint32 `json:"keyIndex"`
	Alg        string `json:"alg"`
	AlgID      uint32 `json:"algID"`
	PublicKey  string `json:"publicKey"`
}

func guardVaultKey() error {
	if _guardKeyCount == 0 || uint64(_guardKeyIndex)+uint64(_guardKeyCount)-1 > math.MaxUint32 {
		return output.NewError(output.FlagError, "invalid --key-index/--count", nil)
	}
	kr, err := unlockVault(vaultFilePath(), _guardVaultPassword, _guardVaultIndex)
	if err != nil {
		return err
	}
	defer kr.Zero()
	var (
		keys  []guardKeyInfo
		lines []string
	)
	for i := uint64(0); i < uint64(_guardKeyCount); i++ {
		idx := _guardKeyIndex + uint32(i)
		k, err := kr.Key(idx)
		if err != nil {
			return output.NewError(output.CryptoError, "", err)
		}
		pub := k.PublicKey()
		keys = append(keys, guardKeyInfo{
			VaultIndex: kr.VaultIndex(), KeyIndex: idx, Alg: pub.Alg().String(), AlgID: uint32(pub.Alg()),
			PublicKey: hex.EncodeToString(pub.Bytes()),
		})
		lines = append(lines, fmt.Sprintf("key %d: %s", idx, hex.EncodeToString(pub.Bytes())))
	}
	text := fmt.Sprintf("Guard keys of vault index %d (%s):\n%s", kr.VaultIndex(), pqcrypto.AlgSLHDSA192s, strings.Join(lines, "\n"))
	printGuardResult(text, keys)
	return nil
}

func parseGuardPublicKey(s string) (*pqcrypto.PublicKey, error) {
	b, err := hex.DecodeString(util.TrimHexPrefix(s))
	if err != nil {
		return nil, output.NewError(output.ConvertError, "invalid guard public key", err)
	}
	pk, err := pqcrypto.ParsePublicKey(pqcrypto.AlgSLHDSA192s, b)
	if err != nil {
		return nil, output.NewError(output.ConvertError, "invalid guard public key", err)
	}
	return pk, nil
}

func guardVaultFind(cmd *cobra.Command, args []string) error {
	byBucket := cmd.Flags().Changed("bucket")
	if byBucket == (len(args) == 1) {
		return output.NewError(output.InputError, "give either a public key or --bucket", nil)
	}
	if _guardFindWindow == 0 || uint64(_guardFindFirst)+uint64(_guardFindWindow)-1 > math.MaxUint32 {
		return output.NewError(output.FlagError, "invalid --first/--scan-window", nil)
	}
	var pub *pqcrypto.PublicKey
	if byBucket {
		g, _, err := bc.GetBucketGuard(_guardFindBucket, 0)
		if err != nil {
			return output.NewError(0, "failed to read the bucket guard", err)
		}
		if len(g.GetPublicKey()) == 0 {
			return output.NewError(output.ValidationError, fmt.Sprintf("bucket %d is not guarded", _guardFindBucket), nil)
		}
		if pub, err = pqcrypto.ParsePublicKey(pqcrypto.AlgID(g.GetAlgID()), g.GetPublicKey()); err != nil {
			return output.NewError(output.ConvertError, "", err)
		}
	} else {
		var err error
		if pub, err = parseGuardPublicKey(args[0]); err != nil {
			return err
		}
	}
	idx, ok, err := findGuardKey(pub, _guardFindFirst, _guardFindWindow)
	if err != nil {
		return err
	}
	last := _guardFindFirst + _guardFindWindow - 1
	if !ok {
		return output.NewError(output.ValidationError, fmt.Sprintf(
			"the key is not in key indices %d..%d of vault index %d; try another --vault-index, --first or --scan-window",
			_guardFindFirst, last, _guardVaultIndex), nil)
	}
	printGuardResult(fmt.Sprintf("Found: vault index %d, key index %d", _guardVaultIndex, idx),
		map[string]any{"vaultIndex": _guardVaultIndex, "keyIndex": idx})
	return nil
}

// findGuardKey unlocks the vault and scans window key indices from first.
func findGuardKey(pub *pqcrypto.PublicKey, first, window uint32) (uint32, bool, error) {
	kr, err := unlockVault(vaultFilePath(), _guardVaultPassword, _guardVaultIndex)
	if err != nil {
		return 0, false, err
	}
	defer kr.Zero()
	return scanKeyring(kr, pub, first, window)
}

func scanKeyring(kr *qsb.Keyring, pub *pqcrypto.PublicKey, first, window uint32) (uint32, bool, error) {
	last := first + window - 1
	start := time.Now()
	idx, ok, err := kr.Find(context.Background(), pub, first, last)
	if err != nil {
		return 0, false, output.NewError(output.CryptoError, "", err)
	}
	if !output.Quiet && output.Format == "" && window > 64 {
		fmt.Printf("Searched key indices %d..%d in %v\n", first, last, time.Since(start).Round(time.Millisecond))
	}
	return idx, ok, nil
}
