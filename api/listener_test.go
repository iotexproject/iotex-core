package api

import (
	"testing"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

var MOCKChainListener Listener

// Mock ChainListener
func TestNewChainListener(t *testing.T) {
	test := struct {
		name string
	}{
		name: "New Chain listener.",
	}

	t.Run(test.name, func(t *testing.T) {
		MOCKChainListener = NewChainListener()
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
		if err := MOCKChainListener.Start(); (err != nil) != test.wantErr {
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
		if err := MOCKChainListener.Stop(); (err != nil) != test.wantErr {
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
		if err := MOCKChainListener.HandleBlock(test.args.blk); (err != nil) != test.wantErr {
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

var MOCKChainResponder ChainResponder

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
			r: MOCKChainResponder,
		},
		wantErr: false,
	}
	t.Run(test.name, func(t *testing.T) {
		if err := MOCKChainListener.AddResponder(test.args.r); (err != nil) != test.wantErr {
			t.Errorf("chainListener.AddResponder() error = %v, wantErr %v", err, test.wantErr)
		}
	})
}
