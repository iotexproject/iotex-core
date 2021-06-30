pragma solidity ^0.8.4;
contract AA {
    mapping(uint => uint) public objects;

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

    function get() public view returns(uint) {
        return a.objects(0);
    }
}
