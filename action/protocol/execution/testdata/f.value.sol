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
   function Two(){
       One o = One(0x675f1057f81e9e768e33faddbd5609c09f4c0a5c);
       o.setMsg("test");
   }
   function getMsg() returns(string){
       return o.getMsg();
   }
}