// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBasicHash(t *testing.T) {
	require := require.New(t)

	// basic hash test
	input := []byte("hello")
	hash := sha256.Sum256(input)
	hash = sha256.Sum256(hash[:])
	hello, _ := hex.DecodeString("9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50")
	require.Equal(hello, hash[:])
	t.Logf("sha256(sha256(\"hello\") = %x", hash)

	hash = blake2b.Sum256(input)
	hash = blake2b.Sum256(hash[:])
	hello, _ = hex.DecodeString("901c60ffffd77f743729f8fea0233c0b00223428b5192c2015f853562b45ce59")
	require.Equal(hello, hash[:])
	t.Logf("blake2b(blake2b(\"hello\") = %x", hash)
}

func TestMerkle(t *testing.T) {
	require := require.New(t)

	amount := uint64(50 << 22)
	// create testing transactions
	cbtsf0 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["producer"].RawAddress)
	require.NotNil(cbtsf0)
	cbtsf1 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["alfa"].RawAddress)
	require.NotNil(cbtsf1)
	cbtsf2 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbtsf2)
	cbtsf3 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(cbtsf3)
	cbtsf4 := action.NewCoinBaseTransfer(big.NewInt(int64(amount)), ta.Addrinfo["echo"].RawAddress)
	require.NotNil(cbtsf4)

	// verify tx hash
	hash0, e := hex.DecodeString("a6707cec4886bceb8bd7a4e970c1a30133196fea40b7d35c3934f71fafd6139a")
	require.NoError(e)
	actual := cbtsf0.Hash()
	require.Equal(hash0, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash1, e := hex.DecodeString("59fe0fd0dd0ab4c96a7129f2a0ffee47d2e44a9914e99f64d9a64fad9c020d89")
	require.NoError(e)
	actual = cbtsf1.Hash()
	require.Equal(hash1, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash2, e := hex.DecodeString("be6aff36dc22cde8bc240fa8fd2ac7fd1c7ddfadde1b8f9111d0028d4446316c")
	require.NoError(e)
	actual = cbtsf2.Hash()
	require.Equal(hash2, actual[:])
	//require.Equal(hash2, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash3, e := hex.DecodeString("2816ca41c060572ceb6d217d671528380f3d0d091efd8c967fb73c260db5f778")
	require.NoError(e)
	actual = cbtsf3.Hash()
	require.Equal(hash3, actual[:])
	t.Logf("actual hash = %x", actual[:])

	hash4, e := hex.DecodeString("519fbf378f5f44565fb6de829e37de1e407a4603ab64febb8db39fa4f3971182")
	require.NoError(e)
	actual = cbtsf4.Hash()
	require.Equal(hash4, actual[:])
	t.Logf("actual hash = %x", actual[:])

	// manually compute merkle root
	cat := append(hash0, hash1...)
	hash01 := blake2b.Sum256(cat)
	t.Logf("hash01 = %x", hash01)

	cat = append(hash2, hash3...)
	hash23 := blake2b.Sum256(cat)
	t.Logf("hash23 = %x", hash23)

	cat = append(hash4, hash4...)
	hash45 := blake2b.Sum256(cat)
	t.Logf("hash45 = %x", hash45)

	cat = append(hash01[:], hash23[:]...)
	hash03 := blake2b.Sum256(cat)
	t.Logf("hash03 = %x", hash03)

	cat = append(hash45[:], hash45[:]...)
	hash47 := blake2b.Sum256(cat)
	t.Logf("hash47 = %x", hash47)

	cat = append(hash03[:], hash47[:]...)
	hash07 := blake2b.Sum256(cat)
	t.Logf("hash07 = %x", hash07)

	// create block using above 5 tx and verify merkle
	block := NewBlock(
		0,
		0,
		hash.ZeroHash32B,
		testutil.TimestampNow(),
		[]action.Action{cbtsf0, cbtsf1, cbtsf2, cbtsf3, cbtsf4},
	)
	hash := block.TxRoot()
	require.Equal(hash07[:], hash[:])

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   101,
			},
			{Action: &iproto.ActionPb_Transfer{
				Transfer: &iproto.TransferPb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   102,
			},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   103,
			},
			{Action: &iproto.ActionPb_Vote{
				Vote: &iproto.VotePb{},
			},
				Version: version.ProtocolVersion,
				Nonce:   104,
			},
		},
	})

	blk.Header.txRoot = blk.TxRoot()

	raw, err := blk.Serialize()
	require.Nil(t, err)

	var newblk Block
	err = newblk.Deserialize(raw)
	require.Nil(t, err)

	blockBytes := blk.ByteStream()
	require.True(t, len(blockBytes) > 0)

	require.Equal(t, uint64(123456789), newblk.Header.height)

	require.Equal(t, uint64(101), newblk.Actions[0].Nonce())
	require.Equal(t, uint64(102), newblk.Actions[1].Nonce())

	require.Equal(t, uint64(103), newblk.Actions[2].Nonce())
	require.Equal(t, uint64(104), newblk.Actions[3].Nonce())
}

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{nil, ""}
	tsf1, err := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey))
	tsf2, err := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey))
	hash := tsf1.Hash()
	blk := NewBlock(1, 1, hash, testutil.TimestampNow(), []action.Action{tsf1, tsf2})
	blk.Header.Pubkey = ta.Addrinfo["producer"].PublicKey
	blkHash := blk.HashBlock()
	blk.Header.blockSig = crypto.EC283.Sign(ta.Addrinfo["producer"].PrivateKey, blkHash[:])
	require.Nil(val.Validate(blk, 0, hash, true))
	blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
	require.NotNil(val.Validate(blk, 0, hash, true))
}

func TestSignBlock(t *testing.T) {
	require := require.New(t)
	val := validator{nil, ""}
	tsf1, err := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey))
	tsf2, err := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey))
	hash := tsf1.Hash()
	blk := NewBlock(1, 3, hash, testutil.TimestampNow(), []action.Action{tsf1, tsf2})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.Nil(err)
	require.Nil(val.Validate(blk, 2, hash, true))
}

func TestWrongNonce(t *testing.T) {
	cfg := &config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := state.NewFactory(cfg, state.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	val := validator{sf, ""}

	// correct nonce
	coinbaseTsf := action.NewCoinBaseTransfer(Gen.BlockReward, ta.Addrinfo["producer"].RawAddress)
	tsf1, err := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey))
	hash := tsf1.Hash()
	blk := NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, tsf1})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	require.Nil(val.Validate(blk, 2, hash, true))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = ws.RunActions(1, []action.Action{tsf1})
	require.NoError(err)
	require.Nil(sf.Commit(ws))

	// low nonce
	tsf2, err := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, tsf1, tsf2})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Equal(ErrActionNonce, errors.Cause(err))

	vote, err := action.NewVote(1, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(vote, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, vote})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(ErrActionNonce, errors.Cause(err))

	// duplicate nonce
	tsf3, err := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf3, ta.Addrinfo["producer"].PrivateKey))
	tsf4, err := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf4, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, tsf3, tsf4})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(ErrActionNonce, errors.Cause(err))

	vote2, err := action.NewVote(2, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(vote2, ta.Addrinfo["producer"].PrivateKey))
	vote3, err := action.NewVote(2, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(vote3, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, vote2, vote3})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(ErrActionNonce, errors.Cause(err))

	// non consecutive nonce
	tsf5, err := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf5, ta.Addrinfo["producer"].PrivateKey))
	tsf6, err := action.NewTransfer(4, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf6, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, tsf5, tsf6})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(ErrActionNonce, errors.Cause(err))

	vote4, err := action.NewVote(2, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(vote4, ta.Addrinfo["producer"].PrivateKey))
	vote5, err := action.NewVote(4, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(vote5, ta.Addrinfo["producer"].PrivateKey))
	hash = tsf1.Hash()
	blk = NewBlock(cfg.Chain.ID, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, vote4, vote5})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(ErrActionNonce, errors.Cause(err))
}

func TestWrongCoinbaseTsf(t *testing.T) {
	cfg := &config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := state.NewFactory(cfg, state.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	val := validator{sf, ""}

	// no coinbase tsf
	coinbaseTsf := action.NewCoinBaseTransfer(Gen.BlockReward, ta.Addrinfo["producer"].RawAddress)
	tsf1, err := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	require.NoError(action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey))
	hash := tsf1.Hash()
	blk := NewBlock(1, 3, hash, testutil.TimestampNow(), []action.Action{tsf1})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)

	// extra coinbase transfer
	blk = NewBlock(1, 3, hash, testutil.TimestampNow(), []action.Action{coinbaseTsf, coinbaseTsf, tsf1})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)

	// no transfer
	blk = NewBlock(1, 3, hash, testutil.TimestampNow(), []action.Action{})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)
}

func TestWrongAddress(t *testing.T) {
	val := validator{}
	invalidRecipient := "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep"
	tsf, err := action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["producer"].RawAddress, invalidRecipient, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	blk1 := NewBlock(1, 3, hash.ZeroHash32B, testutil.TimestampNow(), []action.Action{tsf})
	err = val.verifyActions(blk1, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "failed to validate transfer recipient's address"))

	invalidVotee := "ioaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vote, err := action.NewVote(1, ta.Addrinfo["producer"].RawAddress, invalidVotee, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	blk2 := NewBlock(1, 3, hash.ZeroHash32B, testutil.TimestampNow(), []action.Action{vote})
	err = val.verifyActions(blk2, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "failed to validate votee's address"))

	invalidContract := "123"
	execution, err := action.NewExecution(ta.Addrinfo["producer"].RawAddress, invalidContract, 1, big.NewInt(1), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(t, err)
	blk3 := NewBlock(1, 3, hash.ZeroHash32B, testutil.TimestampNow(), []action.Action{execution})
	err = val.verifyActions(blk3, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "failed to validate contract's address"))
}

func TestCoinbaseTransferValidation(t *testing.T) {
	t.Skip("It is skipped because testnet_actions.yaml doesn't match the chain ID")
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.ID = 1
	chain := NewBlockchain(&cfg, InMemStateFactoryOption(), InMemDaoOption())
	require.NotNil(t, chain)
	require.NoError(t, chain.Start(ctx))
	defer require.NoError(t, chain.Stop(ctx))

	pk, err := keypair.DecodePublicKey(
		"1d1727028b1e9dac0cafa693edd8496297f5c3281924ec578c0526e7340f7180bfa5af059084c8b90954bf2802a0060e145bece9580f9021352eb112340186e68dc9bea4f7711707")
	require.NoError(t, err)
	sk, err := keypair.DecodePrivateKey(
		"29cf385adfc5b1a84bd7e778ea2c056b85c977771005d545e54100266e224fc276ed7101")
	require.NoError(t, err)
	pkHash := keypair.HashPubKey(pk)
	addr := address.New(cfg.Chain.ID, pkHash[:])
	iotxAddr := iotxaddress.Address{
		PublicKey:  pk,
		PrivateKey: sk,
		RawAddress: addr.IotxAddress(),
	}
	blk, err := chain.MintNewBlock(nil, &iotxAddr, nil, nil,
		"")
	require.NoError(t, err)
	validator := validator{}
	require.NoError(t, validator.verifyActions(blk, true))
}

func TestValidateSecretBlock(t *testing.T) {
	cfg := &config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := state.NewFactory(cfg, state.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.Nil(addCreatorToFactory(sf))

	idList := make([][]uint8, 0)
	delegates := []string{ta.Addrinfo["producer"].RawAddress}
	for i := 0; i < 20; i++ {
		pk, _, err := crypto.EC283.NewKeyPair()
		require.NoError(err)
		pkHash := keypair.HashPubKey(pk)
		addr := address.New(cfg.Chain.ID, pkHash[:])
		delegates = append(delegates, addr.IotxAddress())
	}

	for _, delegate := range delegates {
		idList = append(idList, iotxaddress.CreateID(delegate))
	}
	producerSK := crypto.DKG.SkGeneration()
	_, shares, witness, err := crypto.DKG.Init(producerSK, idList)
	require.NoError(err)

	secretProposals := make([]*action.SecretProposal, 0)
	for i, share := range shares {
		secretProposal, err := action.NewSecretProposal(uint64(i+1), delegates[0], delegates[i], share)
		require.NoError(err)
		secretProposals = append(secretProposals, secretProposal)
	}
	secretWitness, err := action.NewSecretWitness(uint64(22), delegates[0], witness)
	require.NoError(err)
	hash := secretProposals[0].Hash()
	blk := NewSecretBlock(1, 3, hash, testutil.TimestampNow(), secretProposals, secretWitness)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)

	val := validator{sf, delegates[1]}
	require.NoError(val.Validate(blk, 2, hash, false))

	// Falsify secret proposal
	dummySecretProposal, err := action.NewSecretProposal(2, delegates[0], delegates[1], []uint32{1, 2, 3, 4, 5})
	require.NoError(err)
	secretProposals[1] = dummySecretProposal
	blk = NewSecretBlock(1, 3, hash, testutil.TimestampNow(), secretProposals, secretWitness)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, false)
	require.Error(err)
	require.Equal(ErrDKGSecretProposal, errors.Cause(err))
}

func addCreatorToFactory(sf state.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = ws.LoadOrCreateAccountState(ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply); err != nil {
		return err
	}
	if _, err = ws.RunActions(0, nil); err != nil {
		return err
	}
	if err = sf.Commit(ws); err != nil {
		return err
	}
	return nil
}
