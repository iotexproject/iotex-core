pragma solidity 0.4.24;
contract AA {
    mapping(uint => uint) public objects;

    function Set() {
        objects[0] = 42;
    }
}

contract BB {
    // insert address of deployed First here
    AA a;
    function setContract(address addr) {
        a = AA(addr);
    }

    function get() returns(uint) {
        return a.objects(0);
    }
}
