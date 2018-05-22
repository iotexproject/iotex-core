# Copyright 2015 gRPC authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""The Python implementation of the GRPC simulator.Simulator client."""

import grpc

from proto import simulator_pb2
from proto import simulator_pb2_grpc


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
