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

func TestProposal9RecoveryOnlyAtHeight(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{})

	attacker := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	usdc := "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81"
	listed := sdkmath.NewInt(1_671_301_712_957)
	require.NoError(t, neutronApp.BankKeeper.Balances.Set(ctx, collections.Join(attacker, usdc), listed))

	// Blocks other than neutron-1 height 61635575 leave a fresh deposit in place.
	for _, height := range []int64{61635574, 61635576} {
		require.NoError(t, neutronApp.RecoverProposal9(ctx.WithChainID("neutron-1").WithBlockHeight(height)))
		require.True(t, listed.Equal(neutronApp.BankKeeper.GetBalance(ctx, attacker, usdc).Amount))
	}
	require.NoError(t, neutronApp.RecoverProposal9(ctx.WithChainID("testing").WithBlockHeight(61635575)))
	require.True(t, listed.Equal(neutronApp.BankKeeper.GetBalance(ctx, attacker, usdc).Amount))

	// The recovery block restores contracts first. These mainnet contracts are
	// not on this chain, so the block fails before the unstake and clawback.
	require.Error(t, neutronApp.RecoverProposal9(ctx.WithChainID("neutron-1").WithBlockHeight(61635575)))
}
