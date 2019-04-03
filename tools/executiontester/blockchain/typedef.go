// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

const (
	// Erc721Binary a simple erc721 token bin, the following is its binary
	Erc721Binary = "60806040523480156200001157600080fd5b5060408051808201825260068082527f4e6674696573000000000000000000000000000000000000000000000000000060208084019190915283518085019094529083527f4e465449455300000000000000000000000000000000000000000000000000009083015290620000af7f01ffc9a700000000000000000000000000000000000000000000000000000000640100000000620001b3810204565b620000e37f80ac58cd00000000000000000000000000000000000000000000000000000000640100000000620001b3810204565b620001177f4f558e7900000000000000000000000000000000000000000000000000000000640100000000620001b3810204565b81516200012c90600590602085019062000220565b5080516200014290600690602084019062000220565b50620001777f780e9d6300000000000000000000000000000000000000000000000000000000640100000000620001b3810204565b620001ab7f5b5e139f00000000000000000000000000000000000000000000000000000000640100000000620001b3810204565b5050620002c5565b7fffffffff000000000000000000000000000000000000000000000000000000008082161415620001e357600080fd5b7fffffffff00000000000000000000000000000000000000000000000000000000166000908152602081905260409020805460ff19166001179055565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106200026357805160ff191683800117855562000293565b8280016001018555821562000293579182015b828111156200029357825182559160200191906001019062000276565b50620002a1929150620002a5565b5090565b620002c291905b80821115620002a15760008155600101620002ac565b90565b611b2480620002d56000396000f3006080604052600436106101275763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166301ffc9a7811461012c57806306fdde0314610162578063081812fc146101ec578063095ea7b31461022057806318160ddd1461024657806319fa8f501461026d57806323b872dd1461029f5780632f745c59146102c957806342842e0e146102ed5780634f558e79146103175780634f6ccce71461032f5780636352211e1461034757806370a082311461035f5780638462151c146103805780639507d39a146103f157806395d89b411461045f578063a22cb46514610474578063b88d4fde1461049a578063c87b56dd14610509578063e985e9c514610521578063efc81a8c14610548578063fdb05e851461055d575b600080fd5b34801561013857600080fd5b5061014e600160e060020a031960043516610581565b604080519115158252519081900360200190f35b34801561016e57600080fd5b506101776105a0565b6040805160208082528351818301528351919283929083019185019080838360005b838110156101b1578181015183820152602001610199565b50505050905090810190601f1680156101de5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156101f857600080fd5b50610204600435610637565b60408051600160a060020a039092168252519081900360200190f35b34801561022c57600080fd5b50610244600160a060020a0360043516602435610652565b005b34801561025257600080fd5b5061025b610708565b60408051918252519081900360200190f35b34801561027957600080fd5b5061028261070e565b60408051600160e060020a03199092168252519081900360200190f35b3480156102ab57600080fd5b50610244600160a060020a0360043581169060243516604435610732565b3480156102d557600080fd5b5061025b600160a060020a03600435166024356107d5565b3480156102f957600080fd5b50610244600160a060020a0360043581169060243516604435610822565b34801561032357600080fd5b5061014e600435610843565b34801561033b57600080fd5b5061025b600435610860565b34801561035357600080fd5b50610204600435610895565b34801561036b57600080fd5b5061025b600160a060020a03600435166108bf565b34801561038c57600080fd5b506103a1600160a060020a03600435166108f2565b60408051602080825283518183015283519192839290830191858101910280838360005b838110156103dd5781810151838201526020016103c5565b505050509050019250505060405180910390f35b3480156103fd57600080fd5b5061040960043561095e565b60408051600160a060020a03909816885260ff9687166020890152948616878601529285166060870152908416608086015290921660a084015267ffffffffffffffff90911660c0830152519081900360e00190f35b34801561046b57600080fd5b50610177610a91565b34801561048057600080fd5b50610244600160a060020a03600435166024351515610af2565b3480156104a657600080fd5b50604080516020601f60643560048181013592830184900484028501840190955281845261024494600160a060020a038135811695602480359092169560443595369560849401918190840183828082843750949750610b769650505050505050565b34801561051557600080fd5b50610177600435610b9e565b34801561052d57600080fd5b5061014e600160a060020a0360043581169060243516610c49565b34801561055457600080fd5b5061025b610c77565b34801561056957600080fd5b50610244600160a060020a0360043516602435610ffb565b600160e060020a03191660009081526020819052604090205460ff1690565b60058054604080516020601f600260001961010060018816150201909516949094049384018190048102820181019092528281526060939092909183018282801561062c5780601f106106015761010080835404028352916020019161062c565b820191906000526020600020905b81548152906001019060200180831161060f57829003601f168201915b505050505090505b90565b600090815260026020526040902054600160a060020a031690565b600061065d82610895565b9050600160a060020a03838116908216141561067857600080fd5b33600160a060020a038216148061069457506106948133610c49565b151561069f57600080fd5b600082815260026020526040808220805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0387811691821790925591518593918516917f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b92591a4505050565b60095490565b7f01ffc9a70000000000000000000000000000000000000000000000000000000081565b61073c3382611043565b151561074757600080fd5b600160a060020a038316151561075c57600080fd5b600160a060020a038216151561077157600080fd5b61077b83826110a2565b6107858382611113565b61078f828261121a565b8082600160a060020a031684600160a060020a03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef60405160405180910390a4505050565b60006107e0836108bf565b82106107eb57600080fd5b600160a060020a038316600090815260076020526040902080548390811061080f57fe5b9060005260206000200154905092915050565b61083e8383836020604051908101604052806000815250610b76565b505050565b600090815260016020526040902054600160a060020a0316151590565b600061086a610708565b821061087557600080fd5b600980548390811061088357fe5b90600052602060002001549050919050565b600081815260016020526040812054600160a060020a03168015156108b957600080fd5b92915050565b6000600160a060020a03821615156108d657600080fd5b50600160a060020a031660009081526003602052604090205490565b600160a060020a03811660009081526007602090815260409182902080548351818402810184019094528084526060939283018282801561095257602002820191906000526020600020905b81548152602001906001019080831161093e575b50505050509050919050565b600081815260016020526040812054600c8054839283928392839283928392600160a060020a03909216918a90811061099357fe5b600091825260209091200154600c805460ff909216918b9081106109b357fe5b9060005260206000200160000160019054906101000a900460ff16600c8b8154811015156109dd57fe5b9060005260206000200160000160029054906101000a900460ff16600c8c815481101515610a0757fe5b9060005260206000200160000160039054906101000a900460ff16600c8d815481101515610a3157fe5b9060005260206000200160000160049054906101000a900460ff16600c8e815481101515610a5b57fe5b600091825260209091200154959e949d50929b50909950975095506501000000000090910467ffffffffffffffff169350915050565b60068054604080516020601f600260001961010060018816150201909516949094049384018190048102820181019092528281526060939092909183018282801561062c5780601f106106015761010080835404028352916020019161062c565b600160a060020a038216331415610b0857600080fd5b336000818152600460209081526040808320600160a060020a03871680855290835292819020805460ff1916861515908117909155815190815290519293927f17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31929181900390910190a35050565b610b81848484610732565b610b8d84848484611263565b1515610b9857600080fd5b50505050565b6060610ba982610843565b1515610bb457600080fd5b6000828152600b602090815260409182902080548351601f6002600019610100600186161502019093169290920491820184900484028101840190945280845290918301828280156109525780601f10610c1c57610100808354040283529160200191610952565b820191906000526020600020905b815481529060010190602001808311610c2a5750939695505050505050565b600160a060020a03918216600090815260046020908152604080832093909416825291909152205460ff1690565b60008060008060008060006060610c8c611a07565b610c94610708565b6040805160208082019390935260001943014081830152815180820383018152606090910191829052805190928291908401908083835b60208310610cea5780518252601f199092019160209182019101610ccb565b5181516020939093036101000a600019018019909116921691909117905260405192018290039091209a5060059250505060f860020a60008a901a81020460ff16066001908101975060059089901a60f860020a0260f860020a900460ff16811515610d5257fe5b066001019550600560f860020a60028a901a81020460ff16066001019450600560f860020a60038a901a81020460ff16066001019350600560f860020a60048a901a81020460ff16066001019250610dad87878787876113d0565b6040805160c08101825260ff808b16825289811660208301908152898216938301938452888216606084019081528883166080850190815267ffffffffffffffff43811660a08701908152600c805460018101825560009190915287517fdf6966c971051c3d54ec59162606531493a51404a002842f56009d7e5cf4a8c78201805497519a5196519551935190941665010000000000026cffffffffffffffff0000000000199389166401000000000264ff0000000019968a1663010000000263ff00000019988b16620100000262ff0000199d8c166101000261ff001995909c1660ff19909b169a909a1793909316999099179a909a16969096179490941694909417919091169390931791909116939093179055909a509092509050610ed5338a61152d565b610edf898361157c565b33600160a060020a03167f1ae41272ced32fa050dc3df49761e279866a7a9378212de7a61104821698f18c8a89898989898860a001518a604051808981526020018860ff1660ff1681526020018760ff1660ff1681526020018660ff1660ff1681526020018560ff1660ff1681526020018460ff1660ff1681526020018367ffffffffffffffff1667ffffffffffffffff16815260200180602001828103825283818151815260200191508051906020019080838360005b83811015610faf578181015183820152602001610f97565b50505050905090810190601f168015610fdc5780820380516001836020036101000a031916815260200191505b50995050505050505050505060405180910390a2505050505050505090565b60408051808201909152600481527f74657374000000000000000000000000000000000000000000000000000000006020820152611039838361152d565b61083e828261157c565b60008061104f83610895565b905080600160a060020a031684600160a060020a0316148061108a575083600160a060020a031661107f84610637565b600160a060020a0316145b8061109a575061109a8185610c49565b949350505050565b81600160a060020a03166110b582610895565b600160a060020a0316146110c857600080fd5b600081815260026020526040902054600160a060020a03161561110f576000818152600260205260409020805473ffffffffffffffffffffffffffffffffffffffff191690555b5050565b600080600061112285856115af565b600084815260086020908152604080832054600160a060020a038916845260079092529091205490935061115d90600163ffffffff61164516565b600160a060020a03861660009081526007602052604090208054919350908390811061118557fe5b90600052602060002001549050806007600087600160a060020a0316600160a060020a03168152602001908152602001600020848154811015156111c557fe5b6000918252602080832090910192909255600160a060020a03871681526007909152604090208054906111fc906000198301611a3c565b50600093845260086020526040808520859055908452909220555050565b60006112268383611657565b50600160a060020a039091166000908152600760209081526040808320805460018101825590845282842081018590559383526008909152902055565b60008061127885600160a060020a03166116e7565b151561128757600191506113c7565b6040517f150b7a020000000000000000000000000000000000000000000000000000000081523360048201818152600160a060020a03898116602485015260448401889052608060648501908152875160848601528751918a169463150b7a0294938c938b938b93909160a490910190602085019080838360005b8381101561131a578181015183820152602001611302565b50505050905090810190601f1680156113475780820380516001836020036101000a031916815260200191505b5095505050505050602060405180830381600087803b15801561136957600080fd5b505af115801561137d573d6000803e3d6000fd5b505050506040513d602081101561139357600080fd5b5051600160e060020a031981167f150b7a020000000000000000000000000000000000000000000000000000000014925090505b50949350505050565b6040805180820190915260208082527f68747470733a2f2f6e66746965732e696f2f746f6b656e732f6e66746965732d9082015260609061141181886116ef565b90506114398160408051908101604052806001815260200160f860020a602d02815250611880565b905061144581876116ef565b905061146d8160408051908101604052806001815260200160f860020a602d02815250611880565b905061147981866116ef565b90506114a18160408051908101604052806001815260200160f860020a602d02815250611880565b90506114ad81856116ef565b90506114d58160408051908101604052806001815260200160f860020a602d02815250611880565b90506114e181846116ef565b9050611522816040805190810160405280600481526020017f2e706e6700000000000000000000000000000000000000000000000000000000815250611880565b979650505050505050565b611537828261199f565b600980546000838152600a60205260408120829055600182018355919091527f6e1540171b6c0c960b71a7020d9f60077f6af931a8bbf590da0223dacf75c7af015550565b61158582610843565b151561159057600080fd5b6000828152600b60209081526040909120825161083e92840190611a60565b81600160a060020a03166115c282610895565b600160a060020a0316146115d557600080fd5b600160a060020a0382166000908152600360205260409020546115ff90600163ffffffff61164516565b600160a060020a03909216600090815260036020908152604080832094909455918152600190915220805473ffffffffffffffffffffffffffffffffffffffff19169055565b60008282111561165157fe5b50900390565b600081815260016020526040902054600160a060020a03161561167957600080fd5b6000818152600160208181526040808420805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03881690811790915584526003909152909120546116c7916119fa565b600160a060020a0390921660009081526003602052604090209190915550565b6000903b1190565b60408051606480825260a0820190925260609190829060009081908390819083908760208201610c8080388339019050509550600094505b60ff89161561177c578551600a60ff9a8b168181049b60018901989290910616955060f860020a603087010291889190811061175f57fe5b906020010190600160f860020a031916908160001a905350611727565b899250848351016040519080825280601f01601f1916602001820160405280156117b0578160200160208202803883390190505b509150600090505b82518110156118105782818151811015156117cf57fe5b90602001015160f860020a900460f860020a0282828151811015156117f057fe5b906020010190600160f860020a031916908160001a9053506001016117b8565b5060005b84811015611873578581600187030381518110151561182f57fe5b90602001015160f860020a900460f860020a02828451830181518110151561185357fe5b906020010190600160f860020a031916908160001a905350600101611814565b5098975050505050505050565b606080606080606060008088955087945084518651016040519080825280601f01601f1916602001820160405280156118c3578160200160208202803883390190505b50935083925060009150600090505b85518110156119305785818151811015156118e957fe5b90602001015160f860020a900460f860020a02838380600101945081518110151561191057fe5b906020010190600160f860020a031916908160001a9053506001016118d2565b5060005b845181101561199257848181518110151561194b57fe5b90602001015160f860020a900460f860020a02838380600101945081518110151561197257fe5b906020010190600160f860020a031916908160001a905350600101611934565b5090979650505050505050565b600160a060020a03821615156119b457600080fd5b6119be828261121a565b6040518190600160a060020a038416906000907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef908290a45050565b818101828110156108b957fe5b6040805160c081018252600080825260208201819052918101829052606081018290526080810182905260a081019190915290565b81548183558181111561083e5760008381526020902061083e918101908301611ade565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10611aa157805160ff1916838001178555611ace565b82800160010185558215611ace579182015b82811115611ace578251825591602001919060010190611ab3565b50611ada929150611ade565b5090565b61063491905b80821115611ada5760008155600101611ae45600a165627a7a72305820eb6ef4caf7e8dfb40d659802d0d7106ab4b338205d58c4b1d0596c47c740a77e0029"
	// CreateTo createTo(address _owner,uint256 _tokenId)
	CreateTo = "fdb05e85"
	// TransferFrom transferFrom(address _from, address _to, uint256 _tokenId)
	TransferFrom = "23b872dd"
	// BalanceOf BalanceOf(opts *bind.CallOpts, _owner common.Address)
	BalanceOf = "70a08231"
	// ArrayDeleteBin is a test contract, source code in array-delete.sol, the following is its binary
	ArrayDeleteBin = `608060405234801561001057600080fd5b506101b6806100206000396000f3006080604052600436106100405763ffffffff7c0100000000000000000000000000000000000000000000000000000000600035041663dffeadd08114610045575b600080fd5b34801561005157600080fd5b5061005a6100aa565b60408051602080825283518183015283519192839290830191858101910280838360005b8381101561009657818101518382015260200161007e565b505050509050019250505060405180910390f35b600080546001818101835582805260647f290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563928301558254808201845560c8908301558254808201845561012c908301558254808201845561019090830155825490810183556101f4910155805460609190600290811061012657fe5b9060005260206000200160009055600080548060200260200160405190810160405280929190818152602001828054801561018057602002820191906000526020600020905b81548152602001906001019080831161016c575b50505050509050905600a165627a7a723058208aa9d63499b5c61b7afe28978ee8c5d5b2b55a814a7cbc5ebf426b87115075cf0029`
	// ArrayDeleteMain is array delete contract method
	ArrayDeleteMain = "dffeadd0"
	// ArrayStringBin is a test contract, source code in array-of-strings.sol, the following is its binary
	ArrayStringBin = `608060405234801561001057600080fd5b5060008054600181018083559180526040805180820190915260028082527f686900000000000000000000000000000000000000000000000000000000000060209092019182526100729260008051602061032b8339815191520191906100dc565b505060008054600181018083559180526040805180820190915260038082527f627965000000000000000000000000000000000000000000000000000000000060209092019182526100d59260008051602061032b8339815191520191906100dc565b5050610177565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061011d57805160ff191683800117855561014a565b8280016001018555821561014a579182015b8281111561014a57825182559160200191906001019061012f565b5061015692915061015a565b5090565b61017491905b808211156101565760008155600101610160565b90565b6101a5806101866000396000f3006080604052600436106100405763ffffffff7c0100000000000000000000000000000000000000000000000000000000600035041663febb0f7e8114610045575b600080fd5b34801561005157600080fd5b5061005a6100cf565b6040805160208082528351818301528351919283929083019185019080838360005b8381101561009457818101518382015260200161007c565b50505050905090810190601f1680156100c15780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6060600060018154811015156100e157fe5b600091825260209182902001805460408051601f600260001961010060018716150201909416939093049283018590048502810185019091528181529283018282801561016f5780601f106101445761010080835404028352916020019161016f565b820191906000526020600020905b81548152906001019060200180831161015257829003601f168201915b50505050509050905600a165627a7a72305820088b1228240c10b8bfe93f963f27bb2616ef40476aa512bb11baaa5ff83bb63c0029290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563`
	// ArrayStringBar is string return contract method
	ArrayStringBar = "febb0f7e"
)
