const Multisend = artifacts.require('Multisend.sol');

contract('Multisend', function (accounts) {
  beforeEach(async function(){
    this.multisend = await Multisend.new();
  })
  describe('multisend',function(){
    let recipients1 = ["0x709b248f7B839078724B4E40904fBD7B694ba1Dc", "0xB9b727B4B37157103541FAeF8E8A48451d7aC18D"];
    let recipients2 = ["0xB9b727B4B37157103541FAeF8E8A48451d7aC18D", "0xB9b727B4B37157103541FAeF8E8A48451d7aC18D"];
    let recipients3 = [];
    for (let i = 1; i <= 301; i++) {
      recipients3.push("0xB9b727B4B37157103541FAeF8E8A48451d7aC18D");
    }
    let amounts1 = [100, 200];
    let amounts2 = [];
    for (let i = 1; i <= 301; i++) {
      amounts2.push(i+1);
    }
    let amounts3 = [100, 0]
    let payload1 = "";
    let payload2 = "testPayload";
    let wrongRecipients1 = ["0x709b248f7B839078724B4E40904fBD7B694ba1Dc",
      "0xB9b727B4B37157103541FAeF8E8A48451d7aC18D", "0xB9b727B4B37157103541FAeF8E8A48451d7aC18D"];
    let wrongRecipients2 = ["0xB9b727B4B37157103541FAeF8E8A48451d7aC18C", "0xB9b727B4B37157103541FAeF8E8A48451d7aC18D"];
    let wrongAmounts1 = [100, 200, 400];
    let wrongAmounts2 = [100, -200];
    let emptyArray = [];
    let tx;
    it('should emit two Transfer events and one payload event', async function(){
      tx = await this.multisend.multiSend(recipients1, amounts1, payload1, { from: accounts[0], value: 300 });
      const { logs } = tx;
      assert.equal(logs.length, 3);
      for (let i = 0; i < 2; i++){
        assert.equal(logs[i].event,'Transfer');
        assert.equal(logs[i].args.recipient, recipients1[i]);
        assert.equal(logs[i].args.amount, amounts1[i]);
      }
      assert.equal(logs[2].event,'Payload');
      assert.equal(logs[2].args.payload, payload1);
    })
    it('should emit two Transfer events and one payload event', async function(){
      tx = await this.multisend.multiSend(recipients2, amounts1, payload2, { from: accounts[0], value: 300 });
      const { logs } = tx;
      assert.equal(logs.length, 3);
      for (let i = 0; i < 2; i++){
        assert.equal(logs[i].event,'Transfer');
        assert.equal(logs[i].args.recipient, recipients2[i]);
        assert.equal(logs[i].args.amount, amounts1[i]);
      }
      assert.equal(logs[2].event,'Payload');
      assert.equal(logs[2].args.payload, payload2);
    })
    it('should emit two Transfer events and one payload event', async function(){
      tx = await this.multisend.multiSend(recipients2, amounts3, payload2, { from: accounts[0], value: 100 });
      const { logs } = tx;
      assert.equal(logs.length, 3);
      for (let i = 0; i < 2; i++){
        assert.equal(logs[i].event,'Transfer');
        assert.equal(logs[i].args.recipient, recipients2[i]);
        assert.equal(logs[i].args.amount, amounts3[i]);
      }
      assert.equal(logs[2].event,'Payload');
      assert.equal(logs[2].args.payload, payload2);
    })
    it('should emit two Transfer events and one Rufund event and one payload event', async function(){
      tx = await this.multisend.multiSend(recipients1, amounts1, payload2, { from: accounts[0], value: 315 });
      const { logs } = tx;
      assert.equal(logs.length, 4);
      for (let i = 0; i < 2; i++){
        assert.equal(logs[i].event,'Transfer');
        assert.equal(logs[i].args.recipient, recipients1[i]);
        assert.equal(logs[i].args.amount, amounts1[i]);
      }
      assert.equal(logs[2].event,'Refund');
      assert.equal(logs[2].args.refund, 15);
      assert.equal(logs[3].event,'Payload');
      assert.equal(logs[3].args.payload, payload2);
    })
    it('failed due to not enougn token', async function(){
      let Error;
      try {
        await this.multisend.multiSend(recipients1, amounts1, payload2, { from: accounts[0], value: 215 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('not enough token'), -1);
    })
    it('failed due to more recipients', async function(){
      let Error;
      try {
        await this.multisend.multiSend(wrongRecipients1, amounts1, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('parameters not match'), -1);
    })
    it('failed due to more amounts', async function(){
      let Error;
      try {
        await this.multisend.multiSend(recipients1, wrongAmounts1, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('parameters not match'), -1);

    })
    it('failed due to invalid recipient', async function(){
      let Error;
      try {
        await this.multisend.multiSend(wrongRecipients2, amounts1, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
    })
    it('failed due to invalid amounts', async function(){
      let Error;
      try {
        await this.multisend.multiSend(recipients1, wrongAmounts2, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
    })
    it('failed due to empty recipients', async function(){
      let Error;
      try {
        await this.multisend.multiSend(emptyArray, amounts1, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('parameters not match'), -1);
    })
    it('failed due to empty amounts', async function(){
      let Error;
      try {
        await this.multisend.multiSend(recipients1, emptyArray, payload2, { from: accounts[0], value: 300 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('parameters not match'), -1);
    })
    it('failed due to too many recipients', async function(){
      let Error;
      try {
        await this.multisend.multiSend(recipients3, amounts2, payload2, { from: accounts[0], value: 300000 });
        assert.fail();
      } catch (error) {
        Error = error;
      }
      assert.notEqual(Error, undefined, 'Exception thrown');
      assert.isAbove(Error.message.search('number of recipients is larger than 300'), -1);
    })
  })
})