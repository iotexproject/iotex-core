package candidatesutil

import (
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// CandidatesPrefix is the prefix of the key of candidateList
const CandidatesPrefix = "Candidates."

// AddCandidate adds a new candidate to candidateMap
func AddCandidate(candidateMap map[hash.PKHash]*state.Candidate, vote *action.Vote, height uint64) error {
	votePubkey := vote.VoterPublicKey()
	voterPKHash, err := iotxaddress.AddressToPKHash(vote.Voter())
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	if _, ok := candidateMap[voterPKHash]; !ok {
		candidateMap[voterPKHash] = &state.Candidate{
			Address:        vote.Voter(),
			PublicKey:      votePubkey,
			CreationHeight: height,
		}
	}
	return nil
}

// UpdateCandidate updates a candidate state
func UpdateCandidate(
	candiateMap map[hash.PKHash]*state.Candidate,
	addr string,
	totalWeight *big.Int,
	blockHeight uint64,
) error {
	addrHash, err := iotxaddress.AddressToPKHash(addr)
	if err != nil {
		return errors.Wrap(err, "failed to get public key hash from account address")
	}
	// Candidate was added when self-nomination, always exist in cachedCandidates
	candidate := candiateMap[addrHash]
	candidate.Votes = totalWeight
	candidate.LastUpdateHeight = blockHeight

	return nil
}

// GetMostRecentCandidateMap gets the most recent candidateMap from trie
func GetMostRecentCandidateMap(sm protocol.StateManager) (map[hash.PKHash]*state.Candidate, error) {
	var sc state.CandidateList
	for h := int(sm.Height()); h >= 0; h-- {
		candidatesKey := ConstructKey(uint64(h))
		var err error
		if err = sm.State(candidatesKey, &sc); err == nil {
			return state.CandidatesToMap(sc)
		}
		if errors.Cause(err) != state.ErrStateNotExist {
			return nil, errors.Wrap(err, "failed to get most recent state of candidateList")
		}
	}
	if sm.Height() == uint64(0) {
		return make(map[hash.PKHash]*state.Candidate), nil
	}
	return nil, errors.Wrap(state.ErrStateNotExist, "failed to get most recent state of candidateList")
}

// StoreCandidates puts updated candidates to trie
func StoreCandidates(candidateMap map[hash.PKHash]*state.Candidate, sm protocol.StateManager) error {
	candidateList, err := state.MapToCandidates(candidateMap)
	if err != nil {
		return errors.Wrap(err, "failed to convert candidate map to candidate list")
	}
	sort.Sort(candidateList)
	candidatesKey := ConstructKey(sm.Height())
	return sm.PutState(candidatesKey, &candidateList)
}

// ConstructKey constructs a key for candidates storage
func ConstructKey(height uint64) hash.PKHash {
	heightInBytes := byteutil.Uint64ToBytes(height)
	k := []byte(CandidatesPrefix)
	k = append(k, heightInBytes...)
	return byteutil.BytesTo20B(hash.Hash160b(k))
}
