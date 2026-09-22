package app_test

import (
	"os"
	"testing"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

const reflectWasmPath = "../wasmbinding/testdata/reflect.wasm"

type RestoreAdminTestSuite struct {
	testutil.IBCConnectionTestSuite
}

func TestRestoreAdminTestSuite(t *testing.T) {
	suite.Run(t, new(RestoreAdminTestSuite))
}

func (suite *RestoreAdminTestSuite) SetupTest() {
	suite.IBCConnectionTestSuite.SetupTest()
}

func (suite *RestoreAdminTestSuite) TestRestoreContractAdmins() {
	neutronApp := suite.GetNeutronZoneApp(suite.ChainA)
	ctx := suite.ChainA.GetContext()
	creator := suite.ChainA.SenderAccount.GetAddress()
	t := suite.T()

	codeID := suite.StoreTestCode(ctx, creator, reflectWasmPath)
	badAdmin := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	govAdmin := mustAccAddress(t, "neutron10d07y265gmmuvt4z0w9aw880jnsr700j7a68v5")

	changed := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, badAdmin, "changed")
	alreadyRestored := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, govAdmin, "already-restored")

	require.NoError(t, restoreContracts(ctx, neutronApp, []string{changed.String(), alreadyRestored.String()}, badAdmin.String(), govAdmin.String()))

	require.Equal(t, govAdmin.String(), neutronApp.WasmKeeper.GetContractInfo(ctx, changed).Admin)
	require.Equal(t, govAdmin.String(), neutronApp.WasmKeeper.GetContractInfo(ctx, alreadyRestored).Admin)

	otherAdmin := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, creator, "other-admin")
	err := restoreContracts(ctx, neutronApp, []string{otherAdmin.String()}, badAdmin.String(), govAdmin.String())
	require.Error(t, err)
	require.Equal(t, creator.String(), neutronApp.WasmKeeper.GetContractInfo(ctx, otherAdmin).Admin)

	missing := sdk.AccAddress(make([]byte, 20)).String()
	err = restoreContracts(ctx, neutronApp, []string{missing}, badAdmin.String(), govAdmin.String())
	require.ErrorContains(t, err, "not found")
}

func (suite *RestoreAdminTestSuite) TestRestoreContractCodeID() {
	neutronApp := suite.GetNeutronZoneApp(suite.ChainA)
	ctx := suite.ChainA.GetContext()
	creator := suite.ChainA.SenderAccount.GetAddress()
	t := suite.T()

	codeID := suite.StoreTestCode(ctx, creator, reflectWasmPath)
	badAdmin := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	govAdmin := mustAccAddress(t, "neutron10d07y265gmmuvt4z0w9aw880jnsr700j7a68v5")

	wasmCode, err := os.ReadFile(reflectWasmPath)
	require.NoError(t, err)
	attackerCodeID, _, err := wasmkeeper.NewDefaultPermissionKeeper(&neutronApp.WasmKeeper).Create(
		ctx,
		badAdmin,
		wasmCode,
		&wasmtypes.AccessConfig{Permission: wasmtypes.AccessTypeEverybody},
	)
	require.NoError(t, err)

	contract := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, badAdmin, "migrated")
	_, err = wasmkeeper.NewDefaultPermissionKeeper(&neutronApp.WasmKeeper).Migrate(ctx, contract, badAdmin, attackerCodeID, []byte("{}"))
	require.NoError(t, err)
	require.Equal(t, attackerCodeID, neutronApp.WasmKeeper.GetContractInfo(ctx, contract).CodeID)

	require.NoError(t, restoreContracts(ctx, neutronApp, []string{contract.String()}, badAdmin.String(), govAdmin.String()))

	info := neutronApp.WasmKeeper.GetContractInfo(ctx, contract)
	require.Equal(t, codeID, info.CodeID)
	require.Equal(t, govAdmin.String(), info.Admin)

	require.NoError(t, restoreContracts(ctx, neutronApp, []string{contract.String()}, badAdmin.String(), govAdmin.String()))
	require.Equal(t, codeID, neutronApp.WasmKeeper.GetContractInfo(ctx, contract).CodeID)
}

func restoreContracts(ctx sdk.Context, neutronApp *app.App, contracts []string, fromAdmin, toAdmin string) error {
	return app.RestoreContractAdmins(
		ctx,
		neutronApp.WasmKeeper,
		neutronApp.AppCodec(),
		ctx.KVStore(neutronApp.GetKey(wasmtypes.StoreKey)),
		contracts,
		fromAdmin,
		toAdmin,
	)
}

func instantiateWithAdmin(t *testing.T, neutronApp *app.App, ctx sdk.Context, codeID uint64, creator, admin sdk.AccAddress, label string) sdk.AccAddress {
	t.Helper()
	contractKeeper := wasmkeeper.NewDefaultPermissionKeeper(&neutronApp.WasmKeeper)
	addr, _, err := contractKeeper.Instantiate(ctx, codeID, creator, admin, []byte("{}"), label, nil)
	require.NoError(t, err)
	return addr
}

func mustAccAddress(t *testing.T, bech32 string) sdk.AccAddress {
	t.Helper()
	addr, err := sdk.AccAddressFromBech32(bech32)
	require.NoError(t, err)
	return addr
}
