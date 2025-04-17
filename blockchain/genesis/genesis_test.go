// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

func TestDefaultConfig(t *testing.T) {
	// construct a config without overriding
	cfg, err := New("")
	require.NoError(t, err)
	// Validate blockchain
	assert.Equal(t, Default.BlockGasLimit, cfg.BlockGasLimit)
	assert.Equal(t, Default.ActionGasLimit, cfg.ActionGasLimit)
	assert.Equal(t, Default.NumSubEpochs, cfg.NumSubEpochs)
	assert.Equal(t, Default.NumDelegates, cfg.NumDelegates)
	// Validate rewarding protocol)
	assert.Equal(t, Default.BlockReward(), cfg.BlockReward())
	assert.Equal(t, Default.EpochReward(), cfg.EpochReward())
	assert.Equal(t, Default.FoundationBonus(), cfg.FoundationBonus())
}
func TestHash(t *testing.T) {
	require := require.New(t)
	cfg, err := New("")
	require.NoError(err)
	cfg.Timestamp = 1553558500
	cfg.NumSubEpochs = 15
	cfg.TimeBasedRotation = true
	cfg.InitBalanceMap = map[string]string{
		"io1uqhmnttmv0pg8prugxxn7d8ex9angrvfjfthxa": "9800000000000000000000000000",
		"io1v3gkc49d5vwtdfdka2ekjl3h468egun8e43r7z": "100000000000000000000000000",
		"io1vrl48nsdm8jaujccd9cx4ve23cskr0ys6urx92": "100000000000000000000000000",
		"io1llupp3n8q5x8usnr5w08j6hc6hn55x64l46rr7": "100000000000000000000000000",
		"io1ns7y0pxmklk8ceattty6n7makpw76u770u5avy": "100000000000000000000000000",
		"io1xuavja5dwde8pvy4yms06yyncad4yavghjhwra": "100000000000000000000000000",
		"io1cdqx6p5rquudxuewflfndpcl0l8t5aezen9slr": "100000000000000000000000000",
		"io1hh97f273nhxcq8ajzcpujtt7p9pqyndfmavn9r": "100000000000000000000000000",
		"io1yhvu38epz5vmkjaclp45a7t08r27slmcc0zjzh": "100000000000000000000000000",
		"io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng": "100000000000000000000000000",
		"io1skmqp33qme8knyw0fzgt9takwrc2nvz4sevk5c": "100000000000000000000000000",
		"io1fxzh50pa6qc6x5cprgmgw4qrp5vw97zk5pxt3q": "100000000000000000000000000",
		"io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2": "100000000000000000000000000",
		"io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd": "100000000000000000000000000",
		"io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj": "100000000000000000000000000",
		"io1ed52svvdun2qv8sf2m0xnynuxfaulv6jlww7ur": "100000000000000000000000000",
		"io158hyzrmf4a8xll7gfc8xnwlv70jgp44tzy5nvd": "100000000000000000000000000",
		"io19kshh892255x4h5ularvr3q3al2v8cgl80fqrt": "100000000000000000000000000",
		"io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02": "100000000000000000000000000",
		"io1znka733xefxjjw2wqddegplwtefun0mfdmz7dw": "100000000000000000000000000",
		"io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv": "100000000000000000000000000",
		"io14gnqxf9dpkn05g337rl7eyt2nxasphf5m6n0rd": "100000000000000000000000000",
		"io1l3wc0smczyay8xq747e2hw63mzg3ctp6uf8wsg": "100000000000000000000000000",
		"io1q4tdrahguffdu4e9j9aj4f38p2nee0r9vlhx7s": "100000000000000000000000000",
		"io1k9y4a9juk45zaqwvjmhtz6yjc68twqds4qcvzv": "100000000000000000000000000",
		"io15flratm0nhh5xpxz2lznrrpmnwteyd86hxdtj0": "100000000000000000000000000",
		"io1eq4ehs6xx6zj9gcsax7h3qydwlxut9xcfcjras": "100000000000000000000000000",
		"io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he": "100000000000000000000000000",
	}
	cfg.GravityChainStartHeight = 7614500
	cfg.RegisterContractAddress = "0x95724986563028deb58f15c5fac19fa09304f32d"
	cfg.StakingContractAddress = "0x87c9dbff0016af23f5b1ab9b8e072124ab729193"
	cfg.VoteThreshold = "100000000000000000000"
	cfg.ScoreThreshold = "2000000000000000000000000"
	cfg.SelfStakingThreshold = "1200000000000000000000000"
	cfg.ProductivityThreshold = 85
	hash := cfg.Hash()
	require.Equal("b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b", hex.EncodeToString(hash[:]))
}
func TestAccount_InitBalances(t *testing.T) {
	require := require.New(t)
	InitBalanceMap := make(map[string]string, 0)
	InitBalanceMap["io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"] = "1"
	InitBalanceMap["io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"] = "2"
	acc := Account{InitBalanceMap: InitBalanceMap}
	adds, balances := acc.InitBalances()
	require.Equal("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", adds[0].String())
	require.Equal("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", adds[1].String())
	require.Equal(InitBalanceMap["io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6"], balances[0].Text(10))
	require.Equal(InitBalanceMap["io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms"], balances[1].Text(10))
}

func TestTsunamiBlockGasLimit(t *testing.T) {
	r := require.New(t)

	cfg := Default
	for _, v := range []struct {
		height, gasLimit uint64
	}{
		{1, 20000000},
		{cfg.TsunamiBlockHeight - 1, 20000000},
		{cfg.TsunamiBlockHeight, 50000000},
		{cfg.WakeBlockHeight - 1, 50000000},
		{cfg.WakeBlockHeight, 30000000},
		{cfg.ToBeEnabledBlockHeight, 30000000},
	} {
		r.Equal(v.gasLimit, cfg.BlockGasLimitByHeight(v.height))
	}
}

func TestDeployerWhitelist(t *testing.T) {
	r := require.New(t)

	cases := []struct {
		addr   string
		expect bool
	}{
		{"0x3fab184622dc19b6109349b94811493bf2a45362", true},
		{"io18743s33zmsvmvyynfxu5sy2f80e2g5mzk3y5ue", true},
		{"0x3fab184622dc19b6109349b94811493bf2a45361", false},
		{"io18743s33zmsvmvyynfxu5sy2f80e2g5mpcz3zjx", false},
	}
	runTest := func(g *Genesis) {
		var (
			addr address.Address
			err  error
		)
		for _, c := range cases {
			if c.addr[:2] == "0x" {
				addr, err = address.FromHex(c.addr)
				r.NoError(err)
			} else {
				addr, err = address.FromString(c.addr)
				r.NoError(err)
			}
			r.Equal(c.expect, g.IsDeployerWhitelisted(addr))
		}
	}
	t.Run("0x address", func(t *testing.T) {
		runTest(&Default)
	})
	t.Run("io address", func(t *testing.T) {
		g := Default
		g.ReplayDeployerWhitelist = []string{"io18743s33zmsvmvyynfxu5sy2f80e2g5mzk3y5ue"}
		runTest(&g)
	})
}

func TestWakeBlockReward(t *testing.T) {
	r := require.New(t)
	four, five := unit.ConvertIotxToRau(4), unit.ConvertIotxToRau(5)
	four.Add(four, five)
	four.Rsh(four, 1)
	wake := Default.WakeBlockReward()
	// wake block reward = 4.8, between 4.5 and 5
	r.Equal(1, wake.Cmp(four))
	r.Equal(-1, wake.Cmp(five))
}
