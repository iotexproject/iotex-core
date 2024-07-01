package mock_crypto

//go:generate mockgen -package=mock_crypto -destination=./mock_crypto_publickey.go  github.com/iotexproject/go-pkgs/crypto PublicKey
//go:generate mockgen -package=mock_crypto -destination=./mock_crypto_privatekey.go github.com/iotexproject/go-pkgs/crypto PrivateKey
