package e2etests

import (
	"github.com/iotexproject/iotex-core/blockchain"
	trx "github.com/iotexproject/iotex-core/blockchain/trx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	payee := []*blockchain.Payee{}
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].RawAddress, 20})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].RawAddress, 30})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["charlie"].RawAddress, 50})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].RawAddress, 70})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].RawAddress, 110})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["foxtrot"].RawAddress, 50 << 20})
	tx := bc.CreateTransaction(ta.Addrinfo["miner"], 280+50<<20, payee)
	blk, err := bc.MintNewBlock([]*trx.Tx{tx}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 2
	// Charlie --> A, B, D, E, test
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].RawAddress, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].RawAddress, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].RawAddress, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["echo"].RawAddress, 1})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["miner"].RawAddress, 1})
	tx = bc.CreateTransaction(ta.Addrinfo["charlie"], 5, payee)
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 3
	// Delta --> B, E, F, test
	payee = payee[1:]
	payee[1] = &blockchain.Payee{ta.Addrinfo["echo"].RawAddress, 1}
	payee[2] = &blockchain.Payee{ta.Addrinfo["foxtrot"].RawAddress, 1}
	tx = bc.CreateTransaction(ta.Addrinfo["delta"], 4, payee)
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	// Add block 4
	// Delta --> A, B, C, D, F, test
	payee = nil
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["alfa"].RawAddress, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["bravo"].RawAddress, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["charlie"].RawAddress, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["delta"].RawAddress, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["foxtrot"].RawAddress, 2})
	payee = append(payee, &blockchain.Payee{ta.Addrinfo["miner"].RawAddress, 2})
	tx = bc.CreateTransaction(ta.Addrinfo["echo"], 12, payee)
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	return nil
}
