# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This file runs the solver for any arbitrary user-defined test case. Meant to be programmed on top of."""

import math
import os
import random
import statistics

import player
import solver
import consensus_client

def drive(opts):
    """Drive execution of the program. opts: dictionary of options.
       Ex: opts = {"PLAYERS": [(100, 1)], # list of tuples; [(number of players, stake per player)]
            "N_CONNECTIONS": 8,           # number of connections per player
            "N_ROUNDS": 5,                # number of rounds of proposal/validation/commit
            "MEAN_PROP_TIME": 0.1         # mean propagation time of messages (exponential distribution)
           }"""

    solver.Solver.N_CONNECTIONS         = opts["N_CONNECTIONS"]
    solver.Solver.N_ROUNDS              = opts["N_ROUNDS"]

    player.Player.MEAN_PROP_TIME = opts["MEAN_PROP_TIME"]

    random.seed(opts["SEED"])

    print("====simulating for %s rounds====\n"%(opts["N_ROUNDS"]))

    sol = solver.Solver(opts)
    
    sol.simulate()

    for i in sol.players:
        print(i)
        print("\t"+str(i.blockchain).replace("\n", "\n\t"))
        print()

    try: consensus_client.Consensus.close()
    except: pass

    # get rid of useless .db files
    os.system("rm chain*.db")
