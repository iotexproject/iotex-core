pragma solidity 0.4.24;
contract Contract {
    uint[] public values;

    function Contract() {
    }

    function find(uint value) returns(uint) {
        uint i = 0;
        while (values[i] != value) {
            i++;
        }
        return i;
    }

    function removeByValue(uint value) {
        uint i = find(value);
        removeByIndex(i);
    }

    function removeByIndex(uint i) {
        while (i<values.length-1) {
            values[i] = values[i+1];
            i++;
        }
        values.length--;
    }

    function getValues() constant returns(uint[]) {
        return values;
    }

    function test() returns(uint[]) {
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