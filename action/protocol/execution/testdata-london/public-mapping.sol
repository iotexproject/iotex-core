// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract AA {
    mapping(uint256 => uint256) public objects;

    function Set() public {
        objects[0] = 42;
    }
}

contract BB {
    // insert address of deployed First here
    AA a;

    function setContract(address addr) public {
        a = AA(addr);
    }

    function get() public view returns (uint256) {
        return a.objects(0);
    }
}
