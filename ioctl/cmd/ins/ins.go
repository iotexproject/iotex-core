package ins

import (
	"encoding/hex"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/sha3"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

// IONode hash for io
var IONode, _ = hex.DecodeString("b2b692c69df4aa3b0a24634d20a3ba1b44c3299d09d6c4377577e20b09e68395")

// Multi-language support
var (
	_insCmdShorts = map[config.Language]string{
		config.English: "Manage INS of IoTeX blockchain",
		config.Chinese: "管理IoTeX区块链上INS",
	}
	_flagInsEndPointUsages = map[config.Language]string{
		config.English: "set endpoint for once",
		config.Chinese: "一次设置端点", // this translation
	}
	_flagInsInsecureUsages = map[config.Language]string{
		config.English: "insecure connection for once",
		config.Chinese: "一次不安全连接", // this translation
	}
)

// InsCmd represents the ins command
var InsCmd = &cobra.Command{
	Use:   "ins",
	Short: config.TranslateInLang(_insCmdShorts, config.UILanguage),
}

func init() {
	InsCmd.AddCommand(_insRegisterCmd)
	InsCmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, config.TranslateInLang(_flagInsEndPointUsages,
			config.UILanguage))
	InsCmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		config.TranslateInLang(_flagInsInsecureUsages, config.UILanguage))
}

func nameHash(name string) (hash [32]byte, err error) {
	sha := sha3.NewLegacyKeccak256()
	if _, err = sha.Write(IONode); err != nil {
		return
	}
	nameSha := sha3.NewLegacyKeccak256()
	if _, err = nameSha.Write([]byte(name)); err != nil {
		return
	}
	nameHash := nameSha.Sum(nil)
	if _, err = sha.Write(nameHash); err != nil {
		return
	}
	sha.Sum(hash[:0])
	return
}
