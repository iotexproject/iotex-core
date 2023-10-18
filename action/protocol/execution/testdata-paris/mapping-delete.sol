// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract MyContract {
    struct Data {
        uint256 a;
        uint256 b;
    }
    mapping(uint256 => Data) public items;

    constructor() {
        items[0] = Data(1, 2);
        items[1] = Data(3, 4);
        items[2] = Data(5, 6);
        delete items[1];
    }
}
