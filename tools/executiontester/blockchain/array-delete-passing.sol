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
    uint[] numbers;
    function makeA() returns (uint256) {
        uint256[] numbers;
        numbers.push(10);

        A a = new A(numbers);

        return a.numbers(0);
    }
    function getArray() returns (uint[]) {
            numbers.push(100);
            numbers.push(200);
            numbers.push(300);
            numbers.push(400);
            numbers.push(500);

            delete numbers[2];

            // 100, 200, 0, 400, 500
            return numbers;
        }
}