// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract One {
    string public word;

    function setMsg(string calldata whatever) public {
        word = whatever;
    }

    function getMsg() public view returns (string memory) {
        return word;
    }
}

contract Two {
    One o;

    constructor(address one) {
        o = One(one);
        o.setMsg("test");
    }

    function getMsg() public view returns (string memory) {
        return o.getMsg();
    }
}
