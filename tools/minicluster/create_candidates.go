package main

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/iotexproject/iotex-core/tools/util"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	chainEndpoint  = "127.0.0.1:14014"
	privateKey     = "414efa99dfac6f4095d6954713fb0085268d400d6a05a8ae8a69b5b1c10b4bed"
	accountAddress = "io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02"

	existingOperatorAddr = "io1cs32huf9hg92em6vhmyf5qt6a9h02yys46zpe0"
)

func injectCandidates(addrs []*util.AddressKey) {
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
	addr, err := address.FromBytes(sk.PublicKey().Hash())
	if err != nil {
		log.L().Fatal("Failed to derive address from public key", zap.Error(err))
	}
	fmt.Println("/////////////////////inject addr:", addr.String())

	//candidateNames, err := getAllCandidateNames(chainClient)
	//if err != nil {
	//	log.L().Fatal("Failed to get all candidate names", zap.Error(err))
	//}
	time.Sleep(5 * time.Second)

	rand.Seed(time.Now().Unix())

	res, err := chainClient.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: accountAddress})
	if err != nil {
		log.L().Fatal("Failed to get account", zap.Error(err))
	}
	initNonce := res.AccountMeta.PendingNonce
	startNonce := initNonce
	count := uint64(4)
	candNumber := uint64(1)
	for startNonce < initNonce+uint64(27) {
		for nonce := startNonce; nonce < startNonce+count; nonce++ {
			//private, err := crypto.GenerateKey()
			//if err != nil {
			//	log.L().Fatal("Failed to generate new key for a new account", zap.Error(err))
			//}
			//addr, err := address.FromBytes(private.PublicKey().Hash())
			//if err != nil {
			//	log.L().Fatal("Failed to derive address from public key", zap.Error(err))
			//}

			fixedAmount := unit.ConvertIotxToRau(3000000).String()
			b := make([]byte, 6)
			for i := range b {
				b[i] = charset[rand.Intn(len(charset))]
			}
			candidateName := string(b)
			//if candNumber == uint64(2) {
			//	candidateName = candidateNames[rand.Intn(len(candidateNames))]
			//}
			operatorAddr := addrs[nonce%27].EncodedAddr
			//if candNumber == uint64(3) {
			//	operatorAddr = existingOperatorAddr
			//}

			cr, err := testutil.SignedCandidateRegister(nonce, candidateName, operatorAddr, addrs[nonce%27].EncodedAddr, addrs[nonce%27].EncodedAddr, fixedAmount, 1, false, nil, 1000000, big.NewInt(unit.Qev), sk)
			if err != nil {
				log.L().Fatal("Failed to create create_bucket", zap.Error(err))
			}
			request := &iotexapi.SendActionRequest{Action: cr.Proto()}
			res, err := chainClient.SendAction(context.Background(), request)
			if err != nil {
				log.L().Fatal("Failed to send action", zap.Error(err))
			}
			fmt.Println("inject candidate:", candNumber, res.ActionHash)
			candNumber += 1
		}
		startNonce = startNonce + count
		time.Sleep(time.Second * 7)
	}

	_, err = getAllCandidateNames(chainClient)
	if err != nil {
		log.L().Fatal("Failed to get all candidate names", zap.Error(err))
	}
}

func getAllCandidateNames(chainClient iotexapi.APIServiceClient) ([]string, error) {
	methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATES,
	})
	if err != nil {
		return nil, err
	}
	arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_Candidates_{
			Candidates: &iotexapi.ReadStakingDataRequest_Candidates{
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
		Arguments:  [][]byte{arg, []byte(strconv.FormatUint(1, 10))},
	}
	res, err := chainClient.ReadState(context.Background(), request)
	if err != nil {
		return nil, err
	}
	c := iotextypes.CandidateListV2{}
	if err := proto.Unmarshal(res.Data, &c); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	names := make([]string, 0)
	for _, candidate := range c.Candidates {
		names = append(names, candidate.GetName())
		fmt.Println("getAllCandidateNames:", candidate)
	}
	return names, nil
}
