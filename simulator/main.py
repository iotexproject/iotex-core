# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""main.py: takes in command line arguments and runs the driver with those options

Usage:
  main.py (--help | -h)
  main.py [options]

Options:
  --players=<players>             list of tuples e.g. "[(# of players, player type), ...]"; player type 0 = honest node, 1 = failure stop, 2 = fuzz test, 3 = byzantine fault [default: [(10, 0)]]
  --nconnections=<ncons>          number of connections per person in the network [default: 3]
  --timetosim=<timetosim>         virtual time in seconds to simulate the system for [default: 10]
  --meanproptime=<meanproptime>   mean propagation time of messages (latency) [default: 1]
  --stdproptime=<stdproptime>     std deviation of the propagation time of messages [default: 0.1]
  --seed=<seed>                   random seed [default: 42]
  -g                              generate graph animation
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
            "STD_PROP_TIME":       float(args["--stdproptime"]),
            "SEED":                  int(args["--seed"]),
            "GRAPH":                 bool(args["-g"]),
           }

    driver.drive(opts)
