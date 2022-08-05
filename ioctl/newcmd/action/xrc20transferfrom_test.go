package action

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewXrc20TransferFromCmd(t *testing.T) {
	var (
		require = require.New(t)
		ctrl    = gomock.NewController(t)

		wrongAddr         = strings.Repeat("a", 41)
		wrongAlias        = "xxx"
		wrongAmount       = "xxx"
		wrongContractData = "xxx"

		dftOwner        = identityset.Address(0).String()
		dftRecipient    = identityset.Address(1).String()
		dftContract     = identityset.Address(2).String()
		dftAmount       = "100"
		dftContractData = "2F"
		dftBalance      = "20000000000000100000"
	)

	passwd := "passwd"
	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	account, err := ks.NewAccount(passwd)
	require.NoError(err)
	accountAddr, err := address.FromBytes(account.Address.Bytes())
	require.NoError(err)

	newcmd := func(
		contractAddr string,
		contractData string,
		owner string,
		balance string,
	) *cobra.Command {
		cli := mock_ioctlclient.NewMockClient(ctrl)
		api := mock_iotexapi.NewMockAPIServiceClient(ctrl)

		cli.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
		cli.EXPECT().APIServiceClient().Return(api, nil).AnyTimes()

		cli.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().NewKeyStore().Return(ks).AnyTimes()
		cli.EXPECT().PackABI(gomock.Any(), gomock.Any()).Return([]byte(""), nil)
		cli.EXPECT().IsCryptoSm2().Return(false).AnyTimes()
		cli.EXPECT().ReadSecret().Return(passwd, nil).AnyTimes()
		cli.EXPECT().Address(gomock.Any()).Return(owner, nil).AnyTimes()
		cli.EXPECT().Alias(gomock.Any()).Return("", nil).AnyTimes()
		cli.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).AnyTimes()

		api.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&iotexapi.ReadContractResponse{Data: contractData}, nil)
		api.EXPECT().GetAccount(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetAccountResponse{
				AccountMeta: &iotextypes.AccountMeta{PendingNonce: 100},
			}, nil)
		api.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetChainMetaResponse{}, nil)

		api.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
			Return(&iotexapi.GetAccountResponse{
				AccountMeta: &iotextypes.AccountMeta{Balance: balance},
			}, nil)
		cli.EXPECT().Xrc20ContractAddr().Return(contractAddr)
		api.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil)

		cli.EXPECT().Config().Return(config.Config{
			Explorer: "iotexscan",
			Endpoint: "testnet1",
		}).Times(2)

		cmd := NewXrc20TransferFromCmd(cli)
		xrc20ContractAddr := ""
		cmd.PersistentFlags().StringVarP(&xrc20ContractAddr,
			"contract-address",
			"c",
			contractAddr,
			"set contract address",
		)
		return cmd
	}

	tests := []struct {
		name        string
		args        []string
		newcmd      *cobra.Command
		expectedErr string
	}{
		{
			"WrongArgCount",
			nil,
			newcmd("", "", "", ""),
			"accepts 3 arg(s), received 0",
		}, {
			"WrongOwnerAlias",
			[]string{wrongAlias, "", ""},
			newcmd(dftContract, dftContractData, wrongAlias, dftBalance),
			fmt.Sprintf("failed to get owner address: address length = %d, expecting 41: invalid address", len(wrongAlias)),
		}, {
			"WrongOwnerAddress",
			[]string{wrongAddr, "", ""},
			newcmd(dftContract, dftContractData, wrongAddr, dftBalance),
			"failed to get owner address: invalid index of 1: invalid address",
		}, {
			"WrongContractAddress",
			[]string{dftOwner, dftRecipient, dftAmount},
			newcmd(wrongAddr, dftContractData, dftOwner, dftBalance),
			fmt.Sprintf("invalid address #%s", dftOwner),
		}, {
			"WrongAmount",
			[]string{dftOwner, dftRecipient, wrongAmount},
			newcmd(dftContract, dftContractData, dftOwner, dftBalance),
			"failed to parse amount: failed to convert string into big int",
		}, {
			"WrongContractDataRsp",
			[]string{dftOwner, dftRecipient, dftAmount},
			newcmd(dftContract, wrongContractData, dftOwner, dftBalance),
			fmt.Sprintf("failed to parse amount: failed to convert string into int64: strconv.ParseInt: parsing \"%s\": invalid syntax", wrongContractData),
		}, {
			"BalanceNotEnough",
			[]string{dftOwner, dftRecipient, dftAmount},
			newcmd(dftContract, dftContractData, accountAddr.String(), "0"),
			"failed to pass balance check: balance is not enough",
		}, {
			"ShouldPass",
			[]string{dftOwner, dftRecipient, dftAmount},
			newcmd(dftContract, dftContractData, accountAddr.String(), dftBalance),
			"",
		}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.newcmd
			result, err := util.ExecuteCmd(cmd, test.args...)
			if test.expectedErr == "" {
				require.NoError(err)
				require.Contains(result, "Action has been sent to blockchain.\nWait for several seconds and query this action by hash:")
			} else {
				require.Equal(test.expectedErr, err.Error())
			}
		})
	}
}
