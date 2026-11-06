// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// _osakaTestHeight stands in for the Osaka activation height, which rides
// ToBeEnabled until the hardfork is named
const _osakaTestHeight = 60_000_000

var (
	_p256VerifyAddr   = common.BytesToAddress([]byte{0x01, 0x00})
	_modExpAddr       = common.BytesToAddress([]byte{0x05})
	_iotexSecp256r1   = common.BytesToAddress([]byte{0x80, 0x01})
	_iotexScryptHash  = common.BytesToAddress([]byte{0x80, 0x02})
	_osakaTestHeights = []uint64{_osakaTestHeight - 1, _osakaTestHeight}
)

func osakaTestGenesis() genesis.Genesis {
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = _osakaTestHeight
	return g
}

func ioAddr(t *testing.T, a common.Address) string {
	addr, err := address.FromBytes(a.Bytes())
	require.NoError(t, err)
	return addr.String()
}

// run executes data against to with the given gas limit and returns the
// receipt and the return value, whatever the outcome
func (env *execEnv) run(to string, data []byte, gas uint64) (*action.Receipt, []byte) {
	require := require.New(env.t)
	env.nonce++
	elp := (&action.EnvelopeBuilder{}).SetNonce(env.nonce).SetGasPrice(big.NewInt(0)).
		SetGasLimit(gas).SetAction(action.NewExecution(to, big.NewInt(0), data)).Build()
	intrinsic, err := elp.IntrinsicGas()
	require.NoError(err)
	actx := protocol.WithActionCtx(env.ctx, protocol.ActionCtx{
		Caller:       env.caller,
		ActionHash:   hash.Hash256b(append(binary.BigEndian.AppendUint64(nil, env.nonce), data...)),
		Nonce:        env.nonce,
		IntrinsicGas: intrinsic,
	})
	ret, receipt, err := ExecuteContract(actx, env.sm, elp)
	require.NoError(err)
	return receipt, ret
}

// deployRuntime deploys a contract whose code is runtime and returns its address
func (env *execEnv) deployRuntime(runtime []byte) string {
	return env.exec(action.EmptyAddress, initCode(nil, runtime)).ContractAddress
}

// TestOsakaChainRules checks that getChainConfig turns go-ethereum's Osaka
// rules on exactly at the height FeatureCtx.EnableOsakaEVM does, that the EVM
// can be built under them with IoTeX's chain config (no blob schedule, no
// Ethereum fork ordering), and that the precompile set changes as expected.
func TestOsakaChainRules(t *testing.T) {
	r := require.New(t)
	g := osakaTestGenesis()
	for _, height := range _osakaTestHeights {
		isOsaka := height >= _osakaTestHeight
		env := newExecEnv(t, g, height)
		r.Equal(isOsaka, protocol.MustGetFeatureCtx(env.ctx).EnableOsakaEVM)

		elp := (&action.EnvelopeBuilder{}).SetGasLimit(100000).
			SetAction(action.NewExecution(action.EmptyAddress, big.NewInt(0), nil)).Build()
		ps, err := newParams(protocol.WithActionCtx(env.ctx, protocol.ActionCtx{Caller: env.caller}), elp)
		r.NoError(err)
		if isOsaka {
			r.NotNil(ps.chainConfig.OsakaTime)
			r.Equal(ps.context.Time, *ps.chainConfig.OsakaTime, "Osaka starts at the block of the activation height")
		} else {
			r.Nil(ps.chainConfig.OsakaTime)
		}
		r.Nil(ps.chainConfig.BlobScheduleConfig)
		evm := vm.NewEVM(ps.context, env.stateDB(), ps.chainConfig, ps.evmConfig)
		rules := ps.chainConfig.Rules(ps.context.BlockNumber, g.IsSumatra(height), ps.context.Time)
		r.True(rules.IsPrague)
		r.Equal(isOsaka, rules.IsOsaka, "height %d", height)
		r.Equal(isOsaka, evm.ChainConfig().IsOsaka(ps.context.BlockNumber, ps.context.Time))
		r.False(rules.IsAmsterdam)
		r.False(rules.IsVerkle)

		active := vm.ActivePrecompiles(rules)
		r.Equal(isOsaka, slices.Contains(active, _p256VerifyAddr), "0x100 is a precompile from Osaka on")
		r.True(slices.Contains(active, _iotexSecp256r1), "0x8001 stays")
		r.True(slices.Contains(active, _iotexScryptHash), "0x8002 stays")
		contracts := vm.ActivePrecompiledContracts(rules)
		prague := vm.ActivePrecompiledContracts(params.Rules{IsPrague: true})
		for _, a := range []common.Address{_iotexSecp256r1, _iotexScryptHash} {
			r.Equal(prague[a].Name(), contracts[a].Name())
		}
	}
}

// clzRuntime returns clz(calldata[0:32]) and the gas spent between two GAS
// opcodes around it (SWAP1 3 + CLZ + GAS 2), measured inside the EVM so the
// EIP-7623 data floor on the transaction does not hide it
var clzRuntime = []byte{
	0x60, 0x00, 0x35, // PUSH1 0 CALLDATALOAD  [x]
	0x5a,             // GAS                   [x g0]
	0x90,             // SWAP1                 [g0 x]
	0x1e,             // CLZ                   [g0 c]
	0x5a,             // GAS                   [g0 c g1]
	0x90,             // SWAP1                 [g0 g1 c]
	0x60, 0x00, 0x52, // MSTORE(0, c)          [g0 g1]
	0x90, 0x03, // SWAP1 SUB                   [g0-g1]
	0x60, 0x20, 0x52, // MSTORE(32, g0-g1)
	0x60, 0x40, 0x60, 0x00, 0xf3, // RETURN(0, 64)
}

// probeRuntime calls the precompile at addr with the calldata and returns
// [success][gas consumed by the call][return data]
func probeRuntime(addr common.Address) []byte {
	a := addr.Bytes()[18:]
	return []byte{
		0x36, 0x60, 0x00, 0x60, 0x00, 0x37, // CALLDATACOPY(0, 0, CALLDATASIZE)
		0x5a,                   // GAS                 [g0]
		0x60, 0x00, 0x60, 0x00, // retSize retOffset
		0x36, 0x60, 0x00, // argsSize argsOffset
		0x61, a[0], a[1], // PUSH2 addr
		0x5a,       // GAS
		0xfa,       // STATICCALL            [g0 ok]
		0x5a,       // GAS                   [g0 ok g1]
		0x82, 0x03, // DUP3 SUB              [g0 ok g0-g1]
		0x60, 0x20, 0x52, // MSTORE(32, used)
		0x60, 0x00, 0x52, // MSTORE(0, ok)
		0x3d, 0x60, 0x00, 0x60, 0x40, 0x3e, // RETURNDATACOPY(64, 0, RETURNDATASIZE)
		0x3d, 0x60, 0x40, 0x01, // size = RETURNDATASIZE + 64
		0x60, 0x00, 0xf3, // RETURN(0, size)
	}
}

func decodeProbe(t *testing.T, ret []byte) (bool, uint64, []byte) {
	require.GreaterOrEqual(t, len(ret), 64)
	return new(big.Int).SetBytes(ret[:32]).Sign() != 0, new(big.Int).SetBytes(ret[32:64]).Uint64(), ret[64:]
}

func word(v *big.Int) []byte {
	return common.LeftPadBytes(v.Bytes(), 32)
}

// TestOsakaCLZ runs CLZ (EIP-7939) through ExecuteContract one block before and
// at the activation height: an undefined opcode before, the leading zero count
// at 5 gas after.
func TestOsakaCLZ(t *testing.T) {
	r := require.New(t)
	g := osakaTestGenesis()
	for _, height := range _osakaTestHeights {
		isOsaka := height >= _osakaTestHeight
		env := newExecEnv(t, g, height)
		clz := env.deployRuntime(clzRuntime)
		for _, tc := range []struct {
			in   *big.Int
			want uint64
		}{
			{big.NewInt(0), 256},
			{big.NewInt(1), 255},
			{new(big.Int).Lsh(big.NewInt(1), 255), 0},
			{new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)), 0},
			{new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 248), big.NewInt(1)), 8},
		} {
			receipt, ret := env.run(clz, word(tc.in), 100000)
			if !isOsaka {
				r.EqualValues(100, receipt.Status, "CLZ is undefined before Osaka") // ErrUnknown: invalid opcode
				r.EqualValues(100000, receipt.GasConsumed, "an undefined opcode consumes all gas")
				r.Empty(ret)
				continue
			}
			r.EqualValues(1, receipt.Status)
			r.Len(ret, 64)
			r.Equal(word(new(big.Int).SetUint64(tc.want)), ret[:32], "clz(%x)", tc.in)
			r.Equal(word(big.NewInt(3+5+2)), ret[32:], "CLZ costs 5 gas")
		}
	}
}

func modexpInput(bl, el, ml int) []byte {
	in := make([]byte, 0, 96+bl+el+ml)
	for _, n := range []int{bl, el, ml} {
		in = append(in, word(big.NewInt(int64(n)))...)
	}
	b, e, m := make([]byte, bl), make([]byte, el), make([]byte, ml)
	if bl > 0 {
		b[bl-1] = 2
	}
	if el > 0 {
		e[el-1] = 3
	}
	if ml > 0 {
		m[ml-1] = 7
	}
	return append(append(append(in, b...), e...), m...)
}

// TestOsakaModExp runs MODEXP through a contract one block before and at the
// activation height: inputs with a length above 1024 bytes are rejected from
// Osaka on (EIP-7823), and the gas follows the EIP-7883 formula. The gas is
// compared with what go-ethereum's own Prague and Osaka MODEXP charge, so the
// test pins that IoTeX's EVM switches between the two at the fork.
func TestOsakaModExp(t *testing.T) {
	r := require.New(t)
	g := osakaTestGenesis()
	pragueModExp := vm.PrecompiledContractsPrague[_modExpAddr]
	osakaModExp := vm.PrecompiledContractsOsaka[_modExpAddr]
	cases := []struct {
		bl, el, ml int
		overBound  bool
	}{
		{1, 1, 1, false},
		{32, 32, 32, false},
		{1, 64, 1, false},
		{64, 1, 64, false},
		{1024, 1, 1, false},
		{1, 1024, 1, false},
		{1, 1, 1024, false},
		{1025, 1, 1, true},
		{1, 1025, 1, true},
		{1, 1, 1025, true},
	}
	used := map[uint64][]uint64{}
	for _, height := range _osakaTestHeights {
		isOsaka := height >= _osakaTestHeight
		env := newExecEnv(t, g, height)
		probe := env.deployRuntime(probeRuntime(_modExpAddr))
		for _, tc := range cases {
			in := modexpInput(tc.bl, tc.el, tc.ml)
			receipt, ret := env.run(probe, in, 10_000_000)
			r.EqualValues(1, receipt.Status)
			ok, gas, out := decodeProbe(t, ret)
			if isOsaka && tc.overBound {
				r.False(ok, "EIP-7823 rejects (%d,%d,%d)", tc.bl, tc.el, tc.ml)
				r.Empty(out)
			} else {
				r.True(ok, "(%d,%d,%d) at %d", tc.bl, tc.el, tc.ml, height)
				want := make([]byte, tc.ml)
				want[tc.ml-1] = 1 // 2^3 mod 7
				r.Equal(want, out)
			}
			used[height] = append(used[height], gas)
		}
	}
	for i, tc := range cases {
		if tc.overBound {
			continue
		}
		in := modexpInput(tc.bl, tc.el, tc.ml)
		// the call overhead is the same at both heights, so the difference is
		// the precompile's own price
		r.Equal(osakaModExp.RequiredGas(in)-pragueModExp.RequiredGas(in),
			used[_osakaTestHeight][i]-used[_osakaTestHeight-1][i], "(%d,%d,%d)", tc.bl, tc.el, tc.ml)
	}
	// anchor the formula itself: the minimum price goes from 200 to 500
	r.EqualValues(200, pragueModExp.RequiredGas(modexpInput(1, 1, 1)))
	r.EqualValues(500, osakaModExp.RequiredGas(modexpInput(1, 1, 1)))
}

type p256Vector struct {
	Input    string
	Expected string
	Name     string
}

func loadP256Vectors(t *testing.T) []p256Vector {
	b, err := os.ReadFile("../testdata-osaka/p256verify_vectors.json")
	require.NoError(t, err)
	var vs []p256Vector
	require.NoError(t, json.Unmarshal(b, &vs))
	require.NotEmpty(t, vs)
	return vs
}

// TestOsakaP256Verify runs the EIP-7951 test vectors against 0x100 one block
// before and at the activation height. Before Osaka 0x100 is an ordinary empty
// account: every call succeeds and returns nothing. From Osaka on it is
// P256VERIFY: a valid signature returns 32-byte 1, anything else (bad
// signature, point off the curve or out of range, wrong input length) returns
// nothing, all for 6900 gas.
func TestOsakaP256Verify(t *testing.T) {
	r := require.New(t)
	g := osakaTestGenesis()
	vectors := loadP256Vectors(t)
	var valid, invalid int
	for _, height := range _osakaTestHeights {
		isOsaka := height >= _osakaTestHeight
		env := newExecEnv(t, g, height)
		probe := env.deployRuntime(probeRuntime(_p256VerifyAddr))
		var gasUsed []uint64
		for _, v := range vectors {
			in, err := hex.DecodeString(v.Input)
			r.NoError(err)
			receipt, ret := env.run(probe, in, 2_000_000)
			r.EqualValues(1, receipt.Status, v.Name)
			ok, gas, out := decodeProbe(t, ret)
			r.True(ok, v.Name)
			if !isOsaka {
				r.Empty(out, v.Name)
				continue
			}
			r.Equal(v.Expected, hex.EncodeToString(out), v.Name)
			if len(out) > 0 {
				valid++
			} else {
				invalid++
			}
			gasUsed = append(gasUsed, gas)
		}
		if isOsaka {
			// 6900 for the precompile plus the same call overhead every time
			r.Len(slices.Compact(slices.Sorted(slices.Values(gasUsed))), 1)
		}
	}
	r.Positive(valid)
	r.Positive(invalid)

	// inputs the vector file does not cover: wrong lengths and IoTeX's 0x8001
	// format, which P256VERIFY treats as invalid rather than erroring
	env := newExecEnv(t, g, _osakaTestHeight)
	probe := env.deployRuntime(probeRuntime(_p256VerifyAddr))
	validIn, err := hex.DecodeString(vectors[0].Input)
	r.NoError(err)
	r.Equal("0000000000000000000000000000000000000000000000000000000000000001", vectors[0].Expected)
	for _, in := range [][]byte{nil, validIn[:159], append(slices.Clone(validIn), 0)} {
		receipt, ret := env.run(probe, in, 2_000_000)
		r.EqualValues(1, receipt.Status)
		ok, _, out := decodeProbe(t, ret)
		r.True(ok)
		r.Empty(out, "input of %d bytes", len(in))
	}
}

// TestOsakaIotexPrecompiles checks that IoTeX's precompiles 0x8001 (secp256r1)
// and 0x8002 (scrypt) behave at the Osaka activation height exactly as one
// block before: same success, same output, same gas, for go-ethereum fork's own
// valid and failing test vectors.
func TestOsakaIotexPrecompiles(t *testing.T) {
	r := require.New(t)
	g := osakaTestGenesis()
	b, err := os.ReadFile("../testdata-osaka/iotex_precompile_vectors.json")
	r.NoError(err)
	var vectors []struct {
		Address       string
		Name          string
		Input         string
		Expected      string
		ExpectedError string
	}
	r.NoError(json.Unmarshal(b, &vectors))
	r.NotEmpty(vectors)
	type result struct {
		status uint64
		ret    string
	}
	got := map[uint64][]result{}
	for _, height := range _osakaTestHeights {
		env := newExecEnv(t, g, height)
		probes := map[string]string{
			"8001": env.deployRuntime(probeRuntime(_iotexSecp256r1)),
			"8002": env.deployRuntime(probeRuntime(_iotexScryptHash)),
		}
		for _, v := range vectors {
			in, err := hex.DecodeString(v.Input)
			r.NoError(err)
			receipt, ret := env.run(probes[v.Address], in, 1_000_000)
			r.EqualValues(1, receipt.Status, v.Name)
			ok, _, out := decodeProbe(t, ret)
			if v.ExpectedError != "" {
				r.False(ok, "%s %s", v.Address, v.Name)
			} else {
				r.True(ok, "%s %s", v.Address, v.Name)
				r.Equal(v.Expected, hex.EncodeToString(out), "%s %s", v.Address, v.Name)
			}
			got[height] = append(got[height], result{receipt.Status, hex.EncodeToString(ret)})
		}
	}
	r.Equal(got[_osakaTestHeight-1], got[_osakaTestHeight])
}
