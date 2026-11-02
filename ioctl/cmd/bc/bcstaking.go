// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bc

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// ReadStakingData calls the staking protocol's ReadState method at height (0
// means the tip), unmarshals the result into out, and returns the height the
// node read at.
func ReadStakingData(method iotexapi.ReadStakingDataMethod_Name, req *iotexapi.ReadStakingDataRequest, height uint64, out proto.Message) (uint64, error) {
	methodData, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{Method: method})
	if err != nil {
		return 0, output.NewError(output.SerializationError, "failed to marshal read staking data method", err)
	}
	requestData, err := proto.Marshal(req)
	if err != nil {
		return 0, output.NewError(output.SerializationError, "failed to marshal read staking data request", err)
	}
	request := &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"),
		MethodName: methodData,
		Arguments:  [][]byte{requestData},
	}
	if height > 0 {
		request.Height = strconv.FormatUint(height, 10)
	}

	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return 0, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	ctx := context.Background()
	if jwtMD, err := util.JwtAuth(); err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}
	response, err := iotexapi.NewAPIServiceClient(conn).ReadState(ctx, request)
	if err != nil {
		if sta, ok := status.FromError(err); ok {
			return 0, output.NewError(output.APIError, sta.Message(), nil)
		}
		return 0, output.NewError(output.NetworkError, "failed to invoke ReadState api", err)
	}
	if err := proto.Unmarshal(response.GetData(), out); err != nil {
		return 0, output.NewError(output.SerializationError, "failed to unmarshal response", err)
	}
	return response.GetBlockIdentifier().GetHeight(), nil
}

// GetBucketGuard reads the guard record of a native bucket at height (0 means
// the tip). A bucket without a record returns an empty BucketGuard.
func GetBucketGuard(index, height uint64) (*iotextypes.BucketGuard, uint64, error) {
	g := &iotextypes.BucketGuard{}
	h, err := ReadStakingData(iotexapi.ReadStakingDataMethod_BUCKET_GUARD, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketGuard_{
			BucketGuard: &iotexapi.ReadStakingDataRequest_BucketGuard{Index: index},
		},
	}, height, g)
	if err != nil {
		return nil, 0, err
	}
	return g, h, nil
}

// GetBucketByIndex reads a native bucket at height (0 means the tip). It
// returns an error if the bucket does not exist.
func GetBucketByIndex(index, height uint64) (*iotextypes.VoteBucket, uint64, error) {
	l := &iotextypes.VoteBucketList{}
	h, err := ReadStakingData(iotexapi.ReadStakingDataMethod_BUCKETS_BY_INDEXES, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketsByIndexes{
			BucketsByIndexes: &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{Index: []uint64{index}},
		},
	}, height, l)
	if err != nil {
		return nil, 0, err
	}
	for _, b := range l.GetBuckets() {
		if b.GetIndex() == index {
			return b, h, nil
		}
	}
	return nil, 0, output.NewError(output.APIError, fmt.Sprintf("bucket %d not found", index), nil)
}

// GetCandidateByName reads a candidate by name at the tip.
func GetCandidateByName(name string) (*iotextypes.CandidateV2, error) {
	c := &iotextypes.CandidateV2{}
	if _, err := ReadStakingData(iotexapi.ReadStakingDataMethod_CANDIDATE_BY_NAME, &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByName_{
			CandidateByName: &iotexapi.ReadStakingDataRequest_CandidateByName{CandName: name},
		},
	}, 0, c); err != nil {
		return nil, err
	}
	if c.GetName() == "" && c.GetOwnerAddress() == "" {
		return nil, output.NewError(output.APIError, fmt.Sprintf("candidate %q not found", name), nil)
	}
	return c, nil
}

// GetGenesisHash returns the hash of the genesis block, read as the parent
// hash of block 1.
func GetGenesisHash() ([32]byte, error) {
	meta, err := getBlockMetaByHeight(1)
	if err != nil {
		return [32]byte{}, err
	}
	return parseGenesisHash(meta.GetPreviousBlockHash())
}

func parseGenesisHash(s string) ([32]byte, error) {
	var h [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(h) {
		return h, output.NewError(output.ConvertError, fmt.Sprintf("invalid genesis hash %q", s), nil)
	}
	copy(h[:], b)
	return h, nil
}
