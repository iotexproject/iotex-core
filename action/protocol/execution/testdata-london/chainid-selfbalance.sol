// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract ChainidAndSelfbalance {
    uint256 counter = 0;

    function getChainID() public view returns (uint256) {
        return block.chainid;
    }

    function getSelfBalance() public view returns (uint256) {
        return address(this).balance;
    }

    function increment() public {
        counter++;
    }
}
