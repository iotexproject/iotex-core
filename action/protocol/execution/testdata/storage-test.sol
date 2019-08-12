pragma solidity >=0.4.22 <0.6.0;
contract tester {
    string[] public A;
    function storeStrings(string memory a,int n) public{
        for (int i=0;i<n;i++){
        A.push(a);
        }
    }
}