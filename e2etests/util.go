package e2etests

import (
	"github.com/iotexproject/iotex-core/blockchain"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingBlocks(bc *blockchain.Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	payee := []*blockchain.Payee{}
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].Address, 20})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].Address, 30})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["charlie"].Address, 50})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].Address, 70})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].Address, 110})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["foxtrot"].Address, 50 << 20})
	tx := bc.CreateTransaction(ta.Addrinfo["miner"], 280+50<<20, payee)
	blk := bc.MintNewBlock([]*blockchain.Tx{tx}, ta.Addrinfo["miner"].Address, "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	// Add block 2
	// Charlie --> A, B, D, E, test
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].Address, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].Address, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].Address, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].Address, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["miner"].Address, 1})
	tx = bc.CreateTransaction(ta.Addrinfo["charlie"], 5, payee)
	blk = bc.MintNewBlock([]*blockchain.Tx{tx}, ta.Addrinfo["miner"].Address, "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	// Add block 3
	// Delta --> B, E, F, test
	payee = payee[1:]
	payee[1] = &blockchain.Payee{ta.Addrinfo["echo"].Address, 1}
	payee[2] = &blockchain.Payee{ta.Addrinfo["foxtrot"].Address, 1}
	tx = bc.CreateTransaction(ta.Addrinfo["delta"], 4, payee)
	blk = bc.MintNewBlock([]*blockchain.Tx{tx}, ta.Addrinfo["miner"].Address, "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	// Add block 4
	// Delta --> A, B, C, D, F, test
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].Address, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].Address, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["charlie"].Address, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].Address, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["foxtrot"].Address, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["miner"].Address, 2})
	tx = bc.CreateTransaction(ta.Addrinfo["echo"], 12, payee)
	blk = bc.MintNewBlock([]*blockchain.Tx{tx}, ta.Addrinfo["miner"].Address, "")
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.Reset()

	return nil
}
