package consensusfsm_test

import (
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"

	"github.com/stretchr/testify/require"
)

func TestConsensusConfig_BlockInterval(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	ccfg := consensusfsm.NewConsensusConfig(cfg.Consensus.RollDPoS.FSM, cfg.DardanellesUpgrade, cfg.WakeUpgrade, cfg.Genesis, 2*time.Second)

	// Define test cases
	tests := []struct {
		name           string
		height         uint64
		expectedResult time.Duration
	}{
		{
			name:           "Before Dardanelles and Wake",
			height:         cfg.Genesis.DardanellesBlockHeight - 1,
			expectedResult: 10 * time.Second,
		},
		{
			name:           "At Dardanelles height",
			height:         cfg.Genesis.DardanellesBlockHeight,
			expectedResult: 5 * time.Second,
		},
		{
			name:           "At Wake height",
			height:         cfg.Genesis.WakeBlockHeight,
			expectedResult: 2500 * time.Millisecond,
		},
		{
			name:           "After Wake height",
			height:         cfg.Genesis.WakeBlockHeight + 1,
			expectedResult: 2500 * time.Millisecond,
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ccfg.BlockInterval(tt.height)
			require.Equal(tt.expectedResult, result)
		})
	}
}
