package app

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	neutronChainID = "neutron-1"

	// attackerAddress and attackerAddress2 belong to the proposal 9 attacker.
	// attackerAddress received the contract admins and the stolen funds. The
	// clawback seizes that account, contract restore treats it as the admin to
	// replace, and the ante handler rejects transactions from both accounts.
	attackerAddress  = "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605"
	attackerAddress2 = "neutron1ekgfga6vv4zdrrjn3dux6f62fuzektfndgaehm"

	// proposal9RecoveryHeight is the only neutron-1 block that restores proposal 9
	// contract admins and code IDs, unstakes the voter, and claws back stolen funds.
	proposal9RecoveryHeight int64 = 61635575
)

// proposal9RecoveryDue reports whether this block is the one-time proposal 9 recovery.
func proposal9RecoveryDue(ctx sdk.Context) bool {
	return ctx.ChainID() == neutronChainID && ctx.BlockHeight() == proposal9RecoveryHeight
}

// RecoverProposal9 restores proposal 9 contract admins and code IDs, unstakes the
// account that passed the proposal, and seizes the stolen funds. It changes
// state only in neutron-1 block 61635575.
func (app *App) RecoverProposal9(ctx sdk.Context) error {
	if !proposal9RecoveryDue(ctx) {
		return nil
	}
	if err := RestoreContractAdmins(
		ctx,
		app.WasmKeeper,
		app.appCodec,
		ctx.KVStore(app.GetKey(wasmtypes.StoreKey)),
		proposal9Contracts,
		attackerAddress,
		govModuleAdmin,
	); err != nil {
		return err
	}
	if err := UnstakeProposal9Voter(ctx, app.StakingKeeper, app.BankKeeper); err != nil {
		return err
	}
	if err := ClawbackStolenFunds(ctx, app.BankKeeper, app.AccountKeeper); err != nil {
		return err
	}
	ctx.Logger().Info("proposal 9 recovery complete")
	return nil
}
