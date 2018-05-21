# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""The Python implementation of the GRPC simulator.Simulator client."""

from __future__ import print_function

import grpc

import simulator_pb2
import simulator_pb2_grpc


def run():
    channel = grpc.insecure_channel('localhost:50051')
    stub = simulator_pb2_grpc.SimulatorStub(channel)
    response = stub.Ping(simulator_pb2.Request(playerID=1,
                                               senderID=9,
                                               messageType=4,
                                               value="block 44"))

    for i in response:
        print(i)

if __name__ == '__main__':
    run()
