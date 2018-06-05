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
import solver
import consensus_client

class Player:
    id = 0 # player id
    
    MEAN_TX_FEE    = 0.2                                 # mean transaction fee
    STD_TX_FEE     = 0.05                                # std of transaction fee
    DUMMY_MSG_TYPE = 1999                                # if there are no messages to process, dummy message is sent to consensus engine
    msgMap         = {(DUMMY_MSG_TYPE, ""): "dummy msg"} # maps message to message name for printing

    def __init__(self, stake):
        """Creates a new Player object"""

        self.id = Player.id # the player's id
        Player.id += 1                 

        self.stake = stake  # the number of tokens the player has staked in the system

        self.blockchain   = []    # blockchain (technically a blocklist)
        self.connections  = []    # list of connected players
        self.inbound      = []    # inbound messages from other players in the network at heartbeat r
        self.outbound     = []    # outbound messages to other players in the network at heartbeat r
        self.seenMessages = set() # set of seen messages

        self.consensus          = consensus_client.Consensus()
        self.consensus.playerID = self.id
        self.consensus.player   = self
        
        self.nMsgsPassed = [0]
        self.timeCreated = []

    def action(self, heartbeat):
        """Executes the player's actions for heartbeat r"""

        print("player %d action started at heartbeat %f" % (self.id, heartbeat))

        # self.inbound: [(msgType, msgBody), timestamp]
        # print messages
        for msg, timestamp in self.inbound:
            print("received %s with timestamp %f" % (Player.msgMap[msg], timestamp))

        if len(list(filter(lambda x: x[1] <= heartbeat, self.inbound))) == 0:
            self.inbound += [[(Player.DUMMY_MSG_TYPE, ""), heartbeat]]

        # process each message
        for msg, timestamp in self.inbound:
            # note: msg is a tuple: (msgType, msgBody)
            if timestamp > heartbeat: continue

            if msg[0] != Player.DUMMY_MSG_TYPE and msg in self.seenMessages: continue
            self.seenMessages.add(msg)

            print("sent %s to consensus engine" % Player.msgMap[msg])
            received = self.consensus.processMessage(msg)
            
            for mt, v in received:
                if v not in Player.msgMap:
                    Player.msgMap[v] = "msg "+str(len(Player.msgMap))
                print("received %s from consensus engine" % Player.msgMap[v])
                
                if mt == 0: # view state change message
                    self.outbound.append([v, timestamp])
                else: # block to be committed
                    self.blockchain.append(v[1])
                    self.nMsgsPassed.append(0)
                    self.timeCreated.append(timestamp)
                    print("committed %s to blockchain" % Player.msgMap[v])

            if msg[0] != Player.DUMMY_MSG_TYPE: self.outbound.append([msg, timestamp])
            
        self.inbound = list(filter(lambda x: x[1] > heartbeat, self.inbound)) # get rid of processed messages
        
        return self.sendOutbound()

    def sendOutbound(self):
        """Send all outbound connections to connected nodes.
           Returns list of nodes messages have been sent to"""

        if len(self.outbound) == 0:
            flag = False
        else:
            flag = True

        ci = []
        
        for i in self.connections:
            ci.append(i.id)
            for message, timestamp in self.outbound:
                self.nMsgsPassed[-1] += 1
                dt = np.random.lognormal(self.NORMAL_MEAN, self.NORMAL_STD) # add propagation time to timestamp
                print("sent %s to %s" % (Player.msgMap[message], i))
                i.inbound.append([message, timestamp+dt])

        self.outbound.clear()
        print()

        return ci, flag

    def __str__(self):
        return "player %s" % (self.id)

    def __repr__(self):
        return "player %s" % (self.id)
        

            
