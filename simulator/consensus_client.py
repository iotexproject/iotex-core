# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines a consensus client which interfaces with the RDPoS consensus engine in Go"""

import grpc

from proto import simulator_pb2_grpc
from proto import simulator_pb2

class Consensus:
    def __init__(self):
        self.channel = grpc.insecure_channel('localhost:50051')
        self.stub = simulator_pb2_grpc.SimulatorStub(self.channel)

    @staticmethod
    def initConsensus(nPlayers):
        """Sends a request to initialize n consensus schemes on the consensus server"""

        print("sent init request with %d nodes to consensus engine", nPlayers)
        channel = grpc.insecure_channel('localhost:50051')
        stub = simulator_pb2_grpc.SimulatorStub(channel)
        
        response = stub.Init(simulator_pb2.InitRequest(nPlayers=nPlayers))
        print("received %s from consensus enginer", list(response))

        response = [[r.playerID, (r.internalMsgType, r.value)] for r in response]

        return response

    def processMessage(self, internalMsgType, value):
        """Sends a view state change message to consensus engine and returns responses"""
        
        print("sent %s to consensus engine", value) 
        response = stub.Ping(simulator_pb2.Request(playerID=self.playerID,
                                                   internalMsgType=internalMsgType,
                                                   value=value))
        print("received %s from consensus engine", list(response))

        response = [[r.messageType, (r.internalMsgType, r.value)] for r in response]

        return response
