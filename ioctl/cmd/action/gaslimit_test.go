// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// setGasLimitFlag binds the shared _gasLimitFlag to a throwaway command, as a
// write command does, and sets it to v unless v is nil (flag not given).
func setGasLimitFlag(t *testing.T, v *uint64) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	RegisterWriteCommand(cmd) // resets the bound value to the flag default
	if v != nil {
		require.NoError(t, cmd.Flags().Set("gas-limit", strconv.FormatUint(*v, 10)))
	}
	t.Cleanup(func() {
		_gasLimitFlag.RegisterCommand(&cobra.Command{Use: "cleanup"})
	})
}

// stubEstimators replaces the node gas estimators for the duration of a test.
// Unset estimators fail the test if called.
func stubEstimators(
	t *testing.T,
	exec func(string, *action.Execution) (uint64, error),
	migrate func(string, *action.MigrateStake) (uint64, error),
	setCode func(common.Address, common.Address, *big.Int, []byte, []types.SetCodeAuthorization) (uint64, error),
) {
	t.Helper()
	oldExec, oldMigrate, oldSetCode := _estimateExecutionGas, _estimateMigrateStakeGas, _estimateSetCodeTxGas
	if exec == nil {
		exec = func(string, *action.Execution) (uint64, error) {
			t.Fatal("unexpected execution gas estimation")
			return 0, nil
		}
	}
	if migrate == nil {
		migrate = func(string, *action.MigrateStake) (uint64, error) {
			t.Fatal("unexpected migrate stake gas estimation")
			return 0, nil
		}
	}
	if setCode == nil {
		setCode = func(common.Address, common.Address, *big.Int, []byte, []types.SetCodeAuthorization) (uint64, error) {
			t.Fatal("unexpected SetCodeTx gas estimation")
			return 0, nil
		}
	}
	_estimateExecutionGas, _estimateMigrateStakeGas, _estimateSetCodeTxGas = exec, migrate, setCode
	t.Cleanup(func() {
		_estimateExecutionGas, _estimateMigrateStakeGas, _estimateSetCodeTxGas = oldExec, oldMigrate, oldSetCode
	})
}

// builderFromFlag returns an envelope builder set up the way the write
// commands do: with the gas limit taken from --gas-limit.
func builderFromFlag() *action.EnvelopeBuilder {
	return (&action.EnvelopeBuilder{}).
		SetNonce(1).
		SetGasPrice(big.NewInt(1)).
		SetGasLimit(_gasLimitFlag.Value().(uint64))
}

func u64(v uint64) *uint64 { return &v }

func TestGasLimitFlagDefault(t *testing.T) {
	r := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	RegisterWriteCommand(cmd)
	f := cmd.Flags().Lookup("gas-limit")
	r.NotNil(f)
	r.Equal("l", f.Shorthand)
	r.Equal("0", f.DefValue)
}

func TestResolveGasLimitNativeActions(t *testing.T) {
	caller := identityset.Address(1).String()
	recipient := identityset.Address(2).String()
	payload := []byte("hello")

	transfer := action.NewTransfer(big.NewInt(1), recipient, payload)
	claim := action.NewClaimFromRewardingFund(big.NewInt(1), nil, payload)
	transferOwnership, err := action.NewCandidateTransferOwnership(recipient, payload)
	require.NoError(t, err)

	for _, c := range []struct {
		name      string
		elp       func() action.Envelope
		intrinsic uint64
	}{
		{"transfer", func() action.Envelope { return builderFromFlag().SetAction(transfer).Build() }, action.TransferBaseIntrinsicGas + action.TransferPayloadGas*uint64(len(payload))},
		{"claim", func() action.Envelope { return builderFromFlag().SetAction(claim).Build() }, action.ClaimFromRewardingFundBaseGas + action.ClaimFromRewardingFundGasPerByte*uint64(len(payload))},
		// the payload gas used to be left out by `stake2 transferownership`
		{"candidate transfer ownership", func() action.Envelope { return builderFromFlag().SetAction(transferOwnership).Build() }, action.CandidateTransferOwnershipBaseIntrinsicGas +
			action.CandidateTransferOwnershipPayloadGas*uint64(len(payload))},
	} {
		t.Run(c.name+" uses intrinsic gas by default", func(t *testing.T) {
			stubEstimators(t, nil, nil, nil)
			setGasLimitFlag(t, nil)
			gas, err := resolveGasLimit(caller, c.elp())
			require.NoError(t, err)
			require.Equal(t, c.intrinsic, gas)
		})
		t.Run(c.name+" uses intrinsic gas for -l 0", func(t *testing.T) {
			stubEstimators(t, nil, nil, nil)
			setGasLimitFlag(t, u64(0))
			gas, err := resolveGasLimit(caller, c.elp())
			require.NoError(t, err)
			require.Equal(t, c.intrinsic, gas)
		})
		t.Run(c.name+" keeps an explicit gas limit", func(t *testing.T) {
			stubEstimators(t, nil, nil, nil)
			setGasLimitFlag(t, u64(20000000))
			gas, err := resolveGasLimit(caller, c.elp())
			require.NoError(t, err)
			require.Equal(t, uint64(20000000), gas)
		})
	}
}

func TestResolveGasLimitExecution(t *testing.T) {
	caller := identityset.Address(1).String()
	contract := identityset.Address(3).String()
	data := []byte{0xa9, 0x05, 0x9c, 0xbb}

	t.Run("estimated by the node by default", func(t *testing.T) {
		r := require.New(t)
		var gotCaller string
		var gotExec *action.Execution
		stubEstimators(t, func(c string, e *action.Execution) (uint64, error) {
			gotCaller, gotExec = c, e
			return 51234, nil
		}, nil, nil)
		setGasLimitFlag(t, nil)
		gas, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewExecution(contract, big.NewInt(7), data)).Build())
		r.NoError(err)
		r.Equal(uint64(51234), gas)
		r.Equal(caller, gotCaller)
		r.Equal(contract, gotExec.Contract())
		r.Equal(big.NewInt(7), gotExec.Amount())
		r.Equal(data, gotExec.Data())
	})

	t.Run("keeps an explicit gas limit without estimating", func(t *testing.T) {
		stubEstimators(t, nil, nil, nil)
		// an explicit value is sent as is, even above the per-tx cap
		setGasLimitFlag(t, u64(_maxTxGasLimit+1))
		gas, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewExecution(contract, big.NewInt(0), data)).Build())
		require.NoError(t, err)
		require.Equal(t, _maxTxGasLimit+1, gas)
	})

	t.Run("estimate at the cap is accepted", func(t *testing.T) {
		stubEstimators(t, func(string, *action.Execution) (uint64, error) { return _maxTxGasLimit, nil }, nil, nil)
		setGasLimitFlag(t, nil)
		gas, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewExecution(contract, big.NewInt(0), data)).Build())
		require.NoError(t, err)
		require.Equal(t, _maxTxGasLimit, gas)
	})

	t.Run("estimate above the cap is rejected", func(t *testing.T) {
		stubEstimators(t, func(string, *action.Execution) (uint64, error) { return _maxTxGasLimit + 1, nil }, nil, nil)
		setGasLimitFlag(t, nil)
		_, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewExecution("", big.NewInt(0), data)).Build())
		require.ErrorContains(t, err, "exceeds the per-transaction gas limit cap 16777216")
		require.ErrorContains(t, err, "--gas-limit")
	})

	t.Run("estimation error is returned", func(t *testing.T) {
		stubEstimators(t, func(string, *action.Execution) (uint64, error) { return 0, errors.New("execution reverted") }, nil, nil)
		setGasLimitFlag(t, nil)
		_, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewExecution(contract, big.NewInt(0), data)).Build())
		require.ErrorContains(t, err, "execution reverted")
	})
}

func TestResolveGasLimitMigrateStake(t *testing.T) {
	caller := identityset.Address(1).String()
	stubEstimators(t, nil, func(c string, ms *action.MigrateStake) (uint64, error) {
		require.Equal(t, caller, c)
		require.Equal(t, uint64(5), ms.BucketIndex())
		return 123456, nil
	}, nil)
	setGasLimitFlag(t, nil)
	gas, err := resolveGasLimit(caller, builderFromFlag().SetAction(action.NewMigrateStake(5)).Build())
	require.NoError(t, err)
	require.Equal(t, uint64(123456), gas)
}

func TestSetCodeTxBuilderGasLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	auths := []types.SetCodeAuthorization{{ChainID: *uint256.NewInt(4689), Address: to, Nonce: 3}}
	data := []byte{1, 2, 3}

	t.Run("estimated by the node by default", func(t *testing.T) {
		r := require.New(t)
		stubEstimators(t, nil, nil, func(f, tt common.Address, v *big.Int, d []byte, a []types.SetCodeAuthorization) (uint64, error) {
			r.Equal(from, f)
			r.Equal(to, tt)
			r.Equal(big.NewInt(9), v)
			r.Equal(data, d)
			r.Equal(auths, a)
			return 71234, nil
		})
		tx, err := SetCodeTxBuilder(to, big.NewInt(9), big.NewInt(1), 0, data, auths)(key, 0, big.NewInt(4689))
		r.NoError(err)
		r.Equal(uint64(71234), tx.Gas())
	})

	t.Run("keeps an explicit gas limit without estimating", func(t *testing.T) {
		stubEstimators(t, nil, nil, nil)
		tx, err := SetCodeTxBuilder(to, big.NewInt(9), big.NewInt(1), 90000, data, auths)(key, 0, big.NewInt(4689))
		require.NoError(t, err)
		require.Equal(t, uint64(90000), tx.Gas())
	})

	t.Run("estimate above the cap is rejected", func(t *testing.T) {
		stubEstimators(t, nil, nil, func(common.Address, common.Address, *big.Int, []byte, []types.SetCodeAuthorization) (uint64, error) {
			return _maxTxGasLimit + 1, nil
		})
		_, err := SetCodeTxBuilder(to, big.NewInt(9), big.NewInt(1), 0, data, auths)(key, 0, big.NewInt(4689))
		require.ErrorContains(t, err, "exceeds the per-transaction gas limit cap")
	})
}

func TestFetchEthEstimateGas(t *testing.T) {
	from := common.HexToAddress("0x0000000000000000000000000000000000000011")
	to := common.HexToAddress("0x0000000000000000000000000000000000000022")
	auths := []types.SetCodeAuthorization{{ChainID: *uint256.NewInt(4689), Address: to, Nonce: 3}}

	t.Run("sends the call and parses the hex result", func(t *testing.T) {
		r := require.New(t)
		var req struct {
			Method string `json:"method"`
			Params []struct {
				From              common.Address               `json:"from"`
				To                common.Address               `json:"to"`
				Value             string                       `json:"value"`
				Data              string                       `json:"data"`
				AuthorizationList []types.SetCodeAuthorization `json:"authorizationList"`
			} `json:"params"`
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, hr *http.Request) {
			body, err := io.ReadAll(hr.Body)
			r.NoError(err)
			r.NoError(json.Unmarshal(body, &req))
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0xb412"}`))
		}))
		defer srv.Close()
		gas, err := fetchEthEstimateGas(srv.URL, from, to, big.NewInt(255), []byte{0xab, 0xcd}, auths)
		r.NoError(err)
		r.Equal(uint64(46098), gas) // 0xb412
		r.Equal("eth_estimateGas", req.Method)
		r.Len(req.Params, 1)
		r.Equal(from, req.Params[0].From)
		r.Equal(to, req.Params[0].To)
		r.Equal("0xff", req.Params[0].Value)
		r.Equal("0xabcd", req.Params[0].Data)
		r.Equal(auths, req.Params[0].AuthorizationList)
	})

	t.Run("propagates rpc error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"execution reverted"}}`))
		}))
		defer srv.Close()
		_, err := fetchEthEstimateGas(srv.URL, from, to, nil, nil, auths)
		require.ErrorContains(t, err, "execution reverted")
	})

	t.Run("errors on non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()
		_, err := fetchEthEstimateGas(srv.URL, from, to, nil, nil, auths)
		require.Error(t, err)
	})
}
