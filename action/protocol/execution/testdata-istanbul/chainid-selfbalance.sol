pragma solidity ^0.8.4;

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
