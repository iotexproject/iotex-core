// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

// NOTE: Deploy this contract first
contract B {
    // NOTE: storage layout must be the same as contract A
    uint public num;
    address public sender;
    uint public value;
    mapping(uint => uint) private _a;
    event Done();

    constructor() public {
        _a[1] = 1;
        _a[2] = 2;
        _a[3] = 3;
    }

    function setVars(uint _num) public payable {
        if (_num == 0) {
            delete _a[1];
        }
        if (_num == 1) {
            delete _a[2];
            delete _a[3];
        }
        num = _num;
        sender = msg.sender;
        value = msg.value;
        emit Done();
    }
}

contract A {
    uint public num;
    address public sender;
    uint public value;
    address private c;
    mapping(uint => uint) private _a;
    event Success(bool);

    function make() public {
        c = address(new B());
        _a[1] = 1;
        _a[2] = 2;
        _a[3] = 3;
    }

    function setVars(address _contract, uint _num) public returns (uint256) {
        if (_contract == address(0)) {
            _contract = c;
        }
        delete _a[1];
        (bool success, bytes memory _) = _contract.staticcall(
            abi.encodeWithSignature("setVars(uint256)", 0)
        );
        _contract.staticcall(
            abi.encodeWithSignature("setVars(uint256)", 1)
        );
        
        emit Success(success);

        return 1;
    }
}