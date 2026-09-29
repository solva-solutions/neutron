package app

import (
	"bytes"
	"encoding/json"
	"fmt"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	cronkeeper "github.com/solva-solutions/neutron/v11/x/cron/keeper"
)

const (
	// undelegationsManagerContract gradually undelegates the auth proxy's stake.
	// The v11.0.0 upgrade registered a cron schedule that ticks it every block.
	undelegationsManagerContract = "neutron14esdupa76thkgnqdfy3w3enjlwzg20ry9y6n9jthrg274cpc3a2swzndw5"

	// undelegationsAuthProxy holds the stake and forwards messages from its owner,
	// the undelegations manager, to the staking module.
	undelegationsAuthProxy = "neutron17jsl4t4hhaw37tnhenskrfntm7mv44wzjr3f990hx4p9r5m0gzdqquhtd3"

	undelegationsCronSchedule = "undelegations manager contract tick & burn"

	// authProxyOwnerKey is the auth proxy's storage key for its owner address.
	authProxyOwnerKey = "owner"
)

// DisableUndelegationsManager removes the cron schedule that ticks the
// undelegations manager and makes x/gov the owner of the auth proxy, so the
// manager can no longer forward staking messages. It changes state only in
// neutron-1 block 61635575. A missing schedule or an auth proxy owner other than
// the manager or x/gov returns an error.
func DisableUndelegationsManager(ctx sdk.Context, cron *cronkeeper.Keeper, wasmKeeper *wasmkeeper.Keeper) error {
	if !proposal9RecoveryDue(ctx) {
		return nil
	}
	return disableUndelegationsManager(ctx, cron, wasmKeeper, undelegationsManagerContract, undelegationsAuthProxy)
}

func disableUndelegationsManager(ctx sdk.Context, cron *cronkeeper.Keeper, wasmKeeper *wasmkeeper.Keeper, manager, proxy string) error {
	if _, ok := cron.GetSchedule(ctx, undelegationsCronSchedule); !ok {
		return fmt.Errorf("cron schedule %q not found", undelegationsCronSchedule)
	}
	cron.RemoveSchedule(ctx, undelegationsCronSchedule)
	ctx.Logger().Info("removed cron schedule", "name", undelegationsCronSchedule)

	managerAddr, err := sdk.AccAddressFromBech32(manager)
	if err != nil {
		return fmt.Errorf("invalid undelegations manager %s: %w", manager, err)
	}
	proxyAddr, err := sdk.AccAddressFromBech32(proxy)
	if err != nil {
		return fmt.Errorf("invalid auth proxy %s: %w", proxy, err)
	}
	gov := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	managerOwner, _ := json.Marshal(manager)
	govOwner, _ := json.Marshal(gov)

	switch current := wasmKeeper.QueryRaw(ctx, proxyAddr, []byte(authProxyOwnerKey)); {
	case bytes.Equal(current, govOwner):
		return nil
	case !bytes.Equal(current, managerOwner):
		return fmt.Errorf("auth proxy %s owner is %s, expected %s", proxy, current, managerOwner)
	}

	updateOwner, err := json.Marshal(map[string]any{"update_owner": map[string]string{"owner": gov}})
	if err != nil {
		return err
	}
	if _, err := wasmkeeper.NewDefaultPermissionKeeper(wasmKeeper).Execute(ctx, proxyAddr, managerAddr, updateOwner, nil); err != nil {
		return fmt.Errorf("set auth proxy %s owner to %s: %w", proxy, gov, err)
	}
	if current := wasmKeeper.QueryRaw(ctx, proxyAddr, []byte(authProxyOwnerKey)); !bytes.Equal(current, govOwner) {
		return fmt.Errorf("auth proxy %s owner is %s after update, expected %s", proxy, current, govOwner)
	}
	ctx.Logger().Info("set auth proxy owner", "contract", proxy, "owner", gov)
	return nil
}
