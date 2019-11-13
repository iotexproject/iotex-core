package api

import (
	"testing"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

var MOCK_ChainListener Listener

// Mock ChainListener
func TestNewChainListener(t *testing.T) {
	test := struct {
		name string
	}{
		name: "New Chain listener.",
	}

	t.Run(test.name, func(t *testing.T) {
		MOCK_ChainListener = NewChainListener()
	})

}

func Test_chainListener_Start(t *testing.T) {
	test := struct {
		name    string
		wantErr bool
	}{
		name:    "Test start listener.",
		wantErr: false,
	}

	t.Run(test.name, func(t *testing.T) {
		if err := MOCK_ChainListener.Start(); (err != nil) != test.wantErr {
			t.Errorf("chainListener.Start() error = %v, wantErr %v", err, test.wantErr)
		}
	})

}

func Test_chainListener_Stop(t *testing.T) {
	test := struct {
		name    string
		wantErr bool
	}{

		name:    "Stop lisntener",
		wantErr: false,
	}

	t.Run(test.name, func(t *testing.T) {
		if err := MOCK_ChainListener.Stop(); (err != nil) != test.wantErr {
			t.Errorf("chainListener.Stop() error = %v, wantErr %v", err, test.wantErr)
		}
	})

}

func Test_chainListener_HandleBlock(t *testing.T) {
	type args struct {
		blk *block.Block
	}
	test := struct {
		name    string
		args    args
		wantErr bool
	}{
		name: "Test HandleBlock.",
		args: args{
			blk: new(block.Block),
		},
		wantErr: false,
	}

	t.Run(test.name, func(t *testing.T) {
		if err := MOCK_ChainListener.HandleBlock(test.args.blk); (err != nil) != test.wantErr {
			t.Errorf("chainListener.HandleBlock() error = %v, wantErr %v", err, test.wantErr)
		}
	})

}

type ChainResponder struct{}

func (cr ChainResponder) Respond(*block.Block) error {
	return nil
}
func (cr ChainResponder) Exit() {
}

var MOCK_ChainResponder ChainResponder

func Test_chainListener_AddResponder(t *testing.T) {
	type args struct {
		r Responder
	}
	test := struct {
		name    string
		args    args
		wantErr bool
	}{
		name: "Test AddResponder",
		args: args{
			r: MOCK_ChainResponder,
		},
		wantErr: false,
	}
	t.Run(test.name, func(t *testing.T) {
		if err := MOCK_ChainListener.AddResponder(test.args.r); (err != nil) != test.wantErr {
			t.Errorf("chainListener.AddResponder() error = %v, wantErr %v", err, test.wantErr)
		}
	})
}
