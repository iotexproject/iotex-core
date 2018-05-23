# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines the Player class, which represents a player in the network and contains functionality to make transactions, propose blocks, and validate blocks.
"""

import random

import grpc
import numpy as np

from proto import simulator_pb2_grpc
from proto import simulator_pb2
import block
import solver
import transaction
import states
import message
import consensus_client

VERBOSE = True

class Player:
    id = 0 # player id
    
    MEAN_TX_FEE = 0.2  # mean transaction fee
    STD_TX_FEE  = 0.05 # std of transaction fee

    def __init__(self, stake):
        """Creates a new Player object"""

        self.id = Player.id # the player's id
        Player.id += 1                 

        self.stake = stake  # the number of tokens the player has staked in the system

        self.blockchain  = []   # blockchain (technically a blocklist)
        self.connections = []   # list of connected players
        self.inbound     = []   # inbound messages from other players in the network at heartbeat r
        self.outbound    = []   # outbound messages to other players in the network at heartbeat r

        self.consensus = consensus_client.Consensus()
        self.consensus.playerID = self.id

    def action(self, heartbeat):
        """Executes the player's actions for heartbeat r"""

        for msg, timestamp in self.inbound:
            if timestamp > heartbeat:
                if VERBOSE: print("received %s but timestamp > heartbeat" % msg)
                continue
            if VERBOSE: print("received %s" % msg)

            received = self.consensus.processMessage(msg)
            for mt, v in received:
                if mt == 0: # view state change message
                    outbound.append([v, timestamp])
                else: # block to be committed
                    blockchain.append(v)

            self.outbound += received
            
        self.inbound = list(filter(lambda x: x[1] > heartbeat, self.inbound)) # get rid of processed messages
        
        # self.blockchain = self.consensus.getBlockchain() # update blockchain from consensus scheme results

        self.sendOutbound() # send messages to connected players

    def sendOutbound(self):
        """Send all outbound connections to connected nodes"""
        
        for i in self.connections:
            for message, timestamp in self.outbound:
                dt = np.random.exponential(self.MEAN_PROP_TIME) # add propagation time to timestamp
                print("sent %s to %s" % (message, i))
                i.inbound.append([message, timestamp+dt])

        self.outbound.clear()

    def __str__(self):
        return "player %s" % (self.id)

    def __repr__(self):
        return "player %s" % (self.id)
        

            
