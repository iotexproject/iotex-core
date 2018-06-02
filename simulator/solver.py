# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines the Solver class, which takes in a list of players and simulates a proof of stake system for a specified number of rounds.
"""

import math
import random
import statistics

import plot
import player
import consensus_client

class Solver:
    def __init__(self, opts):
        """Initiates the solver class with the list of players and number of rounds"""

        self.players = [] # the list of nodes in the system
        for nPlayers, stake in opts["PLAYERS"]:
            self.players.extend([player.Player(stake) for i in range(nPlayers)])
            
        self.nHeartbeats = opts["N_ROUNDS"] # number of total heartbeats
        self.heartbeat   = 0                # the heartbeat, or clock, of the system

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

        # initializes consensus engines and gets the initial block proposals
        response = consensus_client.Consensus.initConsensus(self.N_PLAYERS)
        timestamp = 0 # at the start, timestamp is 0

        # adds initial block proposals to players' outbound queue
        for id, v in response:
            assert self.players[id].id == id, "player id does not match array position" # make sure player at index id has id id

            self.players[id].outbound.append([v, timestamp])

            print("added %s to player %d" % (v, id))

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
            message, flag = i.action(heartbeat)
            if flag:
                messages.append(message)
            else:
                messages.append([])
            connections.append(message)

        plot.makeGraph(heartbeat, len(self.players), connections, messages)

    def simulate(self):
        """Simulate the system"""
        
        for i in range(self.nHeartbeats):
            self.nextRound(i)

    def calcPercentStake(self):
        """Calculates the percent stake for each player"""
        
        totalStake = self.calcTotalStake()
        percent = [i.stake/totalStake for i in self.players]

        return percent

    def calcTotalStake(self):
        """Calculates the total stake among all players"""

        return sum([i.stake for i in self.players])
