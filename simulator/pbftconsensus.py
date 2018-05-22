# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""Consensus class which accepts inbound messages from other players, processes the messages, and sends outbound messages"""

import block
import solver
import transaction
import states
import message

import random
import numpy as np

VERBOSE = True

class PBFTConsensus:
    id = 0
    
    def __init__(self):
        self.id = PBFTConsensus.id
        PBFTConsensus.id += 1

        self.blockchain      = None  # the current state of the player's blockchain
        self.mempool         = set() # the current list of txs the player knows about
        self.seenTxs         = set() # set of seen txs
        self.seenBlocks      = {}    # map of blockId to block
        self.committedBlocks = set()

        self.preVotes    = {}
        self.votes       = {}

        self.outbound = []
        self.inbound  = []

        self.stage = states.States.Consensus.PRE_PRE_VOTE

        self.proposer  = False

    def roundInit(self):
        """Initializes instance variables and the such at the start of a new round"""

        outbound = []

        if VERBOSE: print("\n%s"%self.player)

        heartbeat = self.player.solver.heartbeat
        
        # if start of round, reset node role
        if heartbeat % solver.Solver.N_HEARTBEATS_IN_ROUND == 0:
            if VERBOSE: print("STARTED NEW ROUND")
            self.proposer  = False
            if self.player.id == self.player.solver.propSet:
                self.proposer = True

            self.stage = states.States.Consensus.PRE_PRE_VOTE
            
            self.preVotes.clear()
            self.votes.clear()
            
        # if proposer and at the start of round, propose block and send prevote
        if heartbeat % solver.Solver.N_HEARTBEATS_IN_ROUND == 0 and self.proposer:
            pBlock = self.proposeBlock()
            outbound.append([message.Message(message.Message.MessageType.BLOCK, pBlock, self.player.id), heartbeat])
            outbound.append([message.Message(message.Message.MessageType.PRE_VOTE, hash(pBlock), self.player.id), heartbeat])

            self.seenBlocks[hash(pBlock)] = pBlock
            self.preVotes[hash(pBlock)] = set([self.player.id])
            self.stage = states.States.Consensus.PRE_VOTE
            
            if VERBOSE: print("proposed block")
                
        # make transaction with probability p
        if random.random() < self.player.P_TRANSACTIONS:
            tx = self.makeTransaction()
            outbound.append([message.Message(message.Message.MessageType.TRANSACTION, tx, self.player.id), heartbeat])
            self.mempool.add(tx)

        if VERBOSE: print("preVotes:", self.preVotes)
        if VERBOSE: print("votes:", self.votes)

        return outbound

    def processMessage(self, msg):
        """Process a message at specified heartbeat"""

        outbound = []
        
        # if inbound message is transaction, add to local mempool
        if msg.type == message.Message.MessageType.TRANSACTION:
            if msg.value in self.seenTxs:
                return outbound
             
            self.mempool.add(msg.value)
            self.seenTxs.add(msg.value)

            outbound.append(msg)

        # handle pre pre vote
        if self.stage == states.States.Consensus.PRE_PRE_VOTE and msg.type == message.Message.MessageType.BLOCK:
            # message value is block
            if hash(msg.value) in self.seenBlocks:
                return outbound
                
            self.seenBlocks[hash(msg.value)] = msg.value
                    
            if self.isValid(msg.value):
                pv = hash(msg.value)
                if VERBOSE: print("pre pre vote valid; moved to pre vote")
            else:
                pv = None
                if VERBOSE: print("pre pre vote invalid; moved to pre vote")

            outbound.append(msg)
            outbound.append(message.Message(message.Message.MessageType.PRE_VOTE, pv, self.player.id))
            self.preVotes[pv] = set([self.player.id])
            self.stage = states.States.Consensus.PRE_VOTE
            # todo: add timeout

        # handle pre vote
        if self.stage == states.States.Consensus.PRE_VOTE and msg.type == message.Message.MessageType.PRE_VOTE:
            # message value is block hash
            if msg.value not in self.preVotes:
                self.preVotes[msg.value] = set()

            if msg.senderId not in self.preVotes[msg.value]:
                self.preVotes[msg.value].add(msg.senderId)
                outbound.append(msg)

            if len(self.preVotes[msg.value]) >= 2*self.player.solver.N_PLAYERS/3:
                self.stage = states.States.Consensus.VOTE
                self.votes[msg.value] = set([self.player.id])
                outbound.append(message.Message(message.Message.MessageType.VOTE, msg.value, self.player.id))
                if VERBOSE: print("moved to vote stage")

        # handle vote
        if self.stage == states.States.Consensus.VOTE and msg.type == message.Message.MessageType.VOTE:
            if msg.value not in self.votes:
                self.votes[msg.value] = set()

            if msg.senderId not in self.votes[msg.value]:
                self.votes[msg.value].add(msg.senderId)
                outbound.append(msg)

            if len(self.votes[msg.value]) >= 2*self.player.solver.N_PLAYERS/3 and msg.value not in self.committedBlocks:
                self.committedBlocks.add(msg.value)

                nBlock = self.seenBlocks[msg.value].copy()
                if VERBOSE: print("committed block %s" % (nBlock))
                nBlock.next = self.blockchain
                self.blockchain = nBlock

                # remove txs from local mempool
                for tx in nBlock.txs:
                    if tx in self.mempool:
                        self.mempool.remove(tx)
                        
        return outbound

    def isValid(self, block):
        """Returns whether a block is valid or not"""
        
        return True
    
    def makeTransaction(self):
        """Returns a random transaction"""

        fee = max(random.gauss(self.player.MEAN_TX_FEE, self.player.STD_TX_FEE), 0)
        return transaction.Transaction(self.player.id, 0, fee)

    def proposeBlock(self):
        """Proposes a Block consisting of multiple random transactions"""
        
        txs = random.sample(self.mempool, min(self.player.N_TRANSACTIONS, len(self.mempool)))

        return block.Block(txs, proposer=self.player)

    def getBlockchain(self):
        return self.blockchain
        
