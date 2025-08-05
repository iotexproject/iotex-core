// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

interface INativeStakingContract {
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
        bytes memory blsPublicKey,
        uint8[] memory data
    ) external payable;

    function candidateActivate(uint64 bucketIndex) external;

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
