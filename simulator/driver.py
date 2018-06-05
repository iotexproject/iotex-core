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

import numpy as np

import player
import solver
import consensus_client
import plot

def drive(opts):
    """Drive execution of the program. opts: dictionary of options.
       Ex: opts = {"PLAYERS": [(100, 1)], # list of tuples; [(number of players, stake per player)]
            "N_CONNECTIONS": 8,           # number of connections per player
            "TIME_TO_SIM": 5,             # virtual time to simulate system for
            "MEAN_PROP_TIME": 0.1         # mean propagation time of messages
            "STD_PROP_TIME": 0.001        # standard deviation of the propagation time of messages
           }"""

    ALPHA = 0.5 # scaling constant -- simulation updates 1//(alpha*latency) times per heartbeat
    opts["D_HEARTBEAT"] = opts["MEAN_PROP_TIME"] * ALPHA
    opts["N_ROUNDS"]    = math.ceil(opts["TIME_TO_SIM"] / opts["D_HEARTBEAT"])
    
    solver.Solver.N_CONNECTIONS         = opts["N_CONNECTIONS"]
    solver.Solver.N_ROUNDS              = opts["N_ROUNDS"]

    # convert lognormal mean, std to normal mean, std
    normal_std = np.sqrt(np.log(1 + (opts["STD_PROP_TIME"]/opts["MEAN_PROP_TIME"])**2))
    normal_mean = np.log(opts["MEAN_PROP_TIME"]) - normal_std**2 / 2

    player.Player.NORMAL_STD = normal_std
    player.Player.NORMAL_MEAN  = normal_mean

    random.seed(opts["SEED"])

    print("====simulating for %s rounds====\n"%(opts["N_ROUNDS"]))

    sol = solver.Solver(opts)
    
    sol.simulate()

    print("==PRINTING LOCAL BLOCKCHAINS==")
    blockchains = []
    nMsgsPassed = []
    timeCreated = []
    for i in sol.players:
        nMsgsPassed.append(i.nMsgsPassed)
        blockchains.append(i.blockchain)
        timeCreated.append(i.timeCreated)
        print("%s: %s"%(i, str(i.blockchain).replace("\n", "\n\t")))

    correctHashes = player.Player.correctHashes
    print("correct block hashes: %s"%(str(correctHashes).replace("\n", "\n\t")))

    try: consensus_client.Consensus.close()
    except: pass

    # calc stats
    print("\n==CALCULATING STATISTICS==")
    nRounds = min(map(len, blockchains)) # number of rounds completed by all
    fullConsensus = 0 # number of rounds where full consensus was achieved among all players
    for i in range(nRounds):
        fullConsensus += all(blockchains[0][i] == j[i] for j in blockchains)

    correctHashes = player.Player.correctHashes
    nMsgsPassed = [sum(nMsgsPassed[j][i] for j in range(len(nMsgsPassed))) for i in range(nRounds)]
    timeCreated = [max(timeCreated[j][i] for j in range(len(timeCreated))) for i in range(nRounds)]
    dts = [timeCreated[0]]+[timeCreated[i+1]-timeCreated[i] for i in range(len(timeCreated)-1)]
    nConsensus = [0]*max(map(len, blockchains))
    for i in blockchains:
        for j in range(len(i)):
            if i[j] == correctHashes[j]: nConsensus[j] += 1
    fullConsensus2 = nConsensus.count(len(sol.players))
    assert fullConsensus2==fullConsensus

    print()
    print("BLOCKS CREATED PER NODE = %s"%(", ".join(list(map(str, list(map(len, blockchains)))))))
    print("N DELEGATES WHICH ACHIEVED CONSENSUS IN EACH ROUND = %s"%(", ".join(list(map(str, nConsensus)))))
    print("N ROUNDS WITH FULL CONSENSUS ACHIEVED/N ROUNDS FULLY COMPLETED = %d/%d = %f"%(fullConsensus, nRounds, fullConsensus/nRounds))
    print("MSGS PASSED PER BLOCK = %s"%(", ".join(list(map(str, nMsgsPassed)))))
    print("AVERAGE MSGS PASSED PER BLOCK = %f"%(sum(nMsgsPassed)/len(nMsgsPassed)))
    print("TIME BLOCKS COMMITTED BY ALL NODES = %s"%(", ".join(list(map(str, timeCreated)))))
    print("TIME TO CREATE BLOCKS = %s"%(", ".join(list(map(str, dts)))))
    print("AVERAGE TIME TO CREATE BLOCK = %f"%(sum(dts)/len(dts)))
    print("BLOCKS CREATED PER SECOND = %f"%(nRounds/opts["TIME_TO_SIM"]))

    # get rid of useless .db files
    os.system("rm chain*.db")

    plot.makeAnim(["out%d.png"%i for i in range(opts["N_ROUNDS"])])
    os.system("rm -f out*.dot out*.png")
