pragma solidity ^0.8.4;
contract Contract {
    uint[] public values;

    function find(uint value) internal view returns(uint) {
        uint i = 0;
        while (values[i] != value) {
            i++;
        }
        return i;
    }

    function removeByValue(uint value) public {
        uint i = find(value);
        removeByIndex(i);
    }

    function removeByIndex(uint i) public {
        while (i<values.length-1) {
            values[i] = values[i+1];
            i++;
        }
        values.pop();
    }

    function getValues() internal view returns(uint[] storage) {
        return values;
    }

    function test() public returns(uint[] memory) {
        values.push(10);
        values.push(20);
        values.push(30);
        values.push(40);
        values.push(50);
        removeByValue(30);
        uint i=find(40);
        removeByIndex(i);
        return getValues();
    }
}
