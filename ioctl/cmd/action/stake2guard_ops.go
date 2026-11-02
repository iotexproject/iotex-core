// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Flags of the op commands, shared with sign/accept/submit. One command runs
// per process.
var (
	_guardOpKeyIndex       uint32
	_guardScanWindow       uint32
	_guardDeadline         uint64
	_guardDeadlineBlocks   uint64
	_guardOut              string
	_guardAllowHighIndex   bool
	_guardRecipient        string
	_guardNewOwner         string
	_guardClearGuard       bool
	_guardNewKeyIndex      uint32
	_guardNewVaultFile     string
	_guardNewVaultIndex    uint32
	_guardNewVaultPassword string
	_guardNewGuardPubkey   string
	_guardOpAutoStake      bool
)

const _guardWriteFlags = " [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y] [-w]"

// guardOpSpec describes one online command.
type guardOpSpec struct {
	op    bucketguard.Op // 0 for bind
	use   string
	short map[config.Language]string
	nargs int
}

var _guardOpSpecs = []guardOpSpec{
	{0, "bind BUCKET_INDEX --key-index N [--out FILE]", map[config.Language]string{
		config.English: "Bind a guard key to a native bucket (sent by the owner; irreversible for the owner key)",
		config.Chinese: "为质押桶绑定守护密钥（由所有者发送）",
	}, 1},
	{bucketguard.OpUnstake, "unstake BUCKET_INDEX [--out FILE]", map[config.Language]string{
		config.English: "Unstake a guarded bucket",
		config.Chinese: "解除受守护质押桶的质押",
	}, 1},
	{bucketguard.OpWithdraw, "withdraw BUCKET_INDEX --recipient ADDRESS [--out FILE]", map[config.Language]string{
		config.English: "Withdraw a guarded bucket to a recipient",
		config.Chinese: "将受守护质押桶提取到接收地址",
	}, 1},
	{bucketguard.OpTransfer, "transfer BUCKET_INDEX --new-owner ADDRESS (--clear-guard | --new-key-index N [--new-vault-file FILE] | --new-guard-pubkey HEX --out FILE)", map[config.Language]string{
		config.English: "Transfer a guarded bucket, with a new guard key or clearing the guard",
		config.Chinese: "转让受守护质押桶",
	}, 1},
	{bucketguard.OpChangeCandidate, "change-candidate BUCKET_INDEX CANDIDATE_NAME [--out FILE]", map[config.Language]string{
		config.English: "Change the candidate of a guarded bucket",
		config.Chinese: "更改受守护质押桶的候选人",
	}, 2},
	{bucketguard.OpRestake, "restake BUCKET_INDEX STAKE_DURATION [--auto-stake] [--out FILE]", map[config.Language]string{
		config.English: "Restake a guarded bucket",
		config.Chinese: "重新质押受守护质押桶",
	}, 2},
	{bucketguard.OpRotate, "rotate BUCKET_INDEX (--new-key-index N | --new-guard-pubkey HEX --out FILE)", map[config.Language]string{
		config.English: "Replace the guard key of a bucket",
		config.Chinese: "更换质押桶的守护密钥",
	}, 1},
}

func init() {
	for _, spec := range _guardOpSpecs {
		_stake2GuardCmd.AddCommand(newGuardOpCmd(spec))
	}
}

// registerKeyFlags registers the key selection flags of commands that sign.
func registerKeyFlags(cmd *cobra.Command, newKey bool) {
	cmd.Flags().Uint32Var(&_guardOpKeyIndex, "key-index", 0, "key index of the signing guard key (default: search --scan-window indices for it)")
	cmd.Flags().Uint32Var(&_guardScanWindow, "scan-window", _defaultOpScanWindow, "key indices searched for the guard key when --key-index is not given")
	cmd.Flags().BoolVar(&_guardAllowHighIndex, "allow-high-index", false, "allow a new guard key index beyond the default recovery window (1024)")
	if newKey {
		cmd.Flags().Uint32Var(&_guardNewKeyIndex, "new-key-index", 0, "key index of the new guard key")
		cmd.Flags().StringVar(&_guardNewVaultFile, "new-vault-file", "", "guard vault of the new key (default: --vault-file)")
		cmd.Flags().Uint32Var(&_guardNewVaultIndex, "new-vault-index", 0, "vault index of the new key in --new-vault-file")
		cmd.Flags().StringVar(&_guardNewVaultPassword, "new-vault-password", "", "password of --new-vault-file (prompted if empty)")
	}
}

func newGuardOpCmd(spec guardOpSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   spec.use + _guardWriteFlags,
		Short: config.TranslateInLang(spec.short, config.UILanguage),
		Args:  cobra.ExactArgs(spec.nargs),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return output.PrintError(runGuardOp(cmd, spec.op, args))
		},
	}
	RegisterWriteCommand(cmd)
	f := cmd.Flags()
	f.StringVar(&_guardOut, "out", "", "write the prepared request to FILE for offline signing instead of signing and sending it")
	registerKeyFlags(cmd, spec.op == bucketguard.OpTransfer || spec.op == bucketguard.OpRotate)
	if spec.op != 0 {
		f.Uint64Var(&_guardDeadlineBlocks, "deadline-blocks", _defaultDeadlineBlocks,
			fmt.Sprintf("deadline in blocks after the tip (default %d, or %d with --out)", _defaultDeadlineBlocks, _defaultOfflineDeadlineBlocks))
		f.Uint64Var(&_guardDeadline, "deadline", 0, "deadline as an absolute block height")
	}
	switch spec.op {
	case bucketguard.OpWithdraw:
		f.StringVar(&_guardRecipient, "recipient", "", "address or alias receiving the withdrawn amount")
	case bucketguard.OpTransfer:
		f.StringVar(&_guardNewOwner, "new-owner", "", "address or alias of the new owner")
		f.BoolVar(&_guardClearGuard, "clear-guard", false, "remove the guard: the new owner's ECDSA key alone controls the bucket")
		f.StringVar(&_guardNewGuardPubkey, "new-guard-pubkey", "", "public key of the new guard (from the new owner's 'vault key'); needs --out")
	case bucketguard.OpRotate:
		f.StringVar(&_guardNewGuardPubkey, "new-guard-pubkey", "", "public key of the new guard; needs --out")
	case bucketguard.OpRestake:
		f.BoolVar(&_guardOpAutoStake, "auto-stake", false, "auto-stake")
	}
	return cmd
}

// keyIndexFlag returns the flag's value if it was given.
func keyIndexFlag(cmd *cobra.Command, name string, v uint32) *uint32 {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &v
}

func resolveAddress(flag, s string) (address.Address, error) {
	if s == "" {
		return nil, output.NewError(output.FlagError, "--"+flag+" is required", nil)
	}
	a, err := util.Address(s)
	if err != nil {
		return nil, output.NewError(output.AddressError, "invalid --"+flag, err)
	}
	return address.FromString(a)
}

// guardOpParams validates the op-specific flags, before any network access.
func guardOpParams(cmd *cobra.Command, o bucketguard.Op, args []string) (*qsb.OpParams, error) {
	offline := _guardOut != ""
	changed := cmd.Flags().Changed
	p := &qsb.OpParams{Op: o}
	var err error
	switch o {
	case bucketguard.OpWithdraw:
		if p.Recipient, err = resolveAddress("recipient", _guardRecipient); err != nil {
			return nil, err
		}
	case bucketguard.OpTransfer:
		if p.NewOwner, err = resolveAddress("new-owner", _guardNewOwner); err != nil {
			return nil, err
		}
		p.ClearGuard = _guardClearGuard
		modes := 0
		for _, m := range []bool{_guardClearGuard, changed("new-key-index"), _guardNewGuardPubkey != ""} {
			if m {
				modes++
			}
		}
		if modes > 1 || (modes == 0 && !offline) {
			return nil, output.NewError(output.FlagError, "give one of --clear-guard, --new-key-index or --new-guard-pubkey", nil)
		}
	case bucketguard.OpChangeCandidate:
		if !action.IsValidCandidateName(args[1]) {
			return nil, output.NewError(output.ValidationError, "", action.ErrInvalidCanName)
		}
		p.CandidateName = args[1]
	case bucketguard.OpRestake:
		d, err := parseStakeDuration(args[1])
		if err != nil {
			return nil, err
		}
		p.Duration, p.AutoStake = uint32(d.Uint64()), _guardOpAutoStake
	case bucketguard.OpRotate:
		if changed("new-key-index") && _guardNewGuardPubkey != "" {
			return nil, output.NewError(output.FlagError, "give --new-key-index or --new-guard-pubkey, not both", nil)
		}
		if !offline && !changed("new-key-index") {
			return nil, output.NewError(output.FlagError, "--new-key-index is required (or --new-guard-pubkey with --out)", nil)
		}
	}
	if _guardNewGuardPubkey != "" {
		if !offline {
			return nil, output.NewError(output.FlagError,
				"--new-guard-pubkey needs --out: the holder of the new key signs ACCEPT with 'accept'", nil)
		}
		if p.NewGuardKey, err = parseGuardPublicKey(_guardNewGuardPubkey); err != nil {
			return nil, err
		}
	}
	if changed("new-key-index") {
		if err := checkKeyIndex(_guardNewKeyIndex); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func runGuardOp(cmd *cobra.Command, o bucketguard.Op, args []string) error {
	index, err := parseBucketIndex(args[0])
	if err != nil {
		return err
	}
	offline := _guardOut != ""
	changed := cmd.Flags().Changed
	if offline {
		for _, f := range []string{"key-index", "new-key-index", "new-vault-file"} {
			if changed(f) {
				return output.NewError(output.FlagError, fmt.Sprintf(
					"--out prepares the request only; pass --%s to 'ioctl stake2 guard sign'", f), nil)
			}
		}
	}
	var p *qsb.OpParams
	if o == 0 {
		if !offline && !changed("key-index") {
			return output.NewError(output.FlagError, "--key-index is required: the guard key to bind", nil)
		}
		if !offline {
			if err := checkKeyIndex(_guardOpKeyIndex); err != nil {
				return err
			}
		}
	} else if p, err = guardOpParams(cmd, o, args); err != nil {
		return err
	}

	s, err := loadChainState(index)
	if err != nil {
		return err
	}
	var r *qsb.Request
	if o == 0 {
		r, err = qsb.NewBindRequest(s, nil)
	} else {
		if p.Deadline, err = guardDeadline(cmd, s.Height, offline); err != nil {
			return err
		}
		if o == bucketguard.OpChangeCandidate {
			if p.ExpectedCandidateID, err = candidateID(p.CandidateName); err != nil {
				return err
			}
		}
		r, err = qsb.NewOpRequest(s, p)
	}
	if err != nil {
		return output.NewError(output.ValidationError, "", err)
	}
	if offline {
		if err := qsb.WriteRequestFile(_guardOut, r); err != nil {
			return output.NewError(output.WriteFileError, "failed to write the request", err)
		}
		sum := qsb.Summary(r)
		printGuardResult(fmt.Sprintf("%s\n\nWrote %s\n%s", sum, _guardOut, nextSteps(r, _guardOut)),
			map[string]any{"file": _guardOut, "summary": sum})
		return nil
	}

	if o == 0 {
		sender, err := Signer()
		if err != nil {
			return output.NewError(output.AddressError, "failed to get signer address", err)
		}
		if !util.AliasIsHdwalletKey(sender) && sender != s.Owner.String() {
			return output.NewError(output.ValidationError, fmt.Sprintf(
				"a bind must be sent by the bucket owner %s (-s), not %s", s.Owner, sender), nil)
		}
	}
	sum := qsb.Summary(r)
	printGuardResult(sum.String(), sum)
	if o == 0 && _yesFlag.Value() == false {
		ok, err := output.Confirm(fmt.Sprintf("Bind guard key %d of vault index %d to bucket %d? After this the owner key "+
			"alone cannot move the bucket, and losing the vault mnemonic locks it for good.",
			_guardOpKeyIndex, _guardVaultIndex, index))
		if err != nil {
			return output.NewError(output.InputError, "use -y to skip confirmation", err)
		}
		if !ok {
			return nil
		}
	}
	kr, err := unlockVault(vaultFilePath(), _guardVaultPassword, _guardVaultIndex)
	if err != nil {
		return err
	}
	defer kr.Zero()
	if err := signRequest(r, kr, &guardSignOptions{
		keyIndex:      keyIndexFlag(cmd, "key-index", _guardOpKeyIndex),
		scanWindow:    _guardScanWindow,
		newKeyIndex:   keyIndexFlag(cmd, "new-key-index", _guardNewKeyIndex),
		newVault:      _guardNewVaultFile,
		newVaultIndex: _guardNewVaultIndex,
		newVaultPass:  _guardNewVaultPassword,
		signCall:      true,
	}); err != nil {
		return err
	}
	return submitRequest(cmd, r, false)
}

// nextSteps tells the user how to finish an offline request.
func nextSteps(r *qsb.Request, file string) string {
	var b strings.Builder
	b.WriteString("Next steps:\n")
	if r.Kind == qsb.KindBind {
		fmt.Fprintf(&b, "  on the vault device:    ioctl stake2 guard sign %s --key-index N\n", file)
		fmt.Fprintf(&b, "  then, as the owner:     ioctl stake2 guard submit %s -s %s\n", file, r.Bucket.Owner)
		return strings.TrimRight(b.String(), "\n")
	}
	switch {
	case r.IncomingPending():
		fmt.Fprintf(&b, "  on the vault device:    ioctl stake2 guard sign %s --new-key-index N\n", file)
		fmt.Fprintf(&b, "    (or the new key's holder first runs: ioctl stake2 guard accept %s --key-index N)\n", file)
	case r.NeedsAccept():
		fmt.Fprintf(&b, "  new key's holder:       ioctl stake2 guard accept %s\n", file)
		fmt.Fprintf(&b, "  current guard holder:   ioctl stake2 guard sign %s   (either order)\n", file)
	default:
		fmt.Fprintf(&b, "  on the vault device:    ioctl stake2 guard sign %s\n", file)
	}
	fmt.Fprintf(&b, "  then, from any account: ioctl stake2 guard submit %s -s SPONSOR\n", file)
	fmt.Fprintf(&b, "  the signed file is valid until block %d", r.Op.Deadline)
	return b.String()
}
