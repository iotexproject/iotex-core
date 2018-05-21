"""This module defines the Player class, which represents a player in the network and contains functionality to make transactions, propose blocks, and validate blocks.
"""

import block
import solver
import transaction
import states
import message
import pbftconsensus

import random
import numpy as np

VERBOSE = pbftconsensus.VERBOSE

class Player:
    id = 0 # player id
    
    MEAN_TX_FEE = 0.2  # mean transaction fee
    STD_TX_FEE  = 0.05 # std of transaction fee

    def __init__(self, stake):
        """Creates a new Player object"""

        self.id = Player.id # the player's id
        Player.id += 1                 

        self.stake = stake  # the number of tokens the player has staked in the system

        self.blockchain  = None # blockchain
        self.connections = []   # list of connected players
        self.inbound     = []   # inbound messages from other players in the network at heartbeat r
        self.outbound    = []   # outbound messages to other players in the network at heartbeat r

        self.consensus = pbftconsensus.PBFTConsensus()
        self.consensus.player = self

    def action(self, heartbeat):
        """Executes the player's actions for heartbeat r"""

        self.outbound += self.consensus.roundInit() # remove in real version
        
        for msg, timestamp in self.inbound:
            if timestamp > heartbeat:
                if VERBOSE: print("received %s but timestamp > heartbeat" % msg)
                continue
            if VERBOSE: print("received %s" % msg)

            received = self.consensus.processMessage(msg)
            received = zip(received, [timestamp]*len(received))
            
            self.outbound += received
            
        self.inbound = list(filter(lambda x: x[1] > heartbeat, self.inbound)) # get rid of processed messages
        
        self.blockchain = self.consensus.getBlockchain() # update blockchain from consensus scheme results

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
        

            
