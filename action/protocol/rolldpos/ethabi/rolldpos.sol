// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

interface IRollDPoSProtocolContract {
    function NumCandidateDelegates()
        external
        view
        returns (uint256 numCandidateDelegates, uint256 height);

    function NumDelegates()
        external
        view
        returns (uint256 numDelegates, uint256 height);

    function NumSubEpochs(
        uint256 blockHeight
    ) external view returns (uint256 numSubEpochs, uint256 height);

    function EpochNumber(
        uint256 blockHeight
    ) external view returns (uint256 epochNumber, uint256 height);

    function EpochHeight(
        uint256 epochNumber
    ) external view returns (uint256 epochHeight, uint256 height);

    function EpochLastHeight(
        uint256 epochNumber
    ) external view returns (uint256 epochLastHeight, uint256 height);

    function SubEpochNumber(
        uint256 blockHeight
    ) external view returns (uint256 subEpochNumber, uint256 height);
}
