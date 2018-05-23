# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines the Solver class, which takes in a list of players and simulates a proof of stake system for a specified number of rounds.
"""

import block
import player
import transaction
import pbftconsensus
import consensus_client

import math
import random
import statistics

class Solver:
    def __init__(self, opts):
        """Initiates the solver class with the list of players and number of rounds"""

        self.players = [] # the list of nodes in the system
        for nPlayers, stake in opts["PLAYERS"]:
            self.players.extend([player.Player(stake) for i in range(nPlayers)])
            
        self.nHeartbeats = opts["N_ROUNDS"]*Solver.N_HEARTBEATS_IN_ROUND # number of total heartbeats
        self.heartbeat   = 0                                             # the heartbeat, or clock, of the system

        self.blockchain = None # common blockchain among all players

        # add pointer to solver to players
        for i in self.players:
            i.solver = self

        self.N_PLAYERS = len(self.players)
            
        self.connectNetwork()

        print("==NETWORK CONNECTIONS==")
        for i in self.players:
            print("%s: %s" % (i, i.connections))

        consensus_client.Consensus.initConsensus(self.N_PLAYERS)

    def connectNetwork(self):
        """Form the network of players through random assignment of connections"""

        for i in range(len(self.players)):
            others = self.players[:i]+self.players[i+1:]
            self.players[i].connections = random.sample(others, self.N_CONNECTIONS)

    def chooseProposers(self):
        """Choose proposer for next round; chance of being chosen proportional to stake"""

        return random.choice([i.id for i in self.players])
    
        """totalStake = self.calcTotalStake()
        coins = random.sample(list(range(1, int(totalStake+1))), self.N_PROPOSERS) 

        proposers = []
        
        startCoins = 0 # coin number right before the start of the current player's set of coins
        endCoins = 0 # the player's last coin number
        for i in self.players:
            endCoins += i.stake
            for j in coins:
                if j <= endCoins and j > startCoins:
                    proposers.append(i)
            startCoins = endCoins

        proposers = [i.id for i in proposers]

        # prevent player from being chosen twice
        if len(proposers) != len(set(proposers)):
            proposers = self.chooseProposers()
        
        return proposers"""

    """def payout(self, vset, proposer):
        # validator/proposer rewards
        for i in vset:
            i.stake += 1

        proposer.stake += 1

        # tx fee distribution
        totFee = sum([i.fee for i in self.blockchain.txs])
        n      = len(vset) + 1
        for i in vset:
            i.stake += totFee/n

        proposer.stake += totFee/n"""

    def nextRound(self, heartbeat):
        """Simulates the next round"""

        self.heartbeat = heartbeat

        """# if start of round, reset validator, proposer set & update common blockchain
        if heartbeat % Solver.N_HEARTBEATS_IN_ROUND == 0:
            self.propSet = self.chooseProposers()  # choose proposer set

            # update common blockchain among players
            currBlock = self.players[0].blockchain
            same = True
            for i in self.players:
                if currBlock != i.blockchain:
                    same = False
                    break

            if same and currBlock != None:
                currBlock = block.Block(currBlock.txs, id=currBlock.id, proposer=currBlock.proposer)
                currBlock.next = self.blockchain
                self.blockchain = currBlock"""

        for i in self.players:
            i.action(heartbeat)

    def simulate(self):
        """Simulate the system"""
        
        for i in range(self.nHeartbeats):
            self.nextRound(i)

        """# update common blockchain among players
        currBlock = self.players[0].blockchain
        same = True
        for i in self.players:
            if currBlock != i.blockchain:
                same = False
                break

        if same and currBlock != None:
            currBlock = block.Block(currBlock.txs, id=currBlock.id, proposer=currBlock.proposer)
            currBlock.next = self.blockchain
            self.blockchain = currBlock"""

    def calcPercentStake(self):
        """Calculates the percent stake for each player"""
        
        totalStake = self.calcTotalStake()
        percent = [i.stake/totalStake for i in self.players]

        return percent

    def calcTotalStake(self):
        """Calculates the total stake among all players"""

        return sum([i.stake for i in self.players])
