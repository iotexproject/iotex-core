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
  --nvalidators=<nval>            number of validators in the system [default: 5]
  --nproposers=<nprop>            number of proposers in the system [default: 1]
  --nconnections=<ncons>          number of connections in the system [default: 8]
  --nheartbeatsinround=<nhbts>    number of heartbeats (dt) in a round [default: 5]
  --nrounds=<nrounds>             number of rounds (proposal/validation/commit) in the simulation [default: 5]
  --ntransactions=<ntrans>        number of transactions per block [default: 3]
  --ptransactions=<ptrans>        probability of transaction per player per heartbeat [default: 0.1]
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
            "N_VALIDATORS":          int(args["--nvalidators"]),   
            "N_PROPOSERS":           int(args["--nproposers"]),     
            "N_CONNECTIONS":         int(args["--nconnections"]),   
            "N_HEARTBEATS_IN_ROUND": int(args["--nheartbeatsinround"]), 
            "N_ROUNDS":              int(args["--nrounds"]),
            "N_TRANSACTIONS":        int(args["--ntransactions"]),       
            "P_TRANSACTIONS":      float(args["--ptransactions"]),      
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
