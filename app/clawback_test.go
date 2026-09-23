package app_test

import (
	"testing"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

func TestClawbackStolenFunds(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{})

	attacker := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	payout := mustAccAddress(t, "neutron15m3hh904d0cwe4t69ec2w5cmc8535gkxwhgrp5")
	recipient := mustAccAddress(t, "neutron1yr29fd7uzdjp2jsq8hrta8mvyd6ex7vumn0shy")
	usdc := "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81"

	ownNTRN := sdkmath.NewInt(1_006_122_256_417)
	stolenNTRN := sdkmath.NewIntFromUint64(95_196_289_988_992)
	require.NoError(t, neutronApp.BankKeeper.Balances.Set(ctx, collections.Join(attacker, "untrn"), stolenNTRN.Add(ownNTRN)))
	require.NoError(t, neutronApp.BankKeeper.Balances.Set(ctx, collections.Join(attacker, usdc), sdkmath.NewInt(1_671_301_712_957)))
	require.NoError(t, neutronApp.BankKeeper.Balances.Set(ctx, collections.Join(payout, usdc), sdkmath.NewInt(1_000_000_035)))
	require.NoError(t, neutronApp.BankKeeper.Balances.Set(ctx, collections.Join(payout, "untrn"), sdkmath.NewInt(50)))

	require.NoError(t, app.ClawbackStolenFunds(ctx, neutronApp.BankKeeper, neutronApp.AccountKeeper))

	require.True(t, ownNTRN.Equal(neutronApp.BankKeeper.GetBalance(ctx, attacker, "untrn").Amount))
	require.True(t, neutronApp.BankKeeper.GetBalance(ctx, attacker, usdc).IsZero())
	require.True(t, sdkmath.NewInt(1_000_000_035).Equal(neutronApp.BankKeeper.GetBalance(ctx, payout, usdc).Amount))
	require.True(t, sdkmath.NewInt(50).Equal(neutronApp.BankKeeper.GetBalance(ctx, payout, "untrn").Amount))
	require.True(t, stolenNTRN.Equal(neutronApp.BankKeeper.GetBalance(ctx, recipient, "untrn").Amount))
	require.True(t, sdkmath.NewInt(1_671_301_712_957).Equal(neutronApp.BankKeeper.GetBalance(ctx, recipient, usdc).Amount))
	require.True(t, neutronApp.AccountKeeper.HasAccount(ctx, recipient))

	require.NoError(t, app.ClawbackStolenFunds(ctx, neutronApp.BankKeeper, neutronApp.AccountKeeper))
	require.True(t, ownNTRN.Equal(neutronApp.BankKeeper.GetBalance(ctx, attacker, "untrn").Amount))
	require.True(t, sdkmath.NewInt(1_000_000_035).Equal(neutronApp.BankKeeper.GetBalance(ctx, payout, usdc).Amount))
	require.True(t, sdkmath.NewInt(50).Equal(neutronApp.BankKeeper.GetBalance(ctx, payout, "untrn").Amount))
}
