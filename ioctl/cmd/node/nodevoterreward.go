// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package node

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// readRewardingState issues a single rewarding-protocol ReadState call.
func readRewardingState(method string, args ...string) ([]byte, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	byteArgs := make([][]byte, len(args))
	for i, a := range args {
		byteArgs[i] = []byte(a)
	}
	resp, err := cli.ReadState(ctx, &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte(method),
		Arguments:  byteArgs,
	})
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	return resp.Data, nil
}

// resolveDelegateOrVoter turns an alias or address into a bech32 string.
func resolveDelegateOrVoter(arg string) (string, error) {
	addr, err := util.Address(arg)
	if err != nil {
		return "", output.NewError(output.AddressError, "failed to resolve address", err)
	}
	return addr, nil
}

// bytesToBech32 renders a 20-byte address field. Empty means unset, which the
// protocol uses to mean "no override" rather than "the zero address".
func bytesToBech32(b []byte) string {
	if len(b) == 0 {
		return "(unset)"
	}
	addr, err := address.FromBytes(b)
	if err != nil {
		return "0x" + hex.EncodeToString(b)
	}
	return addr.String()
}

func voterRewardPending(delegate string) error {
	addr, err := resolveDelegateOrVoter(delegate)
	if err != nil {
		return err
	}
	data, err := readRewardingState("PendingVoterReward", addr)
	if err != nil {
		return err
	}
	rau, ok := new(big.Int).SetString(string(data), 10)
	if !ok {
		return output.NewError(output.ConvertError, "failed to convert pending reward into big int", nil)
	}
	fmt.Printf("delegate: %s\npendingVoterReward: %s IOTX\n",
		addr, util.RauToString(rau, util.IotxDecimalNum))
	return nil
}

func voterRewardDelegates() error {
	data, err := readRewardingState("PendingVoterRewardDelegates")
	if err != nil {
		return err
	}
	var pb rewardingpb.PendingVoterRewardDelegates
	if err := proto.Unmarshal(data, &pb); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal delegate list", err)
	}
	if len(pb.DelegateIdentifiers) == 0 {
		fmt.Println("no delegate currently has a pending voter reward pool")
		return nil
	}
	fmt.Printf("delegates with a pending voter reward pool (%d):\n", len(pb.DelegateIdentifiers))
	for _, id := range pb.DelegateIdentifiers {
		fmt.Printf("  %s\n", bytesToBech32(id))
	}
	return nil
}

func voterRewardDistribution() error {
	data, err := readRewardingState("VoterRewardDistribution")
	if err != nil {
		return err
	}
	var pb rewardingpb.VoterRewardDistributionState
	if err := proto.Unmarshal(data, &pb); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal distribution state", err)
	}
	phase := map[uint32]string{
		0: "0 (scanning [startVoter, max])",
		1: "1 (scanning [min, startVoter))",
		2: "2 (complete)",
	}[pb.ScanPhase]
	if phase == "" {
		phase = fmt.Sprintf("%d", pb.ScanPhase)
	}
	fmt.Printf("targetEra: %d\nfreezeHeight: %d\nscanPhase: %s\ncompleted: %t\n",
		pb.TargetEra, pb.FreezeHeight, phase, pb.Completed)
	if pb.Completed {
		fmt.Printf("completedHeight: %d\n", pb.CompletedHeight)
	}
	fmt.Printf("startVoter: %s\nresumeVoter: %s\ndelegateAllocations: %d\n",
		bytesToBech32(pb.StartVoter), bytesToBech32(pb.ResumeVoter), len(pb.DelegateAllocations))
	return nil
}

func voterRewardSnapshot(delegate string) error {
	addr, err := resolveDelegateOrVoter(delegate)
	if err != nil {
		return err
	}
	data, err := readRewardingState("DelegateRewardSnapshot", addr)
	if err != nil {
		return err
	}
	var pb stakingpb.CandidateRewardSnapshot
	if err := proto.Unmarshal(data, &pb); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal reward snapshot", err)
	}
	totalWeight := new(big.Int).SetBytes(pb.TotalWeight)
	selfStake := fmt.Sprintf("%d", pb.SelfStakeBucketIdx)
	if pb.SelfStakeBucketIdx == math.MaxUint64 {
		selfStake = "(none)"
	}
	fmt.Printf("delegate: %s\nfreezeHeight: %d\ncommissionConfigured: %t\n",
		addr, pb.FreezeHeight, pb.CommissionConfigured)
	fmt.Printf("blockCommission: %d bp (%.2f%%)\nepochCommission: %d bp (%.2f%%)\n",
		pb.BlockCommissionBasisPoints, float64(pb.BlockCommissionBasisPoints)/100,
		pb.EpochCommissionBasisPoints, float64(pb.EpochCommissionBasisPoints)/100)
	if !pb.CommissionConfigured {
		fmt.Println("  note: commission is unconfigured in DelegateProfile, so the delegate is")
		fmt.Println("        frozen at 100% and its voters receive nothing on the on-chain path")
	}
	fmt.Printf("totalWeight: %s\nselfStakeBucketIdx: %s\n", totalWeight.String(), selfStake)
	return nil
}

func voterRewardPayoutAddress(delegate string) error {
	addr, err := resolveDelegateOrVoter(delegate)
	if err != nil {
		return err
	}
	data, err := readRewardingState("DelegatePayoutAddress", addr)
	if err != nil {
		return err
	}
	var pb rewardingpb.DelegatePayoutAddress
	if err := proto.Unmarshal(data, &pb); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal payout address", err)
	}
	fmt.Printf("delegate: %s\npayoutAddress: %s\nonchainRewardEnabled: %t\n",
		addr, bytesToBech32(pb.Address), pb.OnchainRewardEnabled)
	if !pb.OnchainRewardEnabled {
		fmt.Println("  note: the delegate has not opted in; run 'ioctl stake2 voterrewardoptin'")
	}
	return nil
}

func voterRewardDestination(voter string) error {
	addr, err := resolveDelegateOrVoter(voter)
	if err != nil {
		return err
	}
	data, err := readRewardingState("VoterRewardDestination", addr)
	if err != nil {
		return err
	}
	var pb rewardingpb.VoterRewardDestination
	if err := proto.Unmarshal(data, &pb); err != nil {
		return output.NewError(output.SerializationError, "failed to unmarshal reward destination", err)
	}
	fmt.Printf("voter: %s\nrecipient: %s\nexplicitlySet: %t\n",
		addr, bytesToBech32(pb.Recipient), pb.ExplicitlySet)
	if pb.ExplicitlySet {
		fmt.Printf("updatedHeight: %d\n", pb.UpdatedHeight)
	}
	return nil
}
