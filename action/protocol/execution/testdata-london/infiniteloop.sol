// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract SimpleStorage {
    uint256 storedData;

    event Set(uint256);
    event Get(address, uint256);
    event Deadlock();

    function set(uint256 x) public {
        storedData = x;
        emit Set(x);
    }

    function get(address _to) public returns (uint256) {
        require(_to != address(0));
        emit Get(_to, storedData);
        return storedData;
    }

    function infinite() public {
        while (true) {
            storedData++;
        }
        emit Deadlock();
    }
}
