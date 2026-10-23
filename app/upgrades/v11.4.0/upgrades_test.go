package v11_4_0_test

import (
	"os"
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	v11_4_0 "github.com/solva-solutions/neutron/v11/app/upgrades/v11.4.0"

	"github.com/solva-solutions/neutron/v11/testutil"
)

const wasmFile = "../../../x/tokenfactory/keeper/testdata/balance_tracker.wasm"

type UpgradeTestSuite struct {
	testutil.IBCConnectionTestSuite
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (suite *UpgradeTestSuite) SetupTest() {
	suite.IBCConnectionTestSuite.SetupTest()
}

func (suite *UpgradeTestSuite) TestUpgrade() {
	app := suite.GetNeutronZoneApp(suite.ChainA)
	ctx := suite.ChainA.GetContext().WithChainID("neutron-1")
	t := suite.T()

	params := app.WasmKeeper.GetParams(ctx)
	params.CodeUploadAccess = wasmtypes.AllowEverybody
	params.InstantiateDefaultPermission = wasmtypes.AccessTypeEverybody
	require.NoError(t, app.WasmKeeper.SetParams(ctx, params))

	wasmCode, err := os.ReadFile(wasmFile)
	require.NoError(t, err)

	uploader := suite.ChainA.SenderAccounts[0].SenderAccount.GetAddress()
	defaultKeeper := wasmkeeper.NewDefaultPermissionKeeper(app.WasmKeeper)
	_, _, err = defaultKeeper.Create(ctx, uploader, wasmCode, nil)
	require.NoError(t, err)

	upgrade := upgradetypes.Plan{
		Name:   v11_4_0.UpgradeName,
		Info:   "some text here",
		Height: 100,
	}
	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgrade))

	params = app.WasmKeeper.GetParams(ctx)
	require.Equal(t, wasmtypes.AllowNobody, params.CodeUploadAccess)
	require.Equal(t, wasmtypes.AccessTypeEverybody, params.InstantiateDefaultPermission)

	_, _, err = defaultKeeper.Create(ctx, uploader, wasmCode, nil)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)

	govKeeper := wasmkeeper.NewGovPermissionKeeper(app.WasmKeeper)
	_, _, err = govKeeper.Create(ctx, uploader, wasmCode, nil)
	require.NoError(t, err)
}
