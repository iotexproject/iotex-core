// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// HashSHA256 will compute a cryptographically useful hash of the input string.
func HashSHA256(input []byte) []byte {

	data := sha256.Sum256(input)
	return data[:]

}

// Decrypt Takes two strings, cryptoText and key.
// cryptoText is the text to be decrypted and the key is the key to use for the decryption.
// The function will output the resulting plain text string with an error variable.
func Decrypt(cryptoText, key []byte) (plainText []byte, err error) {

	if len(cryptoText) < aes.BlockSize {
		return nil, fmt.Errorf("cipherText too short. It decodes to %v bytes but the minimum length is 16", len(cryptoText))
	}

	return decryptAES(HashSHA256(key), cryptoText)
}

func decryptAES(key, data []byte) ([]byte, error) {
	// split the input up in to the IV seed and then the actual encrypted data.
	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCFBDecrypter(block, iv)

	stream.XORKeyStream(data, data)
	return data, nil
}

// Encrypt Takes two string, plainText and key.
// plainText is the text that needs to be encrypted by key.
// The function will output the resulting crypto text and an error variable.
func Encrypt(plainText, key []byte) (cipherText []byte, err error) {

	return encryptAES(HashSHA256(key), plainText)
}

func encryptAES(key, data []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// create two 'windows' in to the output slice.
	output := make([]byte, aes.BlockSize+len(data))
	iv := output[:aes.BlockSize]
	encrypted := output[aes.BlockSize:]

	// populate the IV slice with random data.
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)

	// note that encrypted is still a window in to the output slice
	stream.XORKeyStream(encrypted, data)
	return output, nil
}
