package app

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	// posthumanValidator is the validator neutron1ekgfga6… delegated to.
	posthumanValidator = "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa"

	// posthumanDelegation is that delegation in bond-denom base units at the halt.
	// Undelegations or a slash of POSTHUMAN in block 61635574 can move the real
	// amount slightly, so it is only compared and logged.
	posthumanDelegation = "31620400000000"
)

// UnstakeProposal9Voter removes attackerAddress2's POSTHUMAN delegation in this
// block and returns the amount it released. The tokens are returned to that
// account, not placed in the unbonding queue. Withdrawing the shares pays
// outstanding rewards to the same account.
func UnstakeProposal9Voter(ctx sdk.Context, stakingKeeper *stakingkeeper.Keeper, bank bankkeeper.BaseKeeper) (sdkmath.Int, error) {
	delAddr, err := sdk.AccAddressFromBech32(attackerAddress2)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("invalid voter address: %w", err)
	}
	valAddr, err := sdk.ValAddressFromBech32(posthumanValidator)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("invalid validator address: %w", err)
	}

	validator, err := stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("POSTHUMAN validator: %w", err)
	}
	// A bonded validator's tokens are in the bonded pool; a jailed, unbonding,
	// or unbonded validator's tokens are in the not-bonded pool.
	pool := stakingtypes.NotBondedPoolName
	if validator.IsBonded() {
		pool = stakingtypes.BondedPoolName
	}

	delegation, err := stakingKeeper.GetDelegation(ctx, delAddr, valAddr)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("POSTHUMAN delegation: %w", err)
	}

	// Unbond withdraws rewards through the distribution hook and removes the
	// shares. It does not create an unbonding entry or move the tokens.
	amount, err := stakingKeeper.Unbond(ctx, delAddr, valAddr, delegation.Shares)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("remove POSTHUMAN delegation: %w", err)
	}
	if expected, ok := sdkmath.NewIntFromString(posthumanDelegation); ok && !amount.Equal(expected) {
		ctx.Logger().Error("proposal 9 voter unstake differs from the amount at the halt", "unstaked", amount.String(), "at_halt", posthumanDelegation)
	}

	bondDenom, err := stakingKeeper.BondDenom(ctx)
	if err != nil {
		return sdkmath.Int{}, err
	}
	if err := bank.UndelegateCoinsFromModuleToAccount(ctx, pool, delAddr, sdk.NewCoins(sdk.NewCoin(bondDenom, amount))); err != nil {
		return sdkmath.Int{}, fmt.Errorf("return unstaked %s from %s: %w", bondDenom, pool, err)
	}

	ctx.Logger().Info("unstaked proposal 9 voter", "delegator", attackerAddress2, "validator", posthumanValidator, "amount", amount.String(), "pool", pool)
	return amount, nil
}
