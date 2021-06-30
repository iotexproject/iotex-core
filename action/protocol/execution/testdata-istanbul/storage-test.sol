pragma solidity ^0.8.4;
contract tester {
    string[] public A;
    function storeStrings(string memory a,int n) public{
        for (int i=0;i<n;i++){
        A.push(a);
        }
    }
}
