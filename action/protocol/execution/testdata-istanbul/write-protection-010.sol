// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

// NOTE: Deploy this contract first
contract B {
    // NOTE: storage layout must be the same as contract A
    uint public num;
    uint public value;
    mapping(uint => uint) private _b;
    address private c;

    constructor() public {
        _b[1] = 1;
        _b[2] = 1;
        
        c = address(new C());
    }



    function setVars(uint _num) public payable {
        c.staticcall(
            abi.encodeWithSignature("setVars(uint256)", _num)
        );    // staticcall -> staticcall -> revrt        
    }
}

contract C {
    mapping(uint => uint) private _c;

    constructor() public {
        _c[1] = 1;
        _c[2] = 1;
    }

    function setVars(uint _num) public payable {
        delete _c[1];
        delete _c[2];//no reached
    }    
}

contract A {
    uint public num;
    address public sender;
    uint public value;
    address private b;
    mapping(uint => uint) private _a;
    event Success(bool);

    function make() public {
        b = address(new B());
        _a[1] = 1;
        _a[2] = 2;
    }

    function setVars(address _contract, uint _num) public returns (uint256) {
        _contract = b;
        (bool success, bytes memory _) = _contract.staticcall(
            abi.encodeWithSignature("setVars(uint256)", 0)
        );
        delete _a[1];
        delete _a[2];
        emit Success(success);
        _contract.staticcall(
            abi.encodeWithSignature("setVars(uint256)", 1)
        );
        return 1;
    }
}