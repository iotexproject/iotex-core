pragma solidity ^0.8.4;
contract One{
    string public word;

    function setMsg(string calldata whatever) public {
        word = whatever;
    }
    function getMsg() public returns(string memory) {
        return word;
    }
}

contract Two{
    One o;
   constructor(address one){
       o = One(one);
       o.setMsg("test");
   }
   function getMsg() public returns(string memory){
       return o.getMsg();
   }
}
