// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

// NOTE: this Solidity interface is the human-readable source-of-truth for
// the staking ABI surface. Its companion file native_staking_contract_abi.json
// is the runtime ABI that iotex-core loads via NativeStakingContractABI().
// The two are NOT auto-generated from each other; any new method, parameter,
// or event added here must be added by hand to the JSON as well (and the
// reverse). Out-of-sync edits silently break the web3 path — clients see one
// signature while the node decodes another.

interface INativeStakingContract {
    // Events
    event CandidateRegistered(
        address indexed candidate,
        address indexed ownerAddress,
        address operatorAddress,
        string name,
        address rewardAddress,
        bytes blsPubKey
    );
    event Staked(
        address indexed voter,
        address indexed candidate,
        uint64 bucketIndex,
        uint256 amount,
        uint32 duration,
        bool autoStake
    );
    event CandidateActivated(
        address indexed candidate,
        uint64 bucketIndex
    );
    event CandidateDeactivationRequested(
        address indexed candidate
    );
    event CandidateDeactivationScheduled(
        address indexed candidate,
        uint64 blockNumber
    );
    event CandidateDeactivated(
        address indexed candidate
    );
    event CandidateUpdated(
        address indexed candidate,
        address indexed ownerAddress,
        address operatorAddress,
        string name,
        address rewardAddress,
        bytes blsPubKey
    );

    function candidateRegister(
        string memory name,
        address operatorAddress,
        address rewardAddress,
        address ownerAddress,
        uint256 amount,
        uint32 duration,
        bool autoStake,
        uint8[] memory data
    ) external;

    function candidateRegisterWithBLS(
        string memory name,
        address operatorAddress,
        address rewardAddress,
        address ownerAddress,
        uint32 duration,
        bool autoStake,
        bytes memory blsPubKey,
        uint8[] memory data
    ) external payable;

    // candidateRegisterWithBLSAndPoP adds the BLS proof-of-possession
    // (blsPop) required post-fork by the BLS Producer Identity follow-up
    // to IIP-52. The PoP is a 96-byte BLS signature over a domain-tagged
    // hash that binds the registrant to their blsPubKey + candidate
    // identity; without it the handler rejects the registration because
    // IIP-52's FastAggregateVerify path is vulnerable to a rogue-key
    // aggregate-forgery attack against un-attested keys. The legacy
    // candidateRegisterWithBLS entry above is kept so existing tooling
    // doesn't break, but its calldata is rejected post-fork.
    function candidateRegisterWithBLSAndPoP(
        string memory name,
        address operatorAddress,
        address rewardAddress,
        address ownerAddress,
        uint32 duration,
        bool autoStake,
        bytes memory blsPubKey,
        bytes memory blsPop,
        uint8[] memory data
    ) external payable;

    function candidateActivate(uint64 bucketIndex) external;

    // Candidate Deactivate methods
    function requestCandidateDeactivation() external;

    function cancelCandidateDeactivation() external;

    function confirmCandidateDeactivation() external;

    // Candidate Endorsement methods
    function candidateEndorsement(uint64 bucketIndex, bool endorse) external;

    function endorseCandidate(uint64 bucketIndex) external;

    function intentToRevokeEndorsement(uint64 bucketIndex) external;

    function revokeEndorsement(uint64 bucketIndex) external;

    // Candidate Transfer Ownership
    function candidateTransferOwnership(
        address newOwner,
        uint8[] memory payload
    ) external;

    // Candidate Update
    function candidateUpdate(
        string memory name,
        address operatorAddress,
        address rewardAddress
    ) external;

    function candidateUpdateWithBLS(
        string memory name,
        address operatorAddress,
        address rewardAddress,
        bytes memory blsPubKey
    ) external;

    // candidateUpdateWithBLSAndPoP — the V2 counterpart that carries the
    // BLS proof-of-possession needed post-fork when rotating the BLS
    // pubkey (see candidateRegisterWithBLSAndPoP above for the
    // motivation).
    function candidateUpdateWithBLSAndPoP(
        string memory name,
        address operatorAddress,
        address rewardAddress,
        bytes memory blsPubKey,
        bytes memory blsPop
    ) external;

    // Stake Management
    function depositToStake(
        uint64 bucketIndex,
        uint256 amount,
        uint8[] memory data
    ) external;

    function changeCandidate(
        string memory candName,
        uint64 bucketIndex,
        uint8[] memory data
    ) external;

    function createStake(
        string memory candName,
        uint256 amount,
        uint32 duration,
        bool autoStake,
        uint8[] memory data
    ) external;

    function migrateStake(uint64 bucketIndex) external;

    function unstake(uint64 bucketIndex, uint8[] memory data) external;

    function withdrawStake(uint64 bucketIndex, uint8[] memory data) external;

    function restake(
        uint64 bucketIndex,
        uint32 duration,
        bool autoStake,
        uint8[] memory data
    ) external;

    function transferStake(
        address voterAddress,
        uint64 bucketIndex,
        uint8[] memory data
    ) external;
}
