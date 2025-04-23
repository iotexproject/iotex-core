package testutil

import "github.com/iotexproject/iotex-core/v2/blockchain/genesis"

// NormalizeGenesisHeights normalizes the heights in the genesis config
// it's used in tests to make sure the heights are monotonically increasing
func NormalizeGenesisHeights(g *genesis.Blockchain) {
	heights := []*uint64{
		&g.PacificBlockHeight,
		&g.AleutianBlockHeight,
		&g.BeringBlockHeight,
		&g.CookBlockHeight,
		&g.DardanellesBlockHeight,
		&g.DaytonaBlockHeight,
		&g.EasterBlockHeight,
		&g.FbkMigrationBlockHeight,
		&g.FairbankBlockHeight,
		&g.GreenlandBlockHeight,
		&g.HawaiiBlockHeight,
		&g.IcelandBlockHeight,
		&g.JutlandBlockHeight,
		&g.KamchatkaBlockHeight,
		&g.LordHoweBlockHeight,
		&g.MidwayBlockHeight,
		&g.NewfoundlandBlockHeight,
		&g.OkhotskBlockHeight,
		&g.PalauBlockHeight,
		&g.QuebecBlockHeight,
		&g.RedseaBlockHeight,
		&g.SumatraBlockHeight,
		&g.TsunamiBlockHeight,
		&g.UpernavikBlockHeight,
		&g.VanuatuBlockHeight,
		&g.WakeBlockHeight,
		&g.ToBeEnabledBlockHeight,
	}
	for i := len(heights) - 2; i >= 0; i-- {
		if *(heights[i]) > *(heights[i+1]) {
			*(heights[i]) = *(heights[i+1])
		}
	}
}
