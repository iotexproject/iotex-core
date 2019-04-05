pragma solidity ^0.4.24;

contract A {
    uint[] public amounts;
    function init(uint[] _amounts) {
        amounts = _amounts;
    }
}

contract Factory {
    struct AData {
        uint[] amounts;
    }
    mapping (address => AData) listOfData;

    function set(uint[] _amounts) {
        listOfData[msg.sender] = AData(_amounts);
    }

    function make() returns(address) {
        A a = new A();
        a.init(listOfData[msg.sender].amounts);
        return address(a);
    }
}