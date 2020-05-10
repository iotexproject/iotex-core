package main

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
//chainEndpoint  = "api.nightly-cluster-2.iotex.one:80"
//privateKey     = "414efa99dfac6f4095d6954713fb0085268d400d6a05a8ae8a69b5b1c10b4bed"
//accountAddress = "io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02"
)

func injectBuckets() {
	fmt.Println("/////////////////////injectBuckets start:")
	grpcCtx1, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn1, err := grpc.DialContext(grpcCtx1, chainEndpoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to chain's API server.")
	}
	chainClient := iotexapi.NewAPIServiceClient(conn1)

	sk, err := crypto.HexStringToPrivateKey(privateKey)
	if err != nil {
		log.L().Panic(
			"Error when decoding private key string",
			zap.String("keyStr", privateKey),
			zap.Error(err),
		)
	}

	candidateNames, err := getAllCandidateNames(chainClient)
	if err != nil {
		log.L().Fatal("Failed to get all candidate names", zap.Error(err))
	}

	rand.Seed(time.Now().Unix())

	// Create 100 new accounts
	for i := 0; i < 10; i++ {
		private, err := crypto.GenerateKey()
		if err != nil {
			log.L().Fatal("Failed to generate new key for a new account", zap.Error(err))
		}
		addr, err := address.FromBytes(private.PublicKey().Hash())
		if err != nil {
			log.L().Fatal("Failed to derive address from public key", zap.Error(err))
		}
		res, err := chainClient.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: accountAddress})
		if err != nil {
			log.L().Fatal("Failed to get account", zap.Error(err))
		}
		fmt.Println(addr.String(), ":", res.AccountMeta.Nonce, ":", res.AccountMeta.PendingNonce)
		transfer, err := testutil.SignedTransfer(addr.String(), sk, res.AccountMeta.Nonce+1, unit.ConvertIotxToRau(100000), nil, 100000, big.NewInt(unit.Qev))
		if err != nil {
			log.L().Fatal("Failed to create transfer", zap.Error(err))
		}
		request := &iotexapi.SendActionRequest{Action: transfer.Proto()}
		sendRes, err := chainClient.SendAction(context.Background(), request)
		if err != nil {
			log.L().Fatal("Failed to send transfer action", zap.Error(err))
		}
		fmt.Println(i, "Send transfer hash", sendRes.ActionHash)
		fmt.Println("---------------------------------------")

		time.Sleep(time.Second * 15)

		startNonce := uint64(1)
		count := uint64(4)
		for startNonce <= 50 {
			for nonce := startNonce; nonce < startNonce+count; nonce++ {
				fixedAmount := unit.ConvertIotxToRau(100).String()
				candidateName := candidateNames[rand.Intn(len(candidateNames))]
				ex, err := testutil.SignedCreateStake(nonce, candidateName, fixedAmount, 1, true, nil, 1000000, big.NewInt(unit.Qev), private)
				if err != nil {
					log.L().Fatal("Failed to create create_bucket", zap.Error(err))
				}
				request := &iotexapi.SendActionRequest{Action: ex.Proto()}
				res, err := chainClient.SendAction(context.Background(), request)
				if err != nil {
					log.L().Fatal("Failed to send action", zap.Error(err))
				}
				fmt.Println("inject bucket:", i, nonce, res.ActionHash)
			}
			startNonce = startNonce + count
			time.Sleep(time.Second * 7)
		}
		fmt.Println()
	}
	_, err = getAllBuckets(chainClient)
	if err != nil {
		log.L().Fatal("Failed to get all candidate names", zap.Error(err))
	}
}

func getAllBuckets(chainClient iotexapi.APIServiceClient) ([]string, error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKETS,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Buckets{
			Buckets: &iotexapi.ReadStakingDataRequest_VoteBuckets{
				Pagination: &iotexapi.PaginationParam{
					Offset: 0,
					Limit:  500,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodName,
		Arguments:  [][]byte{arg, []byte(strconv.FormatUint(10, 10))},
	}
	res, err := chainClient.ReadState(context.Background(), request)
	if err != nil {
		return nil, err
	}
	b := iotextypes.VoteBucketList{}
	if err := proto.Unmarshal(res.Data, &b); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	names := make([]string, 0)
	for _, bucket := range b.Buckets {
		names = append(names, bucket.Owner)
		fmt.Println("getAllBuckets:", bucket)
	}
	return names, nil
}
