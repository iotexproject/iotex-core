// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package did

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/pkg/util/addrutil"
	"golang.org/x/crypto/sha3"
)

type (
	// Permit permit content for DID
	Permit struct {
		Separator  string `json:"DOMAIN_SEPARATOR"`
		PermitHash string `json:"permitHash"`
	}

	// Signature signature for typed message
	Signature struct {
		R string `json:"r"`
		S string `json:"s"`
		V uint64 `json:"v"`
	}

	// CreateRequest create DID request
	CreateRequest struct {
		Signature
		PublicKey string `json:"publicKey"`
	}

	// ServiceAddRequest add service to DID request
	ServiceAddRequest struct {
		Signature
		Tag             string `json:"tag"`
		Type            string `json:"type"`
		ServiceEndpoint string `json:"serviceEndpoint"`
	}

	// ServiceRemoveRequest remove service from DID request
	ServiceRemoveRequest struct {
		Signature
		Tag string `json:"tag"`
	}
)

// getPermit fetch DID permit from resolver
func getPermit(endpoint, address string) (*Permit, error) {
	resp, err := http.Get(endpoint + "/did/" + address + "/permit")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data Permit
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// signType sign typed message
func signType(key *ecdsa.PrivateKey, separator, hash string) (*Signature, error) {
	separatorBytes, err := hexutil.Decode(separator)
	if err != nil {
		return nil, err
	}
	hashBytes, err := hexutil.Decode(hash)
	if err != nil {
		return nil, err
	}

	data := append([]byte{0x19, 0x01}, append(separatorBytes, hashBytes...)...)
	sha := sha3.NewLegacyKeccak256()
	sha.Write(data)
	signHash := sha.Sum(nil)

	sig, err := crypto.Sign(signHash, key)
	if err != nil {
		return nil, err
	}
	v := new(big.Int).SetBytes([]byte{sig[64] + 27})

	return &Signature{
		R: hexutil.Encode(sig[:32]),
		S: hexutil.Encode(sig[32:64]),
		V: v.Uint64(),
	}, nil
}

// loadPrivateKey load private key and address from signer
func loadPrivateKey() (*ecdsa.PrivateKey, string, error) {
	signer, err := action.Signer()
	if err != nil {
		return nil, "", output.NewError(output.InputError, "failed to get signer addr", err)
	}
	fmt.Printf("Enter password #%s:\n", signer)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return nil, "", output.NewError(output.InputError, "failed to get password", err)
	}
	pri, err := account.PrivateKeyFromSigner(signer, password)
	if err != nil {
		return nil, "", output.NewError(output.InputError, "failed to decrypt key", err)
	}
	ethAddress, err := addrutil.IoAddrToEvmAddr(signer)
	if err != nil {
		return nil, "", output.NewError(output.AddressError, "", err)
	}

	return pri.EcdsaPrivateKey().(*ecdsa.PrivateKey), ethAddress.String(), nil
}

// loadPublicKey load public key by private key
func loadPublicKey(key *ecdsa.PrivateKey) ([]byte, error) {
	publicKey := key.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, output.NewError(output.ConvertError, "generate public key error", nil)
	}
	return crypto.FromECDSAPub(publicKeyECDSA), nil
}

// signPermit fetch permit and sign and return signature, publicKey and address
func signPermit(endpoint string) (*Signature, []byte, string, error) {
	key, addr, err := loadPrivateKey()
	if err != nil {
		return nil, nil, "", err
	}

	publicKey, err := loadPublicKey(key)
	if err != nil {
		return nil, nil, "", err
	}

	permit, err := getPermit(endpoint, addr)
	if err != nil {
		return nil, nil, "", output.NewError(output.InputError, "failed to fetch permit", err)
	}
	signature, err := signType(key, permit.Separator, permit.PermitHash)
	if err != nil {
		return nil, nil, "", output.NewError(output.InputError, "failed to sign typed permit", err)
	}
	return signature, publicKey, addr, nil
}

// postToResolver post data to resolver
func postToResolver(url string, reqBytes []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to create request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return output.NewError(output.NetworkError, "failed to post request", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to read response", err)
	}
	output.PrintResult(string(body))
	return nil
}
