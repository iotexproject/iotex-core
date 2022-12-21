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
        if (_num == 0) {
            delete _b[1];
            c.staticcall(
                abi.encodeWithSignature("setVars(uint256)", _num)
            );    // call -> staticcall -> revrt        
        }
        if (_num == 1) {
            delete _b[2]; //staticcall -> revert
        }
    }
}

contract C {
    mapping(uint => uint) private _c;

    constructor() public {
        _c[1] = 1;
        _c[2] = 1;
    }

    function setVars(uint _num) public payable {
        if (_num == 0) {
            delete _c[1];
        }
    }    
}

contract A {
    uint public num;
    address public sender;
    uint public value;
    address private b;
    event Success(bool);

    function make() public {
        b = address(new B());
    }

    function setVars(address _contract, uint _num) public returns (uint256) {
        _contract = b;
        _contract.call(
            abi.encodeWithSignature("setVars(uint256)", 0)
        );
        (bool success, bytes memory _) = _contract.staticcall(
            abi.encodeWithSignature("setVars(uint256)", 1)
        );
        
        emit Success(success);

        return 1;
    }
}