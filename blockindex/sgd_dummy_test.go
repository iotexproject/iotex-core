// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import "testing"

func TestSGDDummy(t *testing.T) {
	r := NewDummySGDRegistry()
	if err := r.Start(nil); err != nil {
		t.Error(err)
	}
	if err := r.Stop(nil); err != nil {
		t.Error(err)
	}
	if _, err := r.Height(); err != nil {
		t.Error(err)
	}
	if err := r.PutBlock(nil, nil); err != nil {
		t.Error(err)
	}
	if err := r.DeleteTipBlock(nil, nil); err != nil {
		t.Error(err)
	}
	if _, _, ok := r.CheckContract(""); ok {
		t.Error("should return false")
	}
}
