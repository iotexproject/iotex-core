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

import block
import driver
import player
import solver
import transaction

from docopt import docopt
import random

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

    driver.drive(opts)
