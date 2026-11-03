// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
)

// Multi-language support
var (
	_guardSignCmdShorts = map[config.Language]string{
		config.English: "Sign a request file offline with the guard vault (CALL, and ACCEPT with --new-key-index)",
		config.Chinese: "用守护密钥库离线签署请求文件",
	}
	_guardAcceptCmdShorts = map[config.Language]string{
		config.English: "Accept a new guard key offline: sign ACCEPT of a request file with the incoming key",
		config.Chinese: "离线用新守护密钥签署请求文件的 ACCEPT",
	}
	_guardInspectCmdShorts = map[config.Language]string{
		config.English: "Show and verify a request file",
		config.Chinese: "显示并验证请求文件",
	}
	_guardSubmitCmdShorts = map[config.Language]string{
		config.English: "Submit a signed request file from any account (the owner for a bind)",
		config.Chinese: "从任意账户提交已签署的请求文件",
	}
)

var (
	_guardEthData    bool
	_guardSkipChecks bool
)

var _guardSignCmd = &cobra.Command{
	Use:   "sign FILE [--key-index N] [--new-key-index N] [--out FILE] [-y]",
	Short: config.TranslateInLang(_guardSignCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardSignFile(cmd, args[0], true))
	},
}

var _guardAcceptCmd = &cobra.Command{
	Use:   "accept FILE [--key-index N] [--out FILE] [-y]",
	Short: config.TranslateInLang(_guardAcceptCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardSignFile(cmd, args[0], false))
	},
}

var _guardInspectCmd = &cobra.Command{
	Use:   "inspect FILE [--eth-data]",
	Short: config.TranslateInLang(_guardInspectCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(guardInspect(args[0]))
	},
}

var _guardSubmitCmd = &cobra.Command{
	Use:   "submit FILE" + _guardWriteFlags,
	Short: config.TranslateInLang(_guardSubmitCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := func() error {
			r, err := readGuardRequest(args[0])
			if err != nil {
				return err
			}
			sum := qsb.Summary(r)
			printGuardResult(sum.String(), sum)
			return submitRequest(cmd, r, _guardSkipChecks)
		}()
		return output.PrintError(err)
	},
}

func init() {
	_stake2GuardCmd.AddCommand(_guardSignCmd, _guardAcceptCmd, _guardInspectCmd, _guardSubmitCmd)
	registerKeyFlags(_guardSignCmd, true)
	registerKeyFlags(_guardAcceptCmd, false)
	for _, c := range []*cobra.Command{_guardSignCmd, _guardAcceptCmd} {
		c.Flags().StringVar(&_guardOut, "out", "", "write the signed request to FILE (default: overwrite the input)")
		_yesFlag.RegisterCommand(c)
	}
	_guardInspectCmd.Flags().BoolVar(&_guardEthData, "eth-data", false, "print the web3 call data of a complete request")
	RegisterWriteCommand(_guardSubmitCmd)
	_guardSubmitCmd.Flags().BoolVar(&_guardSkipChecks, "skip-checks", false, "send without comparing the request with the chain (tests)")
	if err := _guardSubmitCmd.Flags().MarkHidden("skip-checks"); err != nil {
		panic(err)
	}
}

func readGuardRequest(path string) (*qsb.Request, error) {
	r, err := qsb.ReadRequest(path)
	if err != nil {
		return nil, output.NewError(output.ReadFileError, "invalid request file", err)
	}
	return r, nil
}

// guardSignFile signs a request file: CALL (and ACCEPT with --new-key-index)
// for sign, ACCEPT only for accept. It needs no network. Existing signatures
// are verified first, and the result before it is written.
func guardSignFile(cmd *cobra.Command, path string, signCall bool) error {
	r, err := readGuardRequest(path)
	if err != nil {
		return err
	}
	if err := qsb.Verify(r); err != nil {
		return output.NewError(output.ValidationError, "the request carries an invalid signature", err)
	}
	if !signCall && !r.NeedsAccept() {
		return output.NewError(output.InputError, "the request installs no new guard key; nothing to accept", nil)
	}
	if r.Complete() || (!signCall && r.AcceptSigned()) {
		output.PrintResult("Nothing to sign: the request already carries the signatures.")
		return nil
	}
	if signCall && r.Kind == qsb.KindOp && r.CallSigned() && (!r.NeedsAccept() || !cmd.Flags().Changed("new-key-index")) {
		output.PrintResult("CALL is already signed; the new key's holder still has to run accept.")
		return nil
	}
	sum := qsb.Summary(r)
	printGuardResult(sum.String(), sum)
	if _yesFlag.Value() == false {
		what := "Sign this operation?"
		if !signCall || r.Kind == qsb.KindBind {
			what = fmt.Sprintf("Accept guard ownership of bucket %d for owner %s with this vault?", r.Bucket.Index, sum.ResultingOwner)
		}
		ok, err := output.Confirm(what)
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
	keyIndex := keyIndexFlag(cmd, "key-index", _guardOpKeyIndex)
	if signCall {
		err = signRequest(r, kr, &guardSignOptions{
			keyIndex:      keyIndex,
			scanWindow:    _guardScanWindow,
			newKeyIndex:   keyIndexFlag(cmd, "new-key-index", _guardNewKeyIndex),
			newVault:      _guardNewVaultFile,
			newVaultIndex: _guardNewVaultIndex,
			newVaultPass:  _guardNewVaultPassword,
			signCall:      true,
		})
	} else {
		err = signAcceptWith(r, kr, keyIndex, _guardScanWindow)
	}
	if err != nil {
		return err
	}
	if err := qsb.Verify(r); err != nil {
		return output.NewError(output.CryptoError, "the signed request does not verify", err)
	}
	out := path
	if _guardOut != "" {
		out = _guardOut
	}
	if err := qsb.WriteRequestFile(out, r); err != nil {
		return output.NewError(output.WriteFileError, "failed to write the request", err)
	}
	call, accept := qsb.Signatures(r)
	status := fmt.Sprintf("Wrote %s (CALL: %s, ACCEPT: %s)", out, call, accept)
	if r.Complete() {
		switch sp := r.Sponsor(); {
		case r.Kind == qsb.KindBind:
			status += "\nComplete. Submit as the owner: ioctl stake2 guard submit " + out + " -s " + r.Bucket.Owner.String()
		case sp != nil:
			status += "\nComplete. Submit as the sponsor: ioctl stake2 guard submit " + out + " -s " + sp.String()
		default:
			status += "\nComplete. Submit from any account: ioctl stake2 guard submit " + out + " -s SPONSOR"
		}
	}
	printGuardResult(status, map[string]any{"file": out, "call": call, "accept": accept, "complete": r.Complete()})
	return nil
}

type guardInspectMessage struct {
	*qsb.SummaryMessage
	Verified     bool   `json:"verified"`
	Error        string `json:"error,omitempty"`
	IntrinsicGas uint64 `json:"intrinsicGas,omitempty"`
	EthTo        string `json:"ethTo,omitempty"`
	EthData      string `json:"ethData,omitempty"`
}

// guardInspect prints and verifies a request file. It fails if a signature
// is invalid.
func guardInspect(path string) error {
	r, err := readGuardRequest(path)
	if err != nil {
		return err
	}
	m := &guardInspectMessage{SummaryMessage: qsb.Summary(r)}
	verr := qsb.Verify(r)
	m.Verified = verr == nil
	if verr != nil {
		m.Error = verr.Error()
	}
	text := m.SummaryMessage.String()
	if r.Complete() && verr == nil {
		a, err := qsb.Payload(r)
		if err != nil {
			return output.NewError(output.ValidationError, "", err)
		}
		if m.IntrinsicGas, err = a.IntrinsicGas(); err != nil {
			return output.NewError(output.ValidationError, "", err)
		}
		text += fmt.Sprintf("\n  %-20s %d (gas limit %d by default)", "intrinsic gas:", m.IntrinsicGas, m.IntrinsicGas+_guardGasMargin)
		if _guardEthData {
			to, err := a.EthTo()
			if err != nil {
				return output.NewError(output.ValidationError, "", err)
			}
			data, err := a.EthData()
			if err != nil {
				return output.NewError(output.ValidationError, "", err)
			}
			m.EthTo, m.EthData = to.Hex(), "0x"+hex.EncodeToString(data)
			text += fmt.Sprintf("\n  %-20s %s\n  %-20s %s", "eth to:", m.EthTo, "eth data:", m.EthData)
		}
	}
	switch {
	case verr != nil:
		text += "\n  verification:        FAILED: " + verr.Error()
	case r.Complete():
		text += "\n  verification:        complete and valid"
	default:
		text += "\n  verification:        present signatures are valid; not complete"
	}
	printGuardResult(text, m)
	if verr != nil {
		return output.NewError(output.ValidationError, "the request does not verify", verr)
	}
	return nil
}
