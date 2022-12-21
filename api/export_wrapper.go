// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package api export_wrapper.go export some private functions/types to integration test
// it's a temporary solution to solve these two problems without any modification of the test code logic
//  1. circular dependency between the config and api package
//  2. integration test moved out of package need to access package private functions/types
//
// it should be deprecated after integration test been refactored such as remove access to private things.
// non integration test code should never access thie file.
package api

import (
	"github.com/iotexproject/iotex-core/blockchain"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

type (
	// FilterObjectWrapper export filterObject
	FilterObjectWrapper = filterObject
	// ServerV2Wrapper export ServerV2
	ServerV2Wrapper = ServerV2
	// CoreServiceWrapper export coreService
	CoreServiceWrapper = coreService
	// HTTPHandlerWrapper export hTTPHandler
	HTTPHandlerWrapper = hTTPHandler
)

var (
	// EthAddrToIoAddrWrapper export ethAddrToIoAddr
	EthAddrToIoAddrWrapper = ethAddrToIoAddr
	// HexToBytesWrapper export hexToBytes
	HexToBytesWrapper = hexToBytes
	// NewCoreServiceWrapper export newCoreService
	NewCoreServiceWrapper = newCoreService
	// NewGRPCHandlerWrapper export newGRPCHandler
	NewGRPCHandlerWrapper = newGRPCHandler
	// NewHTTPHandlerWrapper export newHTTPHandler
	NewHTTPHandlerWrapper = newHTTPHandler
	// IoAddrToEthAddrWrapper export ioAddrToEthAddr
	IoAddrToEthAddrWrapper = ioAddrToEthAddr
	// Uint64ToHexWrapper export uint64ToHex
	Uint64ToHexWrapper = uint64ToHex
)

// Core export ServerV2.core
func (w ServerV2Wrapper) Core() CoreService { return w.core }

// GRPCServer export ServerV2.grpcServer
func (w ServerV2Wrapper) GRPCServer() *GRPCServer { return w.grpcServer }

// HTTPSvr export ServerV2.httpSvr
func (w ServerV2Wrapper) HTTPSvr() *HTTPServer { return w.httpSvr }

// WebsocketSvr export ServerV2.websocketSvr
func (w ServerV2Wrapper) WebsocketSvr() *HTTPServer { return w.websocketSvr }

// Tracer export ServerV2.tracer
func (w ServerV2Wrapper) Tracer() *tracesdk.TracerProvider { return w.tracer }

// SetBc export coreService.bc
func (s *CoreServiceWrapper) SetBc(bc blockchain.Blockchain) { s.bc = bc }

// SetBroadcastHandler export coreService.broadcastHandler
func (s *CoreServiceWrapper) SetBroadcastHandler(b BroadcastOutbound) { s.broadcastHandler = b }

// ReadCache export coreService.readCache
func (s *CoreServiceWrapper) ReadCache() *ReadCache { return s.readCache }
