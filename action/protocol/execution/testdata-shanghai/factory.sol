// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract A {
    uint256[] public amounts;

    function init(uint256[] memory _amounts) public {
        amounts = _amounts;
    }
}

contract Factory {
    struct AData {
        uint256[] amounts;
    }
    mapping(address => AData) listOfData;

    function set(uint256[] memory _amounts) public {
        listOfData[msg.sender] = AData(_amounts);
    }

    function make() public returns (address) {
        A a = new A();
        a.init(listOfData[msg.sender].amounts);
        return address(a);
    }
}
