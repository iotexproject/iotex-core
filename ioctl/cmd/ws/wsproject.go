package ws

import (
	"bytes"
	"crypto/sha256"
	_ "embed" // used to embed contract abi
	"encoding/hex"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/go-pkgs/hash"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
)

type Command struct {
}

// flags
var (
	projectFilePath = flag.NewStringVarP("path", "", "", config.TranslateInLang(_flagProjectFileUsage, config.UILanguage))
	projectFileHash = flag.NewStringVarP("hash", "", "", config.TranslateInLang(_flagProjectHashUsage, config.UILanguage))
	projectID       = flag.NewUint64VarP("id", "", 0, config.TranslateInLang(_flagProjectIDUsage, config.UILanguage))
	projectAttrKey  = flag.NewStringVarP("key", "", "", config.TranslateInLang(_flagProjectAttributeKeyUsage, config.UILanguage))
	projectAttrVal  = flag.NewStringVarP("val", "", "", config.TranslateInLang(_flagProjectAttributeValUsage, config.UILanguage))
)

// flag multi-languages
var (
	_flagProjectFileUsage = map[config.Language]string{
		config.English: "project config file path",
		config.Chinese: "项目配置文件路径",
	}

	_flagProjectHashUsage = map[config.Language]string{
		config.English: "project config file hash",
		config.Chinese: "项目配置文件哈希",
	}

	_flagProjectIDUsage = map[config.Language]string{
		config.English: "project id",
		config.Chinese: "项目ID",
	}

	_flagProjectAttributeKeyUsage = map[config.Language]string{
		config.English: "project attribute key",
		config.Chinese: "项目属性键",
	}

	_flagProjectAttributeValUsage = map[config.Language]string{
		config.English: "project attribute value",
		config.Chinese: "项目属性值",
	}
)

var wsProject = &cobra.Command{
	Use: "project",
	Short: config.TranslateInLang(map[config.Language]string{
		config.English: "w3bstream project management",
		config.Chinese: "w3bstream项目管理",
	}, config.UILanguage),
}

// ioctl global configuration
var (
	ipfsEndpoint = "ipfs.mainnet.iotex.io"
)

// contracts and abis
var (
	//go:embed contracts/abis/ProjectRegistrar.json
	projectRegistrarJSON []byte
	projectRegistrarABI  abi.ABI
	projectRegistrarAddr string

	//go:embed contracts/abis/W3bstreamProject.json
	projectStoreJSON    []byte
	projectStoreABI     abi.ABI
	projectStoreAddress string
)

// contract function
const (
	funcProjectRegister     = "register"
	funcUpdateProjectConfig = "updateConfig"
	funcQueryProject        = "config"
	funcPauseProject        = "pause"
	funcResumeProject       = "resume"
	funcIsProjectPaused     = "isPaused"
	funcGetProjectAttr      = "attribute"
	funcSetProjectAttr      = "setAttributes"
	funcQueryProjectOwner   = "ownerOf"
)

// contract events care about
const (
	eventOnProjectRegistered  = "ProjectBinded"
	eventProjectConfigUpdated = "ProjectConfigUpdated"
	eventProjectPaused        = "ProjectPaused"
	eventProjectResumed       = "ProjectResumed"
	eventProjectAttrSet       = "AttributeSet"
)

func init() {
	var err error
	projectRegistrarABI, err = abi.JSON(bytes.NewReader(projectRegistrarJSON))
	if err != nil {
		panic(err)
	}
	projectRegistrarAddr = config.ReadConfig.WsProjectRegisterContract
	projectStoreABI, err = abi.JSON(bytes.NewReader(projectStoreJSON))
	if err != nil {
		panic(err)
	}
	projectStoreAddress = config.ReadConfig.WsProjectStoreContract

	WsCmd.AddCommand(wsProject)
}

// uploadToIPFS uploads content to IPFS, returns fetch url and content hash
func uploadToIPFS(endpoint string, filename, hashstr string) (string, hash.Hash256, error) {
	// read file content
	content, err := os.ReadFile(filename)
	if err != nil {
		err = errors.Wrapf(err, "failed to read file: %s", filename)
		return "", hash.ZeroHash256, err
	}

	// calculate and validate hash
	hash256b := sha256.Sum256(content)
	if hashstr != "" {
		var hashInput hash.Hash256
		hashInput, err = hash.HexStringToHash256(hashstr)
		if err != nil {
			return "", hash.ZeroHash256, err
		}
		if hashInput != hash256b {
			return "", hash.ZeroHash256, errors.Errorf("failed to validate file hash, expect %s actually %s", hashstr, hex.EncodeToString(hash256b[:]))
		}
	}

	// upload content to ipfs endpoint
	var (
		sh  = shell.NewShell(endpoint)
		cid string
	)
	cid, err = sh.Add(bytes.NewReader(content))
	if err != nil {
		return "", hash.ZeroHash256, errors.Wrap(err, "failed to upload file to IPFS")
	}

	err = sh.Pin(cid)
	if err != nil {
		return "", hash.ZeroHash256, errors.Wrapf(err, "failed to pin file to IPFS, cid: %s", cid)
	}

	return fmt.Sprintf("ipfs://%s/%s", endpoint, cid), hash256b, nil
}
