// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package flag

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

// vars
var (
	flagWithArgumentsUsage = map[config.Language]string{
		config.English: "pass arguments in JSON format",
		config.Chinese: "按照JSON格式传入参数",
	}
	WithArgumentsFlag = NewStringVar("with-arguments", "",
		config.TranslateInLang(flagWithArgumentsUsage, config.UILanguage))
)

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
		Value() interface{}
		MarkFlagRequired(*cobra.Command)
	}

	stringVarP struct {
		flagBase
		value        string
		defaultValue string
	}

	uint64VarP struct {
		flagBase
		value        uint64
		defaultValue uint64
	}

	boolVarP struct {
		flagBase
		value        bool
		defaultValue bool
	}
)

func (f *flagBase) MarkFlagRequired(cmd *cobra.Command) {
	cmd.MarkFlagRequired(f.label)
}

func (f *flagBase) Label() string {
	return f.label
}

// NewStringVarP creates a new stringVarP flag
func NewStringVarP(
	label string,
	shortLabel string,
	defaultValue string,
	description string,
) Flag {
	return &stringVarP{
		flagBase: flagBase{
			label:       label,
			shortLabel:  shortLabel,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

// NewStringVar creates a new stringVar flag
func NewStringVar(
	label string,
	defaultValue string,
	description string,
) Flag {
	return &stringVarP{
		flagBase: flagBase{
			label:       label,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

func (f *stringVarP) Value() interface{} {
	return f.value
}

func (f *stringVarP) RegisterCommand(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.value, f.label, f.shortLabel, f.defaultValue, f.description)
}

// NewUint64VarP creates a new uint64VarP flag
func NewUint64VarP(
	label string,
	shortLabel string,
	defaultValue uint64,
	description string,
) Flag {
	return &uint64VarP{
		flagBase: flagBase{
			label:       label,
			shortLabel:  shortLabel,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

func (f *uint64VarP) RegisterCommand(cmd *cobra.Command) {
	cmd.Flags().Uint64VarP(&f.value, f.label, f.shortLabel, f.defaultValue, f.description)
}

func (f *uint64VarP) Value() interface{} {
	return f.value
}

// BoolVarP creates a new stringVarP flag
func BoolVarP(
	label string,
	shortLabel string,
	defaultValue bool,
	description string,
) Flag {
	return &boolVarP{
		flagBase: flagBase{
			label:       label,
			shortLabel:  shortLabel,
			description: description,
		},
		defaultValue: defaultValue,
	}
}

func (f *boolVarP) Value() interface{} {
	return f.value
}

func (f *boolVarP) RegisterCommand(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.value, f.label, f.shortLabel, f.defaultValue, f.description)
}
