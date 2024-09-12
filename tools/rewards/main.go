package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strconv"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/schollz/progressbar/v2"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/state"
)

var (
	blockRewardAmount = genesis.Default.DardanellesBlockReward()
	epochRewardAmount = genesis.Default.EpochReward()
)

func main() {
	conn, err := grpc.NewClient("api.iotex.one:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected to api.iotex.one:443")
	defer conn.Close()
	api := iotexapi.NewAPIServiceClient(conn)

	// epochStart := 47001
	// epochEnd := 47022
	blockStart := 32024261
	blockEnd := 32038923
	batch := 300
	blockRewards := make(map[string]*big.Int)
	epochRewards := make(map[string]*big.Int)
	totalRewards := make(map[string]*big.Int)
	bar := progressbar.NewOptions(blockEnd-blockStart, progressbar.OptionSetWriter(os.Stderr))
	bar.RenderBlank()
	for height := blockStart; height <= blockEnd; height += batch {
		bar.Add(batch)
		resp, err := api.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
			StartHeight:  uint64(height),
			Count:        uint64(batch),
			WithReceipts: true,
		})
		if err != nil {
			panic(err)
		}
		for _, blk := range resp.Blocks {
			for i, act := range blk.Block.Body.Actions {
				if act.Core.GetGrantReward() != nil {
					gr := act.Core.GetGrantReward()
					if blk.Receipts[i].Status == 1 {
						continue
					}
					switch gr.Type {
					case iotextypes.RewardType_BlockReward:
						pk, err := crypto.BytesToPublicKey(blk.Block.Header.GetProducerPubkey())
						if err != nil {
							panic(err)
						}
						addr := pk.Address()
						cand, err := candidateByAddress(api, addr.String(), epochNum(blk.Block.Header.Core.Height))
						if err != nil {
							panic(err)
						}
						rewardAddr, err := address.FromString(cand.RewardAddress)
						if err != nil {
							panic(err)
						}
						rewardAddrIO := cand.RewardAddress
						if _, ok := blockRewards[rewardAddrIO]; !ok {
							blockRewards[rewardAddrIO] = new(big.Int)
						}
						blockRewards[rewardAddrIO].Add(blockRewards[rewardAddrIO], blockRewardAmount)
						if _, ok := totalRewards[rewardAddrIO]; !ok {
							totalRewards[rewardAddrIO] = new(big.Int)
						}
						totalRewards[rewardAddrIO].Add(totalRewards[rewardAddrIO], blockRewardAmount)
						fmt.Printf("%12s, %10d, %42s, %43s, %42s, %43s\n", "blockReward", blk.Block.Header.Core.Height, addr.String(), addr.Hex(), rewardAddrIO, rewardAddr.Hex())
					case iotextypes.RewardType_EpochReward:
						fmt.Printf("%12s, %10d\n", "epochReward", blk.Block.Header.Core.Height)
						cands, err := activeProducersByEpoch(api, epochNum(blk.Block.Header.Core.Height))
						if err != nil {
							panic(err)
						}
						total := new(big.Int)
						for _, c := range cands {
							total.Add(total, c.Votes)
						}
						for _, c := range cands {
							reward := new(big.Int).Mul(epochRewardAmount, c.Votes)
							reward.Div(reward, total)
							if _, ok := epochRewards[c.RewardAddress]; !ok {
								epochRewards[c.RewardAddress] = new(big.Int)
							}
							epochRewards[c.RewardAddress].Add(epochRewards[c.RewardAddress], reward)
							if _, ok := totalRewards[c.RewardAddress]; !ok {
								totalRewards[c.RewardAddress] = new(big.Int)
							}
							totalRewards[c.RewardAddress].Add(totalRewards[c.RewardAddress], reward)
						}
					default:
						panic("invalid reward type")
					}
				}
			}
		}
	}
	bar.Finish()
	fmt.Printf("block rewards:\n")
	for addr, reward := range blockRewards {
		add, err := address.FromString(addr)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s %s\n", reward.String(), addr, add.Hex())
	}
	fmt.Printf("epoch rewards:\n")
	for addr, reward := range epochRewards {
		add, err := address.FromString(addr)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s %s\n", reward.String(), addr, add.Hex())
	}
	fmt.Printf("total rewards:\n")
	for addr, reward := range totalRewards {
		add, err := address.FromString(addr)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s %s\n", reward.String(), addr, add.Hex())
	}
}

func epochNum(height uint64) uint64 {
	if height < 32023801 {
		panic("invalid height")
	}
	return (height-32023801)/720 + 47001
}

func activeProducersByEpoch(api iotexapi.APIServiceClient, epoch uint64) (state.CandidateList, error) {
	if cands, ok := candidateCache[epoch]; ok {
		return cands, nil
	}
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("ActiveBlockProducersByEpoch"),
		Arguments:  [][]byte{[]byte(strconv.FormatUint(epoch, 10))},
		Height:     "",
	}
	abpResponse, err := api.ReadState(context.Background(), request)
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, errors.New("no active BPs found")
		} else if ok {
			return nil, errors.New(sta.Message())
		}
		return nil, errors.Wrap(err, "failed to read active BPs")
	}
	var ABPs state.CandidateList
	if err := ABPs.Deserialize(abpResponse.Data); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize active BPs")
	}
	candidateCache[epoch] = ABPs
	return ABPs, nil
}

var (
	candidateCache map[uint64]state.CandidateList = make(map[uint64]state.CandidateList)
)

func candidateByAddress(api iotexapi.APIServiceClient, addr string, epoch uint64) (*state.Candidate, error) {
	cands, err := activeProducersByEpoch(api, epoch)
	if err != nil {
		return nil, err
	}
	idx := slices.IndexFunc(cands, func(c *state.Candidate) bool {
		return c.Address == addr
	})
	if idx == -1 {
		return nil, errors.New("candidate not found")
	}
	return cands[idx], nil
}
