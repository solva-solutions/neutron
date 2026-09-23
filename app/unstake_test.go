package app_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

func TestUnstakeProposal9Voter(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{})

	voter := mustAccAddress(t, "neutron1ekgfga6vv4zdrrjn3dux6f62fuzektfndgaehm")
	valAddr, err := sdk.ValAddressFromBech32("neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa")
	require.NoError(t, err)
	stake := sdkmath.NewInt(31_620_400_000_000)
	bondDenom, err := neutronApp.StakingKeeper.BondDenom(ctx)
	require.NoError(t, err)

	validator, err := stakingtypes.NewValidator(valAddr.String(), ed25519.GenPrivKey().PubKey(), stakingtypes.Description{Moniker: "POSTHUMAN"})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	validator.Tokens = sdkmath.ZeroInt()
	validator.DelegatorShares = sdkmath.LegacyZeroDec()
	require.NoError(t, neutronApp.StakingKeeper.SetValidator(ctx, validator))
	require.NoError(t, neutronApp.StakingKeeper.Hooks().AfterValidatorCreated(ctx, valAddr))

	require.NoError(t, neutronApp.BankKeeper.MintCoins(ctx, "mint", sdk.NewCoins(sdk.NewCoin(bondDenom, stake))))
	require.NoError(t, neutronApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", voter, sdk.NewCoins(sdk.NewCoin(bondDenom, stake))))

	validator, err = neutronApp.StakingKeeper.GetValidator(ctx, valAddr)
	require.NoError(t, err)
	_, err = neutronApp.StakingKeeper.Delegate(ctx, voter, stake, stakingtypes.Unbonded, validator, true)
	require.NoError(t, err)

	require.NoError(t, app.UnstakeProposal9Voter(ctx, neutronApp.StakingKeeper, neutronApp.BankKeeper))

	require.True(t, stake.Equal(neutronApp.BankKeeper.GetBalance(ctx, voter, bondDenom).Amount))
	_, err = neutronApp.StakingKeeper.GetDelegation(ctx, voter, valAddr)
	require.Error(t, err)
	_, err = neutronApp.StakingKeeper.GetUnbondingDelegation(ctx, voter, valAddr)
	require.Error(t, err)
}
