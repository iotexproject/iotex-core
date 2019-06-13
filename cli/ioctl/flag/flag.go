// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package flag

import "github.com/spf13/cobra"

type (
	flagBase struct {
		label       string
		shortLabel  string
		description string
	}

	// Flag defines a cobra command flag
	Flag interface {
		Label() string
		RegisterCommand(*cobra.Command)
	}

	// StringVarP is a flag of StringVarP type
	StringVarP struct {
		flagBase
		Value        string
		defaultValue string
	}

	// Uint64VarP is a flag of Uint64VarP type
	Uint64VarP struct {
		flagBase
		Value        uint64
		defaultValue uint64
	}
)

func (f *flagBase) MarkFlagRequired(cmd *cobra.Command) {
	cmd.MarkFlagRequired(f.label)
}

func NewStringVarP(
	label string,
	shortLabel string,
	defaultValue string,
	description string,
) *StringVarP {
	return &StringVarP{
		flagBase: flagBase{
			label:       label,
			shortLabel:  shortLabel,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

func (f *StringVarP) RegisterCommand(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.Value, f.label, f.shortLabel, f.defaultValue, f.description)
}

func NewUint64VarP(
	label string,
	shortLabel string,
	defaultValue uint64,
	description string,
) *Uint64VarP {
	return &Uint64VarP{
		flagBase: flagBase{
			label:       label,
			shortLabel:  shortLabel,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

func (f *Uint64VarP) RegisterCommand(cmd *cobra.Command) {
	cmd.Flags().Uint64VarP(&f.Value, f.label, f.shortLabel, f.defaultValue, f.description)
}
