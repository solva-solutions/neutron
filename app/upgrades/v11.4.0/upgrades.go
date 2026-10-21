package v11_4_0

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/solva-solutions/neutron/v11/app/upgrades"
)

func CreateUpgradeHandler(
	mm *module.Manager,
	configurator module.Configurator,
	keepers *upgrades.UpgradeKeepers,
	_ upgrades.StoreKeys,
	_ codec.Codec,
) upgradetypes.UpgradeHandler {
	return func(c context.Context, _ upgradetypes.Plan, vm module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(c)

		ctx.Logger().Info("Starting module migrations...")

		vm, err := mm.RunMigrations(ctx, configurator, vm)
		if err != nil {
			return vm, err
		}

		if err := RestrictCodeUpload(ctx, keepers.WasmKeeper); err != nil {
			return vm, err
		}

		ctx.Logger().Info(fmt.Sprintf("Migration {%s} applied", UpgradeName))
		return vm, nil
	}
}

// RestrictCodeUpload forbids wasm code upload for every account. Governance can
// only execute software upgrades, so new code can only be stored by an upgrade
// handler through the gov permission keeper.
func RestrictCodeUpload(ctx sdk.Context, wk *wasmkeeper.Keeper) error {
	params := wk.GetParams(ctx)
	params.CodeUploadAccess = wasmtypes.AllowNobody
	if err := wk.SetParams(ctx, params); err != nil {
		return fmt.Errorf("failed to set wasm params: %w", err)
	}
	ctx.Logger().Info("wasm code upload access set to Nobody")
	return nil
}
