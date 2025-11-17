// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

interface IPollProtocolContract {
    struct Candidate {
        address id;
        string name;
        address operatorAddress;
        address rewardAddress;
        bytes blsPubKey;
        uint256 votes;
    }

    struct ProbationInfo {
        address candidate;
        address operatorAddress;
        uint32 count;
    }

    struct ProbationList {
        ProbationInfo[] probationInfo;
        uint32 intensityRate;
    }

    function CandidatesByEpoch()
        external
        view
        returns (Candidate[] memory candidates, uint256 height);

    function CandidatesByEpoch(
        uint256 epoch
    ) external view returns (Candidate[] memory candidates, uint256 height);

    function BlockProducersByEpoch()
        external
        view
        returns (Candidate[] memory candidates, uint256 height);

    function BlockProducersByEpoch(
        uint256 epoch
    ) external view returns (Candidate[] memory candidates, uint256 height);

    function ActiveBlockProducersByEpoch()
        external
        view
        returns (Candidate[] memory candidates, uint256 height);

    function ActiveBlockProducersByEpoch(
        uint256 epoch
    ) external view returns (Candidate[] memory candidates, uint256 height);

    function ProbationListByEpoch()
        external
        view
        returns (ProbationList memory probation, uint256 height);

    function ProbationListByEpoch(
        uint256 epoch
    ) external view returns (ProbationList memory probation, uint256 height);
}
