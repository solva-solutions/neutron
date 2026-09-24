package app

import (
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cronkeeper "github.com/solva-solutions/neutron/v11/x/cron/keeper"
)

// UndelegationsCronSchedule is the cron schedule DisableUndelegationsManager removes.
const UndelegationsCronSchedule = undelegationsCronSchedule

// DisableUndelegationsManagerForTest runs DisableUndelegationsManager against
// test contract addresses.
func DisableUndelegationsManagerForTest(ctx sdk.Context, cron *cronkeeper.Keeper, wasmKeeper *wasmkeeper.Keeper, manager, proxy string) error {
	return disableUndelegationsManager(ctx, cron, wasmKeeper, manager, proxy)
}

// ClawbackTransfersForTest returns the clawback sources and amounts.
func ClawbackTransfersForTest() (from []string, coins []sdk.Coin) {
	for _, transfer := range clawbackTransfers {
		from = append(from, transfer.from)
		coins = append(coins, transfer.coin)
	}
	return from, coins
}
