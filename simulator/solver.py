# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines the Solver class, which takes in a list of players and simulates a proof of stake system for a specified number of rounds.
"""

import math
import random
import statistics
import time

import plot
import player
import consensus_client

class Solver:
    timeTakenForPlot = 0
    
    def __init__(self, opts):
        """Initiates the solver class with the list of players and number of rounds"""

        # combine opts["PLAYERS"] elements in the case of [(5, 1), ..., (9, 1)]; should be [(14, 1), ...]
        tmp = {}
        for nPlayers, ct in opts["PLAYERS"]:
            if ct in tmp:
                tmp[ct] += nPlayers
            else:
                tmp[ct] = nPlayers
        opts["PLAYERS"] = [(i[1], i[0]) for i in list(tmp.items())]

        # sort by ct because consensus engine creates consensus objects in order: honest, failure stop, byzantine
        opts["PLAYERS"].sort(key=lambda x: x[1])

        print(opts["PLAYERS"])
        
        self.players = [] # the list of nodes in the system
        for nPlayers, ct in opts["PLAYERS"]:
            for i in range(nPlayers):
                print("Created player of type %d"%ct)
                self.players.append(player.Player(ct))
            
        self.nHeartbeats = opts["N_ROUNDS"] # number of total heartbeats
        self.heartbeat   = 0                # the heartbeat, or clock, of the system
        self.dHeartbeat  = opts["D_HEARTBEAT"]

        self.blockchain = None # common blockchain among all players

        # add pointer to solver to players
        for i in self.players:
            i.solver = self

        self.N_PLAYERS = len(self.players)
            
        self.connectNetwork()

        print("==NETWORK CONNECTIONS==")
        for i in self.players:
            print("%s: %s" % (i, i.connections))
        print()

        nHonest = 0
        nFS = 0
        nBF = 0
        for nPlayers, ct in opts["PLAYERS"]:
            if ct == player.CTypes.Honest:
                nHonest = nPlayers
            elif ct == player.CTypes.FailureStop:
                nFS = nPlayers
            elif ct == player.CTypes.ByzantineFault:
                nBF = nPlayers

        # initializes consensus engines and gets the initial block proposals
        response = consensus_client.Consensus.initConsensus(nHonest, nFS, nBF)
        
        # delay for 2 seconds to allow nodes to ramp up (matches delay in consensus code)
        time.sleep(2)

        self.genGraph = opts["GRAPH"]

    def connectNetwork(self):
        """Form the network of players through random assignment of connections"""

        for i in range(len(self.players)):
            others = self.players[:i]+self.players[i+1:]
            self.players[i].connections = random.sample(others, self.N_CONNECTIONS)

    def nextRound(self, heartbeat):
        """Simulates the next round"""
        
        self.heartbeat = heartbeat

        messages = []
        connections = []
        for i in self.players:
            message, sentMsgs = i.action(heartbeat)
            if sentMsgs:
                messages.append(message)
            else:
                messages.append([])
            connections.append(message)

        start_time = time.time()
        if self.genGraph: plot.makeGraph(round(heartbeat/self.dHeartbeat), len(self.players), connections, messages, "time = %f"%heartbeat)
        Solver.timeTakenForPlot += time.time()-start_time

    def simulate(self):
        """Simulate the system"""
        
        for i in range(self.nHeartbeats+1):
            self.nextRound(i * self.dHeartbeat)

