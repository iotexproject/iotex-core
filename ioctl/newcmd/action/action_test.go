// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestSigner(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(2)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {})
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {})

	t.Run("returns signer's address", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("test", nil).AnyTimes()

		cmd := NewActionCmd(client)
		registerSignerFlag(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--signer", "test")
		require.NoError(err)
		signer, err := cmd.Flags().GetString(signerFlagLabel)
		require.NoError(err)
		result, err := Signer(client, signer)
		require.NoError(err)
		require.Equal(result, "test")
	})
}

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	selp := &iotextypes.Action{}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(3)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(7)

	for _, test := range []struct {
		endpoint string
		insecure bool
	}{
		{
			endpoint: "111:222:333:444:5678",
			insecure: false,
		},
		{
			endpoint: "",
			insecure: true,
		},
	} {
		callbackEndpoint := func(cb func(*string, string, string, string)) {
			cb(&test.endpoint, "endpoint", test.endpoint, "endpoint usage")
		}
		callbackInsecure := func(cb func(*bool, string, bool, string)) {
			cb(&test.insecure, "insecure", !test.insecure, "insecure usage")
		}
		client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(callbackEndpoint).Times(3)
		client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(callbackInsecure).Times(3)

		t.Run("sends raw action to blockchain", func(t *testing.T) {
			response := &iotexapi.SendActionResponse{}

			apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(response, nil).Times(3)

			cmd := NewActionCmd(client)
			_, err := util.ExecuteCmd(cmd)
			require.NoError(err)

			t.Run("endpoint iotexscan", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotexscan",
					Endpoint: "testnet1",
				}).Times(2)

				err = SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint iotxplorer", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "iotxplorer",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})

			t.Run("endpoint default", func(t *testing.T) {
				client.EXPECT().Config().Return(config.Config{
					Explorer: "test",
				}).Times(2)

				err := SendRaw(client, cmd, selp)
				require.NoError(err)
			})
		})
	}

	t.Run("failed to invoke SendAction api", func(t *testing.T) {
		expectedErr := errors.New("failed to invoke SendAction api")

		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		err = SendRaw(client, cmd, selp)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestSendAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	passwd := "123456"

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount(passwd)
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	elp := createEnvelope(0)
	cost, err := elp.Cost()
	require.NoError(err)
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      accAddr.String(),
		Nonce:        1,
		PendingNonce: 1,
		Balance:      cost.String(),
	}}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("action", config.English).Times(64)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {}).AnyTimes()
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {}).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(15)
	client.EXPECT().NewKeyStore().Return(ks).Times(15)

	t.Run("failed to get privateKey", func(t *testing.T) {
		expectedErr := errors.New("failed to get privateKey")
		client.EXPECT().ReadSecret().Return("", expectedErr).Times(1)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", "")
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), "", 0, false)
		require.Contains(err.Error(), expectedErr.Error())
	})

	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).AnyTimes()
	client.EXPECT().ReadSecret().Return(passwd, nil).Times(1)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).Times(7)
	client.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(10)
	client.EXPECT().ReadInput().Return("confirm", nil)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true, nil).Times(2)
	client.EXPECT().Config().Return(config.Config{
		Explorer: "iotexscan",
		Endpoint: "testnet1",
	}).Times(11)

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(6)
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil).Times(4)
	apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil).Times(4)

	t.Run("sends signed action to blockchain", func(t *testing.T) {
		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), passwd, 1, false)
		require.NoError(err)
	})

	t.Run("send action with nonce", func(t *testing.T) {
		mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, "hdw::1/2", passwd, 1, false)
		require.NoError(err)
	})

	t.Run("quit action command", func(t *testing.T) {
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false, nil)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), passwd, 1, false)
		require.NoError(err)
	})

	t.Run("failed to ask confirm", func(t *testing.T) {
		expectedErr := errors.New("failed to ask confirm")
		client.EXPECT().AskToConfirm(gomock.Any()).Return(false, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), passwd, 1, false)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to pass balance check", func(t *testing.T) {
		expectedErr := errors.New("failed to pass balance check")
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), passwd, 1, false)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get nonce", func(t *testing.T) {
		mnemonic := "lake stove quarter shove dry matrix hire split wide attract argue core"
		expectedErr := errors.New("failed to get nonce")
		client.EXPECT().HdwalletMnemonic(gomock.Any()).Return(mnemonic, nil)
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, "hdw::1/2", passwd, 1, false)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get chain meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get chain meta")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr)
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)

		cmd := NewActionCmd(client)
		RegisterWriteCommand(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--password", passwd)
		require.NoError(err)
		err = SendAction(client, cmd, elp, accAddr.String(), passwd, 1, false)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestRead(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	bytecode := "608060405234801561001057600080fd5b506040516040806108018339810180604052810190808051906020019092919080519060200190929190505050816004819055508060058190555050506107a58061005c6000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631249c58b1461007d57806327e235e31461009457806353277879146100eb5780636941b84414610142578063810ad50514610199578063a9059cbb14610223575b600080fd5b34801561008957600080fd5b50610092610270565b005b3480156100a057600080fd5b506100d5600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610475565b6040518082815260200191505060405180910390f35b3480156100f757600080fd5b5061012c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061048d565b6040518082815260200191505060405180910390f35b34801561014e57600080fd5b50610183600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104a5565b6040518082815260200191505060405180910390f35b3480156101a557600080fd5b506101da600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104bd565b604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390f35b34801561022f57600080fd5b5061026e600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610501565b005b436004546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054011115151561032a576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260108152602001807f746f6f20736f6f6e20746f206d696e740000000000000000000000000000000081525060200191505060405180910390fd5b436000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600554600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282540192505081905550600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600081548092919060010191905055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b46005546040518082815260200191505060405180910390a3565b60016020528060005260406000206000915090505481565b60026020528060005260406000206000915090505481565b60006020528060005260406000206000915090505481565b60036020528060005260406000206000915090508060000160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16908060010154905082565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101515156105b8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f696e73756666696369656e742062616c616e636500000000000000000000000081525060200191505060405180910390fd5b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555060408051908101604052803373ffffffffffffffffffffffffffffffffffffffff16815260200182815250600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101559050508173ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b4836040518082815260200191505060405180910390a350505600a165627a7a7230582047e5e1380e66d6b109548617ae59ff7baf70ee2d4a6734559b8fc5cabca0870b0029000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000186a0"

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("123456")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	elp := createEnvelope(0)
	cost, err := elp.Cost()
	require.NoError(err)
	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{
		Address:      accAddr.String(),
		Nonce:        1,
		PendingNonce: 1,
		Balance:      cost.String(),
	}}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("action", config.English).Times(4)
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {}).Times(2)
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {}).Times(2)
	client.EXPECT().APIServiceClient().Return(apiServiceClient, nil).Times(2)

	t.Run("reads smart contract on IoTeX blockchain", func(t *testing.T) {
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("test", nil)
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil)
		apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil)
		apiServiceClient.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return(&iotexapi.SendActionResponse{}, nil)
		apiServiceClient.EXPECT().ReadContract(gomock.Any(), gomock.Any()).Return(&iotexapi.ReadContractResponse{
			Data: "test",
		}, nil)

		cmd := NewActionCmd(client)
		registerSignerFlag(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--signer", "test")
		require.NoError(err)
		signer, err := cmd.Flags().GetString(signerFlagLabel)
		require.NoError(err)
		result, err := Read(client, cmd, accAddr, "100", []byte(bytecode), signer, 100)
		require.NoError(err)
		require.Equal("test", result)
	})

	t.Run("failed to get signer address", func(t *testing.T) {
		expectErr := errors.New("failed to get signer address")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectErr)
		cmd := NewActionCmd(client)
		registerSignerFlag(client, cmd)
		_, err := util.ExecuteCmd(cmd, "--signer", "test")
		require.NoError(err)
		signer, err := cmd.Flags().GetString(signerFlagLabel)
		require.NoError(err)
		_, err = Read(client, cmd, accAddr, "100", []byte(bytecode), signer, 100)
		require.Contains(err.Error(), expectErr.Error())
	})
}
