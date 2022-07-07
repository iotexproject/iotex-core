// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract A {
    function tuple() public pure returns (uint256, string memory) {
        return (1, "Hi");
    }

    function getOne() public pure returns (uint256) {
        uint256 a;
        (a, ) = tuple();
        return a;
    }
}
