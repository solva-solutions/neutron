package app_test

import (
	"encoding/json"
	"fmt"
	"testing"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
	crontypes "github.com/solva-solutions/neutron/v11/x/cron/types"
)

// authProxyWasmPath is mainnet code 5313, the auth proxy that owns the stake the
// undelegations manager undelegates.
const authProxyWasmPath = "testdata/auth_proxy_5313.wasm"

type UndelegationsManagerTestSuite struct {
	testutil.IBCConnectionTestSuite
}

func TestUndelegationsManagerTestSuite(t *testing.T) {
	suite.Run(t, new(UndelegationsManagerTestSuite))
}

func (suite *UndelegationsManagerTestSuite) TestDisableUndelegationsManager() {
	neutronApp := suite.GetNeutronZoneApp(suite.ChainA)
	ctx := suite.ChainA.GetContext()
	t := suite.T()
	creator := suite.ChainA.SenderAccount.GetAddress()
	manager := suite.ChainA.SenderAccounts[1].SenderAccount.GetAddress()
	gov := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	contracts := wasmkeeper.NewDefaultPermissionKeeper(&neutronApp.WasmKeeper)

	// On mainnet the auth proxy was migrated from the Drop puppeteer; code 5313
	// only implements migrate.
	reflectID := suite.StoreTestCode(ctx, creator, reflectWasmPath)
	codeID := suite.StoreTestCode(ctx, creator, authProxyWasmPath)
	newProxy := func(owner string) sdk.AccAddress {
		addr := instantiateWithAdmin(t, neutronApp, ctx, reflectID, creator, creator, "auth proxy "+owner)
		_, err := contracts.Migrate(ctx, addr, creator, codeID, []byte(fmt.Sprintf(`{"owner":%q}`, owner)))
		require.NoError(t, err)
		return addr
	}
	proxy := newProxy(manager.String())

	require.NoError(t, neutronApp.CronKeeper.AddSchedule(ctx, app.UndelegationsCronSchedule, 1,
		[]crontypes.MsgExecuteContract{{Contract: proxy.String(), Msg: `{"tick": {}}`}},
		uint64(ctx.BlockHeight()), crontypes.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)) //nolint:gosec

	// Only the recovery block disables anything.
	require.NoError(t, app.DisableUndelegationsManager(ctx.WithChainID("neutron-1").WithBlockHeight(61635574), &neutronApp.CronKeeper, &neutronApp.WasmKeeper))
	_, found := neutronApp.CronKeeper.GetSchedule(ctx, app.UndelegationsCronSchedule)
	require.True(t, found)

	require.NoError(t, app.DisableUndelegationsManagerForTest(ctx, &neutronApp.CronKeeper, &neutronApp.WasmKeeper, manager.String(), proxy.String()))

	_, found = neutronApp.CronKeeper.GetSchedule(ctx, app.UndelegationsCronSchedule)
	require.False(t, found)
	owner, err := neutronApp.WasmKeeper.QuerySmart(ctx, proxy, []byte(`{"owner":{}}`))
	require.NoError(t, err)
	var ownerResp struct {
		Owner string `json:"owner"`
	}
	require.NoError(t, json.Unmarshal(owner, &ownerResp))
	require.Equal(t, gov, ownerResp.Owner)

	// The manager is no longer the owner and cannot take the proxy back.
	reclaim := []byte(fmt.Sprintf(`{"update_owner":{"owner":%q}}`, manager.String()))
	_, err = contracts.Execute(ctx, proxy, manager, reclaim, nil)
	require.ErrorContains(t, err, "Unauthorized")

	// The schedule is gone, so running again fails the block.
	err = app.DisableUndelegationsManagerForTest(ctx, &neutronApp.CronKeeper, &neutronApp.WasmKeeper, manager.String(), proxy.String())
	require.ErrorContains(t, err, "not found")

	// An auth proxy owned by anyone but the manager or x/gov fails the block.
	other := newProxy(creator.String())
	require.NoError(t, neutronApp.CronKeeper.AddSchedule(ctx, app.UndelegationsCronSchedule, 1, nil, uint64(ctx.BlockHeight()), crontypes.ExecutionStage_EXECUTION_STAGE_BEGIN_BLOCKER)) //nolint:gosec
	err = app.DisableUndelegationsManagerForTest(ctx, &neutronApp.CronKeeper, &neutronApp.WasmKeeper, manager.String(), other.String())
	require.ErrorContains(t, err, "owner is")
}
