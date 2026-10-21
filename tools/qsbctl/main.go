// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// qsbctl drives quantum-safe staking buckets (IIP draft #86) against a node's
// gRPC API: derive guard keys, create and list buckets, bind a guard, submit
// guarded operations as a sponsor, query the guard record, and send legacy
// staking actions to see them rejected on a guarded bucket.
//
// It is a test tool for local and nightly networks. Keys are passed on the
// command line; do not use it with keys that hold real funds.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

const usage = `qsbctl — quantum-safe staking bucket test tool

Usage: qsbctl [-endpoint host:port] <command> [flags]

Commands:
  account   [-n N]                                   generate secp256k1 test accounts (key, address)
  key       -seed HEX [-account N -index N]          print the guard public key (QSB-KDF-v1)
  stake     -key HEX -candidate NAME -amount IOTX    create a native bucket (duration 0, no auto-stake)
  buckets   -owner ADDR                              list native buckets of an owner
  bind      -key HEX -bucket N -seed HEX [-index N]  bind a guard (sent by the bucket owner)
  guard     -bucket N [-height H]                    show the guard record
  op        <unstake|withdraw|transfer|change-candidate|restake|rotate>
            -sponsor HEX -bucket N -seed HEX [-index N]
            [-recipient ADDR] [-new-owner ADDR] [-clear] [-new-seed HEX -new-index N]
            [-candidate NAME] [-duration DAYS -auto-stake] [-deadline-blocks N]
  legacy    <unstake|withdraw|change-candidate|transfer|restake>
            -key HEX -bucket N [-candidate NAME] [-new-owner ADDR] [-duration DAYS]

HEX keys are secp256k1 private keys; -seed is the BIP-39 seed (hex) guard keys are derived from.
`

type client struct {
	api     iotexapi.APIServiceClient
	chainID uint32
}

func main() {
	endpoint := flag.String("endpoint", "127.0.0.1:14120", "node gRPC endpoint")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	cmd, args := flag.Arg(0), flag.Args()[1:]
	switch cmd {
	case "key":
		check(cmdKey(args))
		return
	case "account":
		check(cmdAccount(args))
		return
	}
	conn, err := grpc.NewClient(*endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	check(err)
	defer conn.Close()
	c := &client{api: iotexapi.NewAPIServiceClient(conn)}
	meta, err := c.api.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
	check(err)
	c.chainID = meta.GetChainMeta().GetChainID()

	switch cmd {
	case "stake":
		err = c.cmdStake(args)
	case "buckets":
		err = c.cmdBuckets(args)
	case "bind":
		err = c.cmdBind(args)
	case "guard":
		err = c.cmdGuard(args)
	case "op":
		err = c.cmdOp(args)
	case "legacy":
		err = c.cmdLegacy(args)
	default:
		flag.Usage()
		os.Exit(2)
	}
	check(err)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ---- keys ----

func guardKey(seedHex string, account, index uint) (*signer.PrivateKey, error) {
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) < 16 {
		return nil, fmt.Errorf("-seed must be at least 16 bytes of hex")
	}
	return signer.DeriveQSBKey(seed, uint32(account), uint32(index))
}

func ecdsaKey(h string) (crypto.PrivateKey, error) {
	if h == "" {
		return nil, fmt.Errorf("missing secp256k1 key")
	}
	return crypto.HexStringToPrivateKey(h)
}

func cmdAccount(args []string) error {
	fs := flag.NewFlagSet("account", flag.ExitOnError)
	n := fs.Int("n", 1, "number of accounts")
	check(fs.Parse(args))
	for i := 0; i < *n; i++ {
		sk, err := crypto.GenerateKey()
		if err != nil {
			return err
		}
		fmt.Printf("%s %s\n", sk.HexString(), sk.PublicKey().Address())
	}
	return nil
}

func cmdKey(args []string) error {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	seed := fs.String("seed", "", "BIP-39 seed (hex)")
	account := fs.Uint("account", 0, "account")
	index := fs.Uint("index", 0, "key index")
	check(fs.Parse(args))
	k, err := guardKey(*seed, *account, *index)
	if err != nil {
		return err
	}
	fmt.Printf("alg: %s\npublic key: %x\n", k.PublicKey().Alg(), k.PublicKey().Bytes())
	return nil
}

// ---- chain access ----

func (c *client) send(sk crypto.PrivateKey, payload interface {
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}) (*iotextypes.Receipt, error) {
	ctx := context.Background()
	if err := payload.SanityCheck(); err != nil {
		return nil, err
	}
	acc, err := c.api.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: sk.PublicKey().Address().String()})
	if err != nil {
		return nil, err
	}
	gp, err := c.api.SuggestGasPrice(ctx, &iotexapi.SuggestGasPriceRequest{})
	if err != nil {
		return nil, err
	}
	gas, err := payload.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	elp := (&action.EnvelopeBuilder{}).SetNonce(acc.GetAccountMeta().GetPendingNonce()).
		SetGasLimit(gas + 10_000).SetGasPrice(new(big.Int).SetUint64(gp.GetGasPrice())).
		SetChainID(c.chainID).SetAction(payload).Build()
	selp, err := action.Sign(elp, sk)
	if err != nil {
		return nil, err
	}
	resp, err := c.api.SendAction(ctx, &iotexapi.SendActionRequest{Action: selp.Proto()})
	if err != nil {
		return nil, err
	}
	fmt.Printf("sent %T from %s, action %s, gas limit %d\n", payload, sk.PublicKey().Address(), resp.GetActionHash(), gas+10_000)
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		r, err := c.api.GetReceiptByAction(ctx, &iotexapi.GetReceiptByActionRequest{ActionHash: resp.GetActionHash()})
		if err != nil {
			continue
		}
		rc := r.GetReceiptInfo().GetReceipt()
		fmt.Printf("receipt: height %d, status %s (%d), gas used %d\n",
			rc.GetBlkHeight(), iotextypes.ReceiptStatus(rc.GetStatus()), rc.GetStatus(), rc.GetGasConsumed())
		return rc, nil
	}
	return nil, fmt.Errorf("no receipt for %s after 2 minutes", resp.GetActionHash())
}

func (c *client) readStaking(method iotexapi.ReadStakingDataMethod_Name, req *iotexapi.ReadStakingDataRequest, height string, out proto.Message) error {
	m, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{Method: method})
	if err != nil {
		return err
	}
	a, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := c.api.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"), MethodName: m, Arguments: [][]byte{a}, Height: height,
	})
	if err != nil {
		return err
	}
	return proto.Unmarshal(resp.GetData(), out)
}

func (c *client) guard(index uint64, height string) (*iotextypes.BucketGuard, error) {
	g := &iotextypes.BucketGuard{}
	err := c.readStaking(iotexapi.ReadStakingDataMethod_BUCKET_GUARD, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketGuard_{BucketGuard: &iotexapi.ReadStakingDataRequest_BucketGuard{Index: index}},
	}, height, g)
	return g, err
}

func (c *client) bucket(index uint64) (*iotextypes.VoteBucket, error) {
	l := &iotextypes.VoteBucketList{}
	if err := c.readStaking(iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{Index: []uint64{index}}},
	}, "", l); err != nil {
		return nil, err
	}
	for _, b := range l.GetBuckets() {
		if b.GetIndex() == index {
			return b, nil
		}
	}
	return nil, fmt.Errorf("bucket %d not found", index)
}

func (c *client) tip() (uint64, error) {
	meta, err := c.api.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
	return meta.GetChainMeta().GetHeight(), err
}

func printBucket(b *iotextypes.VoteBucket) {
	fmt.Printf("bucket %d: owner %s candidate %s amount %s IOTX duration %dd autoStake %v unstaked %v\n",
		b.GetIndex(), b.GetOwner(), b.GetCandidateAddress(), iotx(b.GetStakedAmount()), b.GetStakedDuration(), b.GetAutoStake(),
		b.GetUnstakeStartTime().AsTime().Unix() > b.GetStakeStartTime().AsTime().Unix())
}

func iotx(rau string) string {
	v, ok := new(big.Int).SetString(rau, 10)
	if !ok {
		return rau
	}
	return new(big.Float).Quo(new(big.Float).SetInt(v), big.NewFloat(1e18)).Text('f', 4)
}

func printGuard(index uint64, g *iotextypes.BucketGuard) {
	state := "guarded"
	switch {
	case len(g.GetPublicKey()) == 0 && g.GetGeneration() == 0:
		state = "no record"
	case len(g.GetPublicKey()) == 0:
		state = "cleared (tombstone)"
	}
	fmt.Printf("guard of bucket %d: %s, generation %d, nonce %d, bound at %d",
		index, state, g.GetGeneration(), g.GetNonce(), g.GetBoundHeight())
	if len(g.GetPublicKey()) > 0 {
		fmt.Printf(", %s key %x…", pqcrypto.AlgID(g.GetAlgID()), g.GetPublicKey()[:8])
	}
	fmt.Println()
}

// ---- commands ----

func (c *client) cmdStake(args []string) error {
	fs := flag.NewFlagSet("stake", flag.ExitOnError)
	keyHex := fs.String("key", "", "owner secp256k1 key")
	cand := fs.String("candidate", "", "candidate name")
	amount := fs.Int64("amount", 1000, "amount in IOTX")
	check(fs.Parse(args))
	sk, err := ecdsaKey(*keyHex)
	if err != nil {
		return err
	}
	act, err := action.NewCreateStake(*cand, unit.ConvertIotxToRau(*amount).String(), 0, false, nil)
	if err != nil {
		return err
	}
	_, err = c.send(sk, act)
	if err != nil {
		return err
	}
	return c.listBuckets(sk.PublicKey().Address().String())
}

func (c *client) cmdBuckets(args []string) error {
	fs := flag.NewFlagSet("buckets", flag.ExitOnError)
	owner := fs.String("owner", "", "owner address")
	check(fs.Parse(args))
	return c.listBuckets(*owner)
}

func (c *client) listBuckets(owner string) error {
	l := &iotextypes.VoteBucketList{}
	if err := c.readStaking(iotexapi.ReadStakingDataMethod_BUCKETS_BY_VOTER, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByVoter{BucketsByVoter: &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
			VoterAddress: owner, Pagination: &iotexapi.PaginationParam{Limit: 100},
		}},
	}, "", l); err != nil {
		return err
	}
	for _, b := range l.GetBuckets() {
		printBucket(b)
	}
	return nil
}

func (c *client) cmdBind(args []string) error {
	fs := flag.NewFlagSet("bind", flag.ExitOnError)
	keyHex := fs.String("key", "", "bucket owner secp256k1 key")
	index := fs.Uint64("bucket", 0, "bucket index")
	seed := fs.String("seed", "", "guard seed (hex)")
	account := fs.Uint("account", 0, "guard key account")
	keyIndex := fs.Uint("index", 0, "guard key index")
	check(fs.Parse(args))
	sk, err := ecdsaKey(*keyHex)
	if err != nil {
		return err
	}
	k, err := guardKey(*seed, *account, *keyIndex)
	if err != nil {
		return err
	}
	cur, err := c.guard(*index, "")
	if err != nil {
		return err
	}
	pub := k.PublicKey().Bytes()
	owner := sk.PublicKey().Address()
	acc, err := k.Sign([]byte(bucketguard.Context), bucketguard.EncodeAccept(c.chainID, *index, cur.GetGeneration()+1, owner, k.PublicKey().Alg(), pub, 0))
	if err != nil {
		return err
	}
	if _, err := c.send(sk, action.NewBindBucketGuard(&bucketguard.BindBucketGuard{
		BucketIndex: *index, AlgID: k.PublicKey().Alg(), PublicKey: pub, AcceptSig: acc,
	})); err != nil {
		return err
	}
	return c.showGuard(*index, "")
}

func (c *client) cmdGuard(args []string) error {
	fs := flag.NewFlagSet("guard", flag.ExitOnError)
	index := fs.Uint64("bucket", 0, "bucket index")
	height := fs.String("height", "", "block height (default: tip)")
	check(fs.Parse(args))
	return c.showGuard(*index, *height)
}

func (c *client) showGuard(index uint64, height string) error {
	g, err := c.guard(index, height)
	if err != nil {
		return err
	}
	printGuard(index, g)
	if b, err := c.bucket(index); err == nil && height == "" {
		printBucket(b)
	}
	return nil
}

var opNames = map[string]bucketguard.Op{
	"unstake":          bucketguard.OpUnstake,
	"withdraw":         bucketguard.OpWithdraw,
	"transfer":         bucketguard.OpTransfer,
	"change-candidate": bucketguard.OpChangeCandidate,
	"restake":          bucketguard.OpRestake,
	"rotate":           bucketguard.OpRotate,
}

func (c *client) cmdOp(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("op needs one of unstake|withdraw|transfer|change-candidate|restake|rotate")
	}
	o, ok := opNames[args[0]]
	if !ok {
		return fmt.Errorf("unknown op %q", args[0])
	}
	fs := flag.NewFlagSet("op", flag.ExitOnError)
	sponsorHex := fs.String("sponsor", "", "sponsor secp256k1 key (pays gas)")
	index := fs.Uint64("bucket", 0, "bucket index")
	seed := fs.String("seed", "", "current guard seed (hex)")
	account := fs.Uint("account", 0, "current guard key account")
	keyIndex := fs.Uint("index", 0, "current guard key index")
	recipient := fs.String("recipient", "", "WITHDRAW recipient")
	newOwner := fs.String("new-owner", "", "TRANSFER new owner")
	clearGuard := fs.Bool("clear", false, "TRANSFER: clear the guard instead of installing a new key")
	newSeed := fs.String("new-seed", "", "TRANSFER/ROTATE: seed of the incoming key (hex)")
	newAccount := fs.Uint("new-account", 0, "incoming key account")
	newIndex := fs.Uint("new-index", 0, "incoming key index")
	cand := fs.String("candidate", "", "CHANGE_CANDIDATE candidate name")
	duration := fs.Uint("duration", 0, "RESTAKE duration in days")
	autoStake := fs.Bool("auto-stake", false, "RESTAKE auto-stake")
	deadlineBlocks := fs.Uint64("deadline-blocks", 100, "deadline as blocks from the tip")
	check(fs.Parse(args[1:]))

	sponsor, err := ecdsaKey(*sponsorHex)
	if err != nil {
		return err
	}
	k, err := guardKey(*seed, *account, *keyIndex)
	if err != nil {
		return err
	}
	cur, err := c.guard(*index, "")
	if err != nil {
		return err
	}
	b, err := c.bucket(*index)
	if err != nil {
		return err
	}
	owner, err := address.FromString(b.GetOwner())
	if err != nil {
		return err
	}
	tip, err := c.tip()
	if err != nil {
		return err
	}
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: *index, Generation: cur.GetGeneration(), Nonce: cur.GetNonce(),
		Deadline: tip + *deadlineBlocks, Op: o,
	}
	resultingOwner := owner
	switch o {
	case bucketguard.OpWithdraw:
		r, err := address.FromString(*recipient)
		if err != nil {
			return fmt.Errorf("-recipient: %w", err)
		}
		op.Recipient = r.Bytes()
	case bucketguard.OpTransfer:
		n, err := address.FromString(*newOwner)
		if err != nil {
			return fmt.Errorf("-new-owner: %w", err)
		}
		op.NewOwner, op.ClearGuard, resultingOwner = n.Bytes(), *clearGuard, n
	case bucketguard.OpChangeCandidate:
		cv2 := &iotextypes.CandidateV2{}
		if err := c.readStaking(iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME, &iotexapi.ReadStakingDataRequest{
			Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{CandName: *cand}},
		}, "", cv2); err != nil {
			return err
		}
		id := cv2.GetId()
		if id == "" {
			id = cv2.GetOwnerAddress()
		}
		cid, err := address.FromString(id)
		if err != nil {
			return fmt.Errorf("candidate %q: %w", *cand, err)
		}
		op.CandidateName, op.ExpectedCandidateID = *cand, cid.Bytes()
	case bucketguard.OpRestake:
		op.Duration, op.AutoStake = uint32(*duration), *autoStake
	}
	if o == bucketguard.OpRotate || (o == bucketguard.OpTransfer && !*clearGuard) {
		nk, err := guardKey(*newSeed, *newAccount, *newIndex)
		if err != nil {
			return fmt.Errorf("incoming key: %w", err)
		}
		pub := nk.PublicKey().Bytes()
		acc, err := nk.Sign([]byte(bucketguard.Context), bucketguard.EncodeAccept(c.chainID, *index, op.Generation+1, resultingOwner, nk.PublicKey().Alg(), pub, op.Deadline))
		if err != nil {
			return err
		}
		op.NewGuard = &bucketguard.NewGuard{AlgID: nk.PublicKey().Alg(), PublicKey: pub, AcceptSig: acc}
	}
	start := time.Now()
	if op.Sig, err = k.Sign([]byte(bucketguard.Context), bucketguard.EncodeCall(c.chainID, owner, op)); err != nil {
		return err
	}
	fmt.Printf("%s: generation %d nonce %d deadline %d, signed in %v (%d-byte signature)\n",
		o, op.Generation, op.Nonce, op.Deadline, time.Since(start).Round(time.Millisecond), len(op.Sig))
	if _, err := c.send(sponsor, action.NewGuardedBucketOp(op)); err != nil {
		return err
	}
	return c.showGuard(*index, "")
}

func (c *client) cmdLegacy(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("legacy needs one of unstake|withdraw|change-candidate|transfer|restake")
	}
	fs := flag.NewFlagSet("legacy", flag.ExitOnError)
	keyHex := fs.String("key", "", "bucket owner secp256k1 key")
	index := fs.Uint64("bucket", 0, "bucket index")
	cand := fs.String("candidate", "", "candidate name")
	newOwner := fs.String("new-owner", "", "new owner")
	duration := fs.Uint("duration", 0, "restake duration in days")
	check(fs.Parse(args[1:]))
	sk, err := ecdsaKey(*keyHex)
	if err != nil {
		return err
	}
	var p interface {
		IntrinsicGas() (uint64, error)
		SanityCheck() error
		FillAction(*iotextypes.ActionCore)
	}
	switch args[0] {
	case "unstake":
		p = action.NewUnstake(*index, nil)
	case "withdraw":
		p = action.NewWithdrawStake(*index, nil)
	case "change-candidate":
		p = action.NewChangeCandidate(*cand, *index, nil)
	case "transfer":
		n, err := address.FromString(*newOwner)
		if err != nil {
			return err
		}
		if p, err = action.NewTransferStake(n.String(), *index, nil); err != nil {
			return err
		}
	case "restake":
		p = action.NewRestake(*index, uint32(*duration), false, nil)
	default:
		return fmt.Errorf("unknown legacy action %q", args[0])
	}
	_, err = c.send(sk, p)
	return err
}
