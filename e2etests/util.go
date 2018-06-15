package e2etests

import (
	"math/big"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1 := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err := tsf1.Sign(ta.Addrinfo["miner"])
	tsf2 := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["miner"])
	tsf3 := action.NewTransfer(1, big.NewInt(50), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["miner"])
	tsf4 := action.NewTransfer(1, big.NewInt(70), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["miner"])
	tsf5 := action.NewTransfer(1, big.NewInt(110), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["miner"])
	tsf6 := action.NewTransfer(1, big.NewInt(50<<20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["miner"])

	blk, err := bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["charlie"])
	tsf5 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["delta"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["delta"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["delta"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["echo"])
	tsf2 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["echo"])
	tsf3 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["echo"])
	tsf4 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["echo"])
	tsf5 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["echo"])
	tsf6 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["echo"])
	blk, err = bc.MintNewBlock(nil, []*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}

	return nil
}

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
	blk, err := bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
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
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
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
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
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
	blk, err = bc.MintNewBlock([]*trx.Tx{tx}, nil, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.AddBlockCommit(blk); err != nil {
		return err
	}
	bc.ResetUTXO()

	return nil
}
