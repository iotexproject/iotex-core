// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"math/rand"
	"sync"

	"github.com/golang/glog"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-core/config"
)

// stringsAreShuffled shuffles a string slice
func stringsAreShuffled(slice []string) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// lenSyncMap counts the length of a sync.map
func lenSyncMap(m sync.Map) uint {
	len := uint(0)
	m.Range(func(_, _ interface{}) bool {
		len++
		return true
	})
	return len
}

func loadCertAndCertPool(config *config.Network) (*tls.Certificate, *x509.CertPool, error) {
	// Load the certificates from disk
	cert, err := tls.LoadX509KeyPair(config.PeerCrtPath, config.PeerKeyPath)
	if err != nil {
		glog.Errorf("could not load peer key pair: %v", err)
		return nil, nil, err
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(config.CACrtPath)
	if err != nil {
		glog.Errorf("could not read ca certificate: %v", err)
		return nil, nil, err
	}

	// Append the peer certificates from the CA
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, nil, errors.New("failed to append peer certs")
	}
	return &cert, certPool, nil
}

func generateServerCredentials(config *config.Network) (credentials.TransportCredentials, error) {
	cert, certPool, err := loadCertAndCertPool(config)
	if err != nil {
		return nil, err
	}

	// Return the server TLS credentials
	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{*cert},
		ClientCAs:    certPool,
	}), nil
}

func generateClientCredentials(config *config.Network) (credentials.TransportCredentials, error) {
	cert, certPool, err := loadCertAndCertPool(config)
	if err != nil {
		return nil, err
	}

	// Return the client TLS credentials
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{*cert},
		RootCAs:      certPool,
	}), nil
}
