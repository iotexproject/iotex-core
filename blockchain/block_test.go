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
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state/factory"
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

	producer := ta.Addrinfo["producer"]
	amount := uint64(50 << 22)
	// create testing transactions
	cbtsf0 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), producer.RawAddress)
	require.NotNil(cbtsf0)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(producer.RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbtsf0).Build()
	selp0, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf1 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.Addrinfo["alfa"].RawAddress)
	require.NotNil(cbtsf1)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["alfa"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbtsf1).Build()
	selp1, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf2 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.Addrinfo["bravo"].RawAddress)
	require.NotNil(cbtsf2)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["bravo"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbtsf2).Build()
	selp2, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf3 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.Addrinfo["charlie"].RawAddress)
	require.NotNil(cbtsf3)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["charlie"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbtsf3).Build()
	selp3, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	cbtsf4 := action.NewCoinBaseTransfer(1, big.NewInt(int64(amount)), ta.Addrinfo["echo"].RawAddress)
	require.NotNil(cbtsf4)
	bd = action.EnvelopeBuilder{}
	elp = bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["echo"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbtsf4).Build()
	selp4, err := action.Sign(elp, producer.RawAddress, producer.PrivateKey)
	require.NoError(err)

	// verify tx hash
	hash0, e := hex.DecodeString("af7b31e8c68328aa07b4dce318690a898b9d065417b17bdf3482347aed06a5e5")
	require.NoError(e)
	actual := cbtsf0.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash0, actual[:])

	hash1, e := hex.DecodeString("228a88945e1ffd3cd5f5733443c07e3818ae3e6a13de76ee395529855a082c6e")
	require.NoError(e)
	actual = cbtsf1.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash1, actual[:])

	hash2, e := hex.DecodeString("ed55b82760b15a11dc3eb91da7f571b2b4ad7fcb8b9239cc5af6e0bc4608854e")
	require.NoError(e)
	actual = cbtsf2.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash2, actual[:])

	hash3, e := hex.DecodeString("4dc973538baf632f14d5e85a84a51ebed191bc229fc3c0e4e1c84a515e09a9c4")
	require.NoError(e)
	actual = cbtsf3.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash3, actual[:])

	hash4, e := hex.DecodeString("16f37c7de145e31a8fe00f58b04a9ed05169ec5514fb43eff0390c1dc9b06d98")
	require.NoError(e)
	actual = cbtsf4.Hash()
	t.Logf("actual hash = %x", actual[:])
	require.Equal(hash4, actual[:])

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
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{selp0, selp1, selp2, selp3, selp4},
	)
	hash := block.CalculateTxRoot()
	require.Equal(hash07[:], hash[:])

	t.Log("Merkle root match pass\n")
}

func TestConvertFromBlockPb(t *testing.T) {
	blk := Block{}
	sender := ta.Addrinfo["producer"]
	require.NoError(t, blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        102,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
				Sender:       sender.RawAddress,
				SenderPubKey: sender.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        104,
			},
		},
	}))

	blk.Header.txRoot = blk.CalculateTxRoot()

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

	require.Equal(t, blk.Header.txRoot, blk.TxRoot())
	require.Equal(t, blk.Header.stateRoot, blk.StateRoot())
}

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["alfa"], 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	hash := tsf1.Hash()
	blk := NewBlock(1, 1, hash, testutil.TimestampNow(), ta.Addrinfo["producer"].PublicKey, []action.SealedEnvelope{tsf1, tsf2})
	blk.Header.Pubkey = ta.Addrinfo["producer"].PublicKey
	blkHash := blk.HashBlock()
	blk.Header.blockSig = crypto.EC283.Sign(ta.Addrinfo["producer"].PrivateKey, blkHash[:])
	require.Nil(val.Validate(blk, 0, hash, true))
	blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
	require.NotNil(val.Validate(blk, 0, hash, true))
}

func TestSignBlock(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["alfa"], 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	hash := tsf1.Hash()
	blk := NewBlock(1, 3, hash, testutil.TimestampNow(), ta.Addrinfo["producer"].PublicKey, []action.SealedEnvelope{tsf1, tsf2})
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.Nil(err)
	require.Nil(val.Validate(blk, 2, hash, true))
}

func TestWrongNonce(t *testing.T) {
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))

	val := validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	// correct nonce
	cbTsf := action.NewCoinBaseTransfer(1, Gen.BlockReward, ta.Addrinfo["producer"].RawAddress)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["producer"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(cbTsf).Build()
	cbselp, err := action.Sign(elp, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].PrivateKey)
	require.NoError(err)

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["alfa"], 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	hash := tsf1.Hash()
	blk := NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, tsf1},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	require.Nil(val.Validate(blk, 2, hash, true))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{tsf1})
	require.NoError(err)
	require.Nil(sf.Commit(ws))

	// low nonce
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, tsf1, tsf2},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote, err := testutil.SignedVote(ta.Addrinfo["producer"], ta.Addrinfo["producer"], 1, uint64(100000), big.NewInt(10))
	require.NoError(err)

	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, vote},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// duplicate nonce
	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, tsf3, tsf4},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote2, err := testutil.SignedVote(ta.Addrinfo["producer"], ta.Addrinfo["producer"], 2, uint64(100000), big.NewInt(10))
	require.NoError(err)

	vote3, err := testutil.SignedVote(ta.Addrinfo["producer"], ta.Addrinfo["producer"], 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, vote2, vote3},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// non consecutive nonce
	tsf5, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, tsf5, tsf6},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote4, err := testutil.SignedVote(ta.Addrinfo["producer"], ta.Addrinfo["producer"], 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	vote5, err := testutil.SignedVote(ta.Addrinfo["producer"], ta.Addrinfo["producer"], 4, uint64(100000), big.NewInt(10))
	require.NoError(err)

	hash = tsf1.Hash()
	blk = NewBlock(
		cfg.Chain.ID,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cbselp, vote4, vote5},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestWrongCoinbaseTsf(t *testing.T) {
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))

	val := validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol())

	// no coinbase tsf
	coinbaseTsf := action.NewCoinBaseTransfer(1, Gen.BlockReward, ta.Addrinfo["producer"].RawAddress)
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["producer"].RawAddress).
		SetGasLimit(protocol.GasLimit).
		SetAction(coinbaseTsf).Build()
	cb, err := action.Sign(elp, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].PrivateKey)
	require.NoError(err)

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["alfa"], 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	hash := tsf1.Hash()
	blk := NewBlock(
		1,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{tsf1},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)

	// extra coinbase transfer
	blk = NewBlock(
		1,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{cb, cb, tsf1},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers in block"),
	)

	// no transfer
	blk = NewBlock(
		1,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{},
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)
}

func TestWrongAddress(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()
	val := validator{sf: bc.GetFactory(), validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	invalidRecipient := "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep"
	tsf, err := action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["producer"].RawAddress, invalidRecipient, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidRecipient).Build()
	selp, err := action.Sign(elp, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].PrivateKey)
	require.NoError(t, err)
	blk1 := NewBlock(
		1,
		3,
		hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{selp},
	)
	err = val.verifyActions(blk1, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating recipient's address"))

	invalidVotee := "ioaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vote, err := action.NewVote(1, ta.Addrinfo["producer"].RawAddress, invalidVotee, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidVotee).Build()
	selp, err = action.Sign(elp, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].PrivateKey)
	require.NoError(t, err)
	blk2 := NewBlock(
		1,
		3,
		hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{selp},
	)
	err = val.verifyActions(blk2, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating votee's address"))

	invalidContract := "123"
	execution, err := action.NewExecution(ta.Addrinfo["producer"].RawAddress, invalidContract, 1, big.NewInt(1), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(execution).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidContract).Build()
	selp, err = action.Sign(elp, ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["producer"].PrivateKey)
	require.NoError(t, err)
	blk3 := NewBlock(
		1,
		3,
		hash.ZeroHash32B,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		[]action.SealedEnvelope{selp},
	)
	err = val.verifyActions(blk3, true)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating contract's address"))
}

func TestCoinbaseTransferValidation(t *testing.T) {
	t.Skip("It is skipped because testnet_actions.yaml doesn't match the chain ID")
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.ID = 1
	chain := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
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
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
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
	blk := NewSecretBlock(
		1,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		secretProposals,
		secretWitness,
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)

	val := validator{sf: sf, validatorAddr: delegates[1]}
	require.NoError(val.Validate(blk, 2, hash, false))

	// Falsify secret proposal
	dummySecretProposal, err := action.NewSecretProposal(2, delegates[0], delegates[1], []uint32{1, 2, 3, 4, 5})
	require.NoError(err)
	secretProposals[1] = dummySecretProposal
	blk = NewSecretBlock(
		1,
		3,
		hash,
		testutil.TimestampNow(),
		ta.Addrinfo["producer"].PublicKey,
		secretProposals,
		secretWitness,
	)
	err = blk.SignBlock(ta.Addrinfo["producer"])
	require.NoError(err)
	err = val.Validate(blk, 2, hash, false)
	require.Error(err)
	require.Equal(ErrDKGSecretProposal, errors.Cause(err))
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = account.LoadOrCreateAccount(ws, ta.Addrinfo["producer"].RawAddress, Gen.TotalSupply); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
