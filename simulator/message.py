# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines the Message class, which represents a message sent between nodes.
"""

class Message:
    id = 0

    class MessageType:
        TRANSACTION  = 0
        PRE_VOTE     = 1
        VOTE         = 2
        BLOCK        = 3
        
    def __init__(self, type, value, senderId):
        self.id = Message.id
        Message.id += 1

        self.type = type
        self.value = value
        
        self.senderId = senderId

    def __str__(self):
        type = {0: "tx", 1: "prevote", 2: "vote", 3: "block"}
        return "message %s; type %s, val %s, sender %s" % (self.id, type[self.type], self.value, self.senderId)

    
        
