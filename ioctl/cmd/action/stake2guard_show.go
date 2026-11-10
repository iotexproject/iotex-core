// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/bc"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/qsb"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Multi-language support
var (
	_guardShowCmdUses = map[config.Language]string{
		config.English: "show BUCKET_INDEX [--height HEIGHT]",
		config.Chinese: "show 票索引 [--height 高度]",
	}
	_guardShowCmdShorts = map[config.Language]string{
		config.English: "Show the guard record of a bucket and the bucket",
		config.Chinese: "显示质押桶的守护记录",
	}
)

var _guardShowHeight uint64

var _guardShowCmd = &cobra.Command{
	Use:   config.TranslateInLang(_guardShowCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_guardShowCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := func() error {
			index, err := parseBucketIndex(args[0])
			if err != nil {
				return err
			}
			return showGuard(index, _guardShowHeight)
		}()
		return output.PrintError(err)
	},
}

func init() {
	_stake2GuardCmd.AddCommand(_guardShowCmd)
	_guardShowCmd.Flags().Uint64Var(&_guardShowHeight, "height", 0, "block height to read at (default: tip; older heights need an archive node)")
}

func parseBucketIndex(s string) (uint64, error) {
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, output.NewError(output.ConvertError, "invalid bucket index", err)
	}
	return i, nil
}

type guardBucketInfo struct {
	Owner          string `json:"owner"`
	Candidate      string `json:"candidate"`
	StakedAmount   string `json:"stakedAmount"`
	StakedDuration uint32 `json:"stakedDuration"`
	AutoStake      bool   `json:"autoStake"`
	Unstaked       bool   `json:"unstaked"`
}

type guardShowMessage struct {
	Bucket      uint64           `json:"bucket"`
	Height      uint64           `json:"height"`
	State       string           `json:"state"`
	AlgID       uint32           `json:"algID"`
	Alg         string           `json:"alg,omitempty"`
	PublicKey   string           `json:"publicKey,omitempty"`
	Generation  uint64           `json:"generation"`
	Nonce       uint64           `json:"nonce"`
	BoundHeight uint64           `json:"boundHeight"`
	BucketInfo  *guardBucketInfo `json:"bucketInfo,omitempty"`
	BucketError string           `json:"bucketError,omitempty"`
}

func newGuardShowMessage(index, height uint64, g *iotextypes.BucketGuard) *guardShowMessage {
	m := &guardShowMessage{
		Bucket: index, Height: height, State: "guarded", AlgID: g.GetAlgID(),
		Generation: g.GetGeneration(), Nonce: g.GetNonce(), BoundHeight: g.GetBoundHeight(),
	}
	switch {
	case len(g.GetPublicKey()) == 0 && g.GetGeneration() == 0:
		m.State = "no guard"
	case len(g.GetPublicKey()) == 0:
		m.State = "unguarded (cleared)"
	default:
		m.Alg = pqcrypto.AlgID(g.GetAlgID()).String()
		m.PublicKey = hex.EncodeToString(g.GetPublicKey())
	}
	return m
}

func (m *guardShowMessage) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "bucket %d at height %d: %s\n", m.Bucket, m.Height, m.State)
	if m.PublicKey != "" {
		fmt.Fprintf(&b, "  guard key:        %s %s\n", m.Alg, m.PublicKey)
	}
	if m.State != "no guard" {
		fmt.Fprintf(&b, "  generation/nonce: %d/%d\n", m.Generation, m.Nonce)
		fmt.Fprintf(&b, "  key changed at:   %d\n", m.BoundHeight)
	}
	if bi := m.BucketInfo; bi != nil {
		fmt.Fprintf(&b, "  owner:            %s\n", bi.Owner)
		fmt.Fprintf(&b, "  candidate:        %s\n", bi.Candidate)
		fmt.Fprintf(&b, "  staked:           %s, %d days, autoStake %v, unstaked %v\n",
			qsb.FormatRau(bi.StakedAmount), bi.StakedDuration, bi.AutoStake, bi.Unstaked)
	} else if m.BucketError != "" {
		fmt.Fprintf(&b, "  bucket:           %s\n", m.BucketError)
	}
	return strings.TrimRight(b.String(), "\n")
}

// showGuard prints the guard record of a bucket and the bucket.
func showGuard(index, height uint64) error {
	g, readHeight, err := bc.GetBucketGuard(index, height)
	if err != nil {
		return output.NewError(0, "failed to read the bucket guard", err)
	}
	m := newGuardShowMessage(index, readHeight, g)
	if b, _, err := bc.GetBucketByIndex(index, height); err == nil {
		m.BucketInfo = &guardBucketInfo{
			Owner:          b.GetOwner(),
			Candidate:      b.GetCandidateAddress(),
			StakedAmount:   b.GetStakedAmount(),
			StakedDuration: b.GetStakedDuration(),
			AutoStake:      b.GetAutoStake(),
			Unstaked:       b.GetUnstakeStartTime().AsTime().After(b.GetStakeStartTime().AsTime()),
		}
	} else {
		m.BucketError = "not found (withdrawn?)"
	}
	printGuardResult(m.String(), m)
	return nil
}
