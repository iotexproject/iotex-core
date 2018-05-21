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

    def processMessage(self, playerID, senderID, messageType, value):
        response = stub.Ping(simulator_pb2.Request(playerID=playerID,
                                                   senderID=senderID,
                                                   messageType=messageType,
                                                   value=value))
        return response
