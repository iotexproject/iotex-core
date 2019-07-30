// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"encoding/json"
)

// TransferLogs represents a group of evm transfer traces
type TransferLogs struct {
	Transfers []TransferLog `json:"transfers"`
}

// TransferLogs represents one evm transfer trace
type TransferLog struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
}

// NewTransferLogs news a TransferLogs state
func NewTransferLogs(from, to, amount string) TransferLogs {
	return TransferLogs{Transfers: []TransferLog{TransferLog{
		From:   from,
		To:     to,
		Amount: amount,
	}}}
}

// AddLog adds new transfer log into TransferLogs state
func (t *TransferLogs) AddLog(from, to, amount string) {
	t.Transfers = append(t.Transfers, TransferLog{
		From:   from,
		To:     to,
		Amount: amount,
	})
}

// Serialize serializes TransferLogs state into bytes
func (t TransferLogs) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

// Deserialize deserializes bytes into TransferLogs state
func (t *TransferLogs) Deserialize(buf []byte) error {
	if err := json.Unmarshal(buf, t); err != nil {
		return err
	}
	return nil
}
