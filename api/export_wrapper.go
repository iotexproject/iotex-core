// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// export_wrapper.go export some private functions/types to integration test
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
	FilterObjectWrapper = filterObject
	ServerV2Wrapper     = ServerV2
	CoreServiceWrapper  = coreService
	HTTPHandlerWrapper  = hTTPHandler
)

var (
	EthAddrToIoAddrWrapper = ethAddrToIoAddr
	HexToBytesWrapper      = hexToBytes
	NewCoreServiceWrapper  = newCoreService
	NewGRPCHandlerWrapper  = newGRPCHandler
	NewHTTPHandlerWrapper  = newHTTPHandler
	IoAddrToEthAddrWrapper = ioAddrToEthAddr
	Uint64ToHexWrapper     = uint64ToHex
)

func (w ServerV2Wrapper) Core() CoreService                { return w.core }
func (w ServerV2Wrapper) GrpcServer() *GRPCServer          { return w.grpcServer }
func (w ServerV2Wrapper) HttpSvr() *HTTPServer             { return w.httpSvr }
func (w ServerV2Wrapper) WebsocketSvr() *HTTPServer        { return w.websocketSvr }
func (w ServerV2Wrapper) Tracer() *tracesdk.TracerProvider { return w.tracer }

func (s *CoreServiceWrapper) SetBc(bc blockchain.Blockchain)          { s.bc = bc }
func (s *CoreServiceWrapper) SetBroadcastHandler(b BroadcastOutbound) { s.broadcastHandler = b }
func (s *CoreServiceWrapper) ReadCache() *ReadCache                   { return s.readCache }
