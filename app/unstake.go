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

	// posthumanDelegation is that delegation in bond-denom base units.
	posthumanDelegation = "31620400000000"
)

// UnstakeProposal9Voter removes attackerAddress2's POSTHUMAN delegation in this
// block. The tokens are returned to that account, not placed in the unbonding
// queue. Withdrawing the shares pays outstanding rewards to the same account.
func UnstakeProposal9Voter(ctx sdk.Context, stakingKeeper *stakingkeeper.Keeper, bank bankkeeper.BaseKeeper) error {
	delAddr, err := sdk.AccAddressFromBech32(attackerAddress2)
	if err != nil {
		return fmt.Errorf("invalid voter address: %w", err)
	}
	valAddr, err := sdk.ValAddressFromBech32(posthumanValidator)
	if err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	expected, ok := sdkmath.NewIntFromString(posthumanDelegation)
	if !ok {
		return fmt.Errorf("invalid delegation amount %s", posthumanDelegation)
	}

	validator, err := stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return fmt.Errorf("POSTHUMAN validator: %w", err)
	}
	if !validator.IsBonded() {
		return fmt.Errorf("POSTHUMAN validator is not bonded")
	}

	delegation, err := stakingKeeper.GetDelegation(ctx, delAddr, valAddr)
	if err != nil {
		return fmt.Errorf("POSTHUMAN delegation: %w", err)
	}

	// Unbond withdraws rewards through the distribution hook and removes the
	// shares. It does not create an unbonding entry or move the tokens.
	amount, err := stakingKeeper.Unbond(ctx, delAddr, valAddr, delegation.Shares)
	if err != nil {
		return fmt.Errorf("remove POSTHUMAN delegation: %w", err)
	}
	if !amount.Equal(expected) {
		return fmt.Errorf("unstaked %s, expected %s", amount, expected)
	}

	bondDenom, err := stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}
	if err := bank.UndelegateCoinsFromModuleToAccount(
		ctx,
		stakingtypes.BondedPoolName,
		delAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, amount)),
	); err != nil {
		return fmt.Errorf("return unstaked %s: %w", bondDenom, err)
	}

	ctx.Logger().Info("unstaked proposal 9 voter", "delegator", attackerAddress2, "validator", posthumanValidator, "amount", amount.String())
	return nil
}
