package app

import sdk "github.com/cosmos/cosmos-sdk/types"

// ClawbackTransfersForTest returns the clawback sources and amounts.
func ClawbackTransfersForTest() (from []string, coins []sdk.Coin) {
	for _, transfer := range clawbackTransfers {
		from = append(from, transfer.from)
		coins = append(coins, transfer.coin)
	}
	return from, coins
}
