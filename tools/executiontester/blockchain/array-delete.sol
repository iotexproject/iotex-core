pragma solidity 0.4.24;

contract ArrayDelete {
    uint[] numbers;

    function main() returns (uint[]) {
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