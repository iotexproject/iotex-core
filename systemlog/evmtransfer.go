// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package systemlog

import (
	"math/big"
)

type blockEvmTransfer struct {
	blockHeight           uint64
	numEvmTransfer        int
	actionEvmTransferList []*actionEvmTransfer
}

type actionEvmTransfer struct {
	actionHash      []byte
	numEvmTransfer  int
	evmTransferList []*evmTransfer
}

type evmTransfer struct {
	amount *big.Int
	from   string
	to     string
}

//// ConvertToBlockEvmTransferPb converts a blockEvmTransfer to protobuf's blockEvmTransfer
//func (bet *blockEvmTransfer) ConvertToBlockEvmTransferPb() *systemlogpb.BlockEvmTransfer {
//	pb := &systemlogpb.BlockEvmTransfer{
//		BlockHeight:    bet.blockHeight,
//		NumEvmTransfer: int32(bet.numEvmTransfer),
//	}
//	for _, aet := range bet.actionHashList {
//		aet := actionEvmTransfer{actionHash: aet}
//		pb.ActionEvmTransferList = append(pb.ActionEvmTransferList, aet.ConvertToActionEvmTransferPb())
//	}
//	return pb
//}
//
//// ConvertFromBlockEvmTransferPb converts a protobuf's blockEvmTransfer to blockEvmTransfer
//func (bet *blockEvmTransfer) ConvertFromBlockEvmTransferPb(pb *systemlogpb.BlockEvmTransfer) {
//	bet.blockHeight = pb.BlockHeight
//	bet.numEvmTransfer = int(pb.NumEvmTransfer)
//	for _,apb:=pb.ActionEvmTransferList{
//		var aet *actionEvmTransfer
//		aet.ConvertFromActionEvmTransferPb(apb)
//		bet.actionHashList=append(bet.actionHashList,aet)
//	}
//}
//
//func (aet *actionEvmTransfer) ConvertToActionEvmTransferPb() *systemlogpb.ActionEvmTransfer {
//	pb := &systemlogpb.ActionEvmTransfer{
//		ActionHash:     aet.actionHash,
//		NumEvmTransfer: int32(aet.numEvmTransfer),
//	}
//	for _, aet := range aet.evmTransferList {
//		pb.EvmTransferList = append(pb.EvmTransferList, aet.ConvertToEvmTransferPb())
//	}
//	return pb
//}
//
//func (aet *actionEvmTransfer) ConvertFromActionEvmTransferPb(pb *systemlogpb.ActionEvmTransfer) {
//	aet.actionHash = pb.ActionHash
//	aet.numEvmTransfer = int(pb.NumEvmTransfer)
//	for _, epb := range pb.EvmTransferList {
//		var et *evmTransfer
//		et.ConvertFromEvmTransferPb(epb)
//		aet.evmTransferList = append(aet.evmTransferList, et)
//	}
//}
//
//func (et *evmTransfer) ConvertToEvmTransferPb() *systemlogpb.EvmTransfer {
//	return &systemlogpb.EvmTransfer{
//		Amount: et.amount.Bytes(),
//		From:   et.from,
//		To:     et.to,
//	}
//}
//
//func (et *evmTransfer) ConvertFromEvmTransferPb(pb *systemlogpb.EvmTransfer) {
//	et.amount = new(big.Int).SetBytes(pb.Amount)
//	et.from = pb.From
//	et.to = pb.To
//}
//
