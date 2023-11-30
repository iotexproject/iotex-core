package ws

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
)

var (
	// wsCode represents the w3bstream code command
	wsCode = &cobra.Command{
		Use:   "code",
		Short: config.TranslateInLang(wsCodeShorts, config.UILanguage),
	}

	// wsCodeShorts w3bstream code shorts multi-lang support
	wsCodeShorts = map[config.Language]string{
		config.English: "ws code operations",
		config.Chinese: "ws代码操作",
	}
)

func init() {
	wsCode.AddCommand(wsCodeConvert)
}

type VmType string

const (
	Risc0  VmType = "risc0"
	Halo2  VmType = "halo2"
	ZkWasm VmType = "zkwasm"
)

func stringToVmType(vmType string) (VmType, error) {
	switch vmType {
	case string(Risc0):
		return Risc0, nil
	case string(Halo2):
		return Halo2, nil
	case string(ZkWasm):
		return ZkWasm, nil
	default:
		return "", errors.New(fmt.Sprintf("not support %s type, just support %s, %s, and %s", vmType, Risc0, Halo2, ZkWasm))
	}
}
