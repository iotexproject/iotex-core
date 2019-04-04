pragma solidity 0.4.24;
contract A {
    uint256[] public numbers;
    function A(uint256[] _numbers) {
        for(uint256 i=0; i<_numbers.length; i++) {
            numbers.push(_numbers[i]);
        }
    }

    function get() returns (uint256[]) {
        return numbers;
    }
}

contract Manager {
    function makeA() returns (uint256) {
        uint256[] numbers;
        numbers.push(10);

        A a = new A(numbers);

        return a.numbers(0);
    }
}