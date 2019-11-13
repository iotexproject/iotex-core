package api

import (
	"testing"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

var MOCKChainListener Listener

// Mock ChainListener
func TestNewChainListener(t *testing.T) {
	t.Run("New Chain listener.", func(t *testing.T) {
		MOCKChainListener = NewChainListener()
	})
}

func Test_chainListener_Start(t *testing.T) {
	t.Run("Test start Chain listener.", func(t *testing.T) {
		if err := MOCKChainListener.Start(); err != nil {
			t.Errorf("chainListener.Start() error = %v,", err)
		}
	})
}

func Test_chainListener_Stop(t *testing.T) {
	t.Run("Test Stop lisntener", func(t *testing.T) {
		if err := MOCKChainListener.Stop(); err != nil {
			t.Errorf("chainListener.Stop() error = %v.", err)
		}
	})
}

func Test_chainListener_HandleBlock(t *testing.T) {
	t.Run("Test HandleBlock.", func(t *testing.T) {
		if err := MOCKChainListener.HandleBlock(new(block.Block)); err != nil {
			t.Errorf("chainListener.HandleBlock() error = %v.", err)
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
	t.Run("Test AddResponder", func(t *testing.T) {
		if err := MOCKChainListener.AddResponder(MOCKChainResponder); err != nil {
			t.Errorf("chainListener.AddResponder() error = %v.", err)
		}
	})
}
