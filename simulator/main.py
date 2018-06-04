# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""main.py: takes in command line arguments and runs the driver with those options

Usage:
  main.py (--help | -h)
  main.py [options]

Options:
  --players=<players>             list of tuples e.g. "[(# of players, stake per player), ...]" [default: [(100, 1)]]
  --nconnections=<ncons>          number of connections in the system [default: 8]
  --timetosim=<timetosim>         virtual time to simulate the system for [default: 10]
  --meanproptime=<meanproptime>   mean propagation time of messages [default: 0.1]
  --seed=<seed>                   random seed [default: 42]
  --help                          show this
"""
import random

import grpc
from docopt     import docopt

import driver
import player
import solver

if __name__=="__main__":
    args = docopt(__doc__)
    opts = {"PLAYERS":              eval(args["--players"]),      
            "N_CONNECTIONS":         int(args["--nconnections"]),   
            "TIME_TO_SIM":           int(args["--timetosim"]),
            "MEAN_PROP_TIME":      float(args["--meanproptime"]),
            "SEED":                  int(args["--seed"])
           }

    """channel  = grpc.insecure_channel('localhost:50051')
    stub     = simulator_pb2_grpc.SimulatorStub(channel)
    response = stub.Ping(simulator_pb2.Request(playerID=1,
                                                     senderID=9,
                                                     messageType=4,
                                                     value="block 44"))

    for i in response:
        print(i)"""

    driver.drive(opts)
