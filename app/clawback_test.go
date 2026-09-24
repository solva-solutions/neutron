package app_test

import (
	"testing"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

func TestClawbackStolenFunds(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{})
	bank := neutronApp.BankKeeper

	attacker := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	payout := mustAccAddress(t, "neutron15m3hh904d0cwe4t69ec2w5cmc8535gkxwhgrp5")
	recipient := mustAccAddress(t, "neutron1yr29fd7uzdjp2jsq8hrta8mvyd6ex7vumn0shy")
	usdc := "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81"

	// Fund every listed amount, plus the attacker's own NTRN and an unrelated account.
	sources, coins := app.ClawbackTransfersForTest()
	seized := sdk.NewCoins()
	for i, coin := range coins {
		from := mustAccAddress(t, sources[i])
		balance := bank.GetBalance(ctx, from, coin.Denom).Add(coin)
		require.NoError(t, bank.Balances.Set(ctx, collections.Join(from, coin.Denom), balance.Amount))
		seized = seized.Add(coin)
	}
	ownNTRN := sdkmath.NewInt(1_006_122_256_417)
	attackerNTRN := bank.GetBalance(ctx, attacker, "untrn").Amount.Add(ownNTRN)
	require.NoError(t, bank.Balances.Set(ctx, collections.Join(attacker, "untrn"), attackerNTRN))
	require.NoError(t, bank.Balances.Set(ctx, collections.Join(payout, usdc), sdkmath.NewInt(1_000_000_035)))
	require.NoError(t, bank.Balances.Set(ctx, collections.Join(payout, "untrn"), sdkmath.NewInt(50)))
	supplyBefore := bank.GetSupply(ctx, "untrn")

	require.NoError(t, app.ClawbackStolenFunds(ctx, bank, neutronApp.AccountKeeper))

	for _, source := range sources {
		for _, coin := range bank.GetAllBalances(ctx, mustAccAddress(t, source)) {
			if mustAccAddress(t, source).Equals(attacker) && coin.Denom == "untrn" {
				require.True(t, ownNTRN.Equal(coin.Amount), "attacker keeps its own NTRN")
				continue
			}
			require.Failf(t, "balance left after clawback", "%s holds %s", source, coin)
		}
	}
	require.Equal(t, seized.String(), bank.GetAllBalances(ctx, recipient).String())
	require.True(t, sdkmath.NewInt(1_000_000_035).Equal(bank.GetBalance(ctx, payout, usdc).Amount))
	require.True(t, sdkmath.NewInt(50).Equal(bank.GetBalance(ctx, payout, "untrn").Amount))
	require.True(t, neutronApp.AccountKeeper.HasAccount(ctx, recipient))
	require.Equal(t, supplyBefore, bank.GetSupply(ctx, "untrn"))

	// A listed amount that is no longer covered fails the recovery block.
	cacheCtx, _ := ctx.CacheContext()
	require.ErrorContains(t, app.ClawbackStolenFunds(cacheCtx, bank, neutronApp.AccountKeeper), "spendable balance")
}
