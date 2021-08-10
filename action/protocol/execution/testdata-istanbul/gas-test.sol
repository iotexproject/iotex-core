pragma solidity ^0.8.4;
contract tester {
    string public A;
    event logTest(uint n);
    function test(uint mul,uint shift,uint add,uint log) public returns (uint a){
        a = 7;
        for (uint i=0;i<mul;i++){
            a = (a*10007)%100000007;
        }
        for (uint i=0;i<shift;i++){
            a = i<<7;
        }
        for (uint i=0;i<add;i++){
            a = (a + 100000009) % 10007;
        }
        for (uint i=0;i<log;i++){
            emit logTest(i);
        }
    }

    function storeString(string memory a) public {
        A = a;
    }
}
