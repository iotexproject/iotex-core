package mock_blockchain

//go:generate mockgen -package=mock_blockchain -destination=./mock_blockchain.go -source=../../../blockchain/blockchain.go Blockchain
