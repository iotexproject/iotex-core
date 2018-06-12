// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

import (
	"testing"
)

func TestBuildOpNodeFromBytes(t *testing.T) {
	t.Parallel()

	var testBytecodes []byte
	var node *OpNode
	var offset int
	var err error

	// Test 1
	t.Logf("HandleTransition first BuildOpNodeFromBytes test")
	testBytecodes = []byte{Op0, OpData1}
	node, offset, err = BuildOpNodeFromBytes(testBytecodes)
	if err != nil {
		t.Errorf("Meet an error! %s", err.Error())
	}
	if offset != 1 {
		t.Errorf("Offset is wrong. Expect 1, get %d", offset)
	}
	if node.opcode != Op0 {
		t.Errorf("Opcode is wrong. Expect %x, get %x", Op0, node.opcode)
	}

	// Test 2: Opnode not found error
	t.Logf("HandleTransition 2nd BuildOpNodeFromBytes test")
	testBytecodes = []byte{OpUnused, Op0}
	_, _, err = BuildOpNodeFromBytes(testBytecodes)
	if err == nil {
		t.Errorf("Should get an error!")
	}

	// Test 3: PushData opcode test
	t.Logf("HandleTransition 3rd BuildOpNodeFromBytes test")
	testBytecodes = []byte{OpData2, 0x14, 0xa0, 0xf3}
	node, offset, err = BuildOpNodeFromBytes(testBytecodes)
	if err != nil {
		t.Errorf("Meet an error! %s", err.Error())
	}
	if offset != 3 {
		t.Errorf("Offset is wrong. Expect 3, get %d", offset)
	}
	if len(node.data) != 2 || node.data[0] != 0x14 || node.data[1] != 0xa0 {
		t.Errorf("OpNode data is wrong")
	}

	// Test 4: Branch opcode test
	t.Logf("HandleTransition 4th BuildOpNodeFromBytes test")
	testBytecodes = []byte{OpIf, OpData2, 0xa0, 0xf3, Op0, OpElse, Op0, OpIf, OpData1, 0x00, Op0, OpEndIf, OpEndIf}
	node, offset, err = BuildOpNodeFromBytes(testBytecodes)
	if err != nil {
		t.Errorf("Meet an error ! %s", err.Error())
	}
	if offset != len(testBytecodes) {
		t.Errorf("Offset is wrong. Expect %d, get %d", len(testBytecodes), offset)
	}
	if len(node.asts) != 2 || len(node.asts[0].nodes) != 2 || len(node.asts[1].nodes[1].asts) != 1 {
		t.Errorf("Get a wrong branch node")
	}

	testBytecodes = []byte{OpIf, OpData2, 0xa0, 0xf3, Op0, OpElse, Op0, OpIf, OpData1, 0x00, Op0, OpEndIf}
	_, _, err = BuildOpNodeFromBytes(testBytecodes)
	if err == nil || err.Error() != "Unbalanced conditional branch. No OpEndIf found" {
		t.Errorf("Expect an unbalanced conditional branch error")
	}

}
