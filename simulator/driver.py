# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This file runs the solver for any arbitrary user-defined test case. Meant to be programmed on top of."""

import math
import os
import pstats
import random
import statistics
import time

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

    start_time = time.time()

    ALPHA = 0.8 # scaling constant -- simulation updates 1//(alpha*latency) times per heartbeat
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
        if i.consensusType == player.CTypes.Honest:
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
    correctHashes = player.Player.correctHashes
    nMsgsPassed = [sum(nMsgsPassed[j][i] for j in range(len(nMsgsPassed))) for i in range(nRounds)]
    timeCreated = [max(timeCreated[j][i] for j in range(len(timeCreated))) for i in range(nRounds)]
    if len(timeCreated) > 0:
        dts = [timeCreated[0]]+[timeCreated[i+1]-timeCreated[i] for i in range(len(timeCreated)-1)]
    else:
        dts = []
    nConsensus = [0]*max(map(len, blockchains))
    for i in blockchains:
        for j in range(len(i)):
            if i[j] == correctHashes[j]: nConsensus[j] += 1
    fullConsensus = nConsensus.count(len(blockchains))

    print()
    print("==Note: consensus statistics only apply to honest nodes==")
    print("BLOCKS CREATED PER NODE = %s"%(", ".join(list(map(str, list(map(len, blockchains)))))))
    print("N HONEST DELEGATES WHICH ACHIEVED CONSENSUS IN EACH ROUND = %s"%(", ".join(list(map(str, nConsensus)))))
    if nRounds != 0:
        print("N ROUNDS WITH FULL CONSENSUS ACHIEVED/N ROUNDS FULLY COMPLETED = %d/%d = %f"%(fullConsensus, nRounds, fullConsensus/nRounds))
    else:
        print("N ROUNDS WITH FULL CONSENSUS ACHIEVED/N ROUNDS FULLY COMPLETED = %d/%d = 0"%(fullConsensus, nRounds))
    print("MSGS PASSED PER BLOCK = %s"%(", ".join(list(map(str, nMsgsPassed)))))
    if len(nMsgsPassed) != 0:
        print("AVERAGE MSGS PASSED PER BLOCK = %f"%(sum(nMsgsPassed)/len(nMsgsPassed)))
    else:
        print("AVERAGE MSGS PASSED PER BLOCK = 0")
    print("TIME BLOCKS COMMITTED BY ALL HONEST NODES = %s"%(", ".join(list(map(str, timeCreated)))))
    print("TIME TO CREATE BLOCKS = %s"%(", ".join(list(map(str, dts)))))
    if len(dts) != 0:
        avg = sum(dts)/len(dts)
        print("AVERAGE TIME TO CREATE BLOCK/LATENCY = %f/%f = %f"%(avg, opts["MEAN_PROP_TIME"], avg/opts["MEAN_PROP_TIME"]))
    else:
        print("AVERAGE TIME TO CREATE BLOCK/LATENCY = inf/%f = inf"%opts["MEAN_PROP_TIME"])
    if opts["TIME_TO_SIM"] != 0:
        print("BLOCKS CREATED PER SECOND = %f"%(nRounds/opts["TIME_TO_SIM"]))
    else:
        print("BLOCKS CREATED PER SECOND = 0")

    # get rid of useless .db files
    os.system("rm chain*.db")

    print("time taken for sim: ", time.time()-start_time)
    print("time taken to output graphs: ", solver.Solver.timeTakenForPlot)

    if opts["GRAPH"]: plot.makeAnim(["out%d.png"%i for i in range(opts["N_ROUNDS"])])
    
    os.system("rm -f out*.dot out*.png")
