package state

const (
	// SystemNamespace is the namespace to store system information such as candidates/probationList/unproductiveDelegates
	// Poll Protocol uses this namespace to store states:
	//   - hash256(CurCandidateKey/NextCandidateKey) --> CandidatesList
	//   - hash256(CurProbationKey/NextProbationKey) --> ProbationList
	//   - hash256(UnproductiveDelegatesKey) --> UnproductiveDelegates
	//   - hash256(BlockMetaPrefix)+height%heightInEpoch --> BlockMeta
	SystemNamespace = "System"

	// AccountKVNamespace is the bucket name for account
	// Poll Protocol uses this namespace to store LEGACY states:
	//   - hash160(CandidatesPrefix+height) --> CandidatesList
	// Rewarding Protocol uses this namespace to store LEGACY states:
	//   - hash160(hash160(rewarding)+adminKey) --> admin
	//   - hash160(hash160(rewarding)+exemptKey) --> exempt
	//   - hash160(hash160(rewarding)+fundKey) --> fund
	//   - hash160(hash160(rewarding)+_blockRewardHistoryKeyPrefix+height) --> rewardHistory
	//   - hash160(hash160(rewarding)+_epochRewardHistoryKeyPrefix+epoch) --> rewardHistory
	//   - hash160(hash160(rewarding)+adminKey+address) --> rewardAccount
	AccountKVNamespace = "Account"

	// RewardingNamespace is the namespace to store rewarding information
	//   - hash160(rewarding)+adminKey --> admin
	//   - hash160(rewarding)+exemptKey --> exempt
	//   - hash160(rewarding)+fundKey --> fund
	//   - hash160(rewarding)+_blockRewardHistoryKeyPrefix+height --> rewardHistory
	//   - hash160(rewarding)+_epochRewardHistoryKeyPrefix+epoch --> rewardHistory
	//   - hash160(rewarding)+adminKey+address --> rewardAccount
	RewardingNamespace = "Rewarding"

	// StakingNamespace is the namespace to store staking information
	//   - "0" + totalBucketKey --> totalBucketCount
	//   - "1" + <bucketID> --> VoteBucket
	//   - "2" + <owner> --> BucketIndices
	//   - "3" + <candidate> --> BucketIndices
	//   - "4" + <bucketID> --> Endorsement
	StakingNamespace = "Staking"

	// CandidateNamespace is the namespace to store candidate information
	//   - <ID> --> Candidate
	CandidateNamespace = "Candidate"

	// CandsMapNamespace is the namespace to store candidate map
	//   - "name" --> CandidateList
	//   - "operator" --> CandidateList
	//   - "owner" --> CandidateList
	CandsMapNamespace = "CandsMap"

	// CodeKVNameSpace is the bucket name for code
	//   codeHash --> code
	CodeKVNameSpace = "Code"

	// ContractKVNameSpace is the bucket name for contract data storage
	//   trieKey --> trieValue
	ContractKVNameSpace = "Contract"

	// PreimageKVNameSpace is the bucket name for preimage data storage
	PreimageKVNameSpace = "Preimage"
)
