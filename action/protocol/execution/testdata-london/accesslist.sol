// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract AccessList {
    mapping(address => mapping(bytes32 => bool)) public list;

    function set(
        address addr,
        bytes32 key,
        bool ret
    ) public {
        list[addr][key] = ret;
    }

    function get(address addr, bytes32 key) public view returns (bool) {
        return list[addr][key];
    }

    function remove(address addr, bytes32 key) public {
        delete list[addr][key];
    }
}
