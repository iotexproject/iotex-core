pragma solidity ^0.4.24;
contract One{
    string public word;

    function setMsg(string whatever) {
        word = whatever;
    }
    function getMsg() returns(string) {
        return word;
    }
}

contract Two{
    One o;
   function Two(address one){
       o = One(one);
       o.setMsg("test");
   }
   function getMsg() returns(string){
       return o.getMsg();
   }
}