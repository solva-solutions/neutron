package app_test

import (
	"os"
	"testing"

	"cosmossdk.io/store/prefix"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

const (
	reflectWasmPath = "../wasmbinding/testdata/reflect.wasm"
	satellite       = "neutron1ffus553eet978k024lmssw0czsxwr97mggyv85lpcsdkft8v9ufsz3sa07"
	pairCW2         = `{"contract":"astroport-pair-concentrated","version":"1.2.13"}`
)

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
	ownAdmin := mustAccAddress(t, satellite)

	changed := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, badAdmin, "changed")
	setCW2(ctx, neutronApp, changed, app.AttackerCW2)
	alreadyRestored := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, ownAdmin, "already-restored")
	setCW2(ctx, neutronApp, alreadyRestored, pairCW2)
	require.Equal(t, app.AttackerCW2, getCW2(ctx, neutronApp, changed))

	// Block 61635574 does not restore anything.
	require.NoError(t, app.RestoreContractAdmins(
		ctx.WithChainID("neutron-1").WithBlockHeight(61635574),
		neutronApp.WasmKeeper,
		neutronApp.AppCodec(),
		ctx.KVStore(neutronApp.GetKey(wasmtypes.StoreKey)),
		[]app.ContractRestore{{changed.String(), satellite, pairCW2}},
		badAdmin.String(),
	))
	require.Equal(t, badAdmin.String(), neutronApp.WasmKeeper.GetContractInfo(ctx, changed).Admin)
	require.Equal(t, app.AttackerCW2, getCW2(ctx, neutronApp, changed))

	require.NoError(t, restoreContracts(ctx, neutronApp, []app.ContractRestore{
		{changed.String(), satellite, pairCW2},
		{alreadyRestored.String(), satellite, pairCW2},
	}, badAdmin.String()))

	for _, contract := range []sdk.AccAddress{changed, alreadyRestored} {
		require.Equal(t, satellite, neutronApp.WasmKeeper.GetContractInfo(ctx, contract).Admin)
		require.Equal(t, pairCW2, getCW2(ctx, neutronApp, contract))
	}

	otherAdmin := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, creator, "other-admin")
	setCW2(ctx, neutronApp, otherAdmin, app.AttackerCW2)
	err := restoreContracts(ctx, neutronApp, []app.ContractRestore{{otherAdmin.String(), satellite, pairCW2}}, badAdmin.String())
	require.ErrorContains(t, err, "admin is")

	// Code 5399 can rewrite its own storage in block 61635574; cw2 is overwritten anyway.
	otherCW2 := instantiateWithAdmin(t, neutronApp, ctx, codeID, creator, badAdmin, "other-cw2")
	setCW2(ctx, neutronApp, otherCW2, `{"contract":"something-else","version":"1.0.0"}`)
	require.NoError(t, restoreContracts(ctx, neutronApp, []app.ContractRestore{{otherCW2.String(), satellite, pairCW2}}, badAdmin.String()))
	require.Equal(t, pairCW2, getCW2(ctx, neutronApp, otherCW2))
	require.Equal(t, satellite, neutronApp.WasmKeeper.GetContractInfo(ctx, otherCW2).Admin)

	missing := sdk.AccAddress(make([]byte, 20)).String()
	err = restoreContracts(ctx, neutronApp, []app.ContractRestore{{missing, satellite, pairCW2}}, badAdmin.String())
	require.ErrorContains(t, err, "not found")
}

func (suite *RestoreAdminTestSuite) TestRestoreContractCodeID() {
	neutronApp := suite.GetNeutronZoneApp(suite.ChainA)
	ctx := suite.ChainA.GetContext()
	creator := suite.ChainA.SenderAccount.GetAddress()
	t := suite.T()

	codeID := suite.StoreTestCode(ctx, creator, reflectWasmPath)
	badAdmin := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")

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
	setCW2(ctx, neutronApp, contract, app.AttackerCW2)

	restore := []app.ContractRestore{{contract.String(), satellite, pairCW2}}
	require.NoError(t, restoreContracts(ctx, neutronApp, restore, badAdmin.String()))

	info := neutronApp.WasmKeeper.GetContractInfo(ctx, contract)
	require.Equal(t, codeID, info.CodeID)
	require.Equal(t, satellite, info.Admin)
	require.Equal(t, pairCW2, getCW2(ctx, neutronApp, contract))

	require.NoError(t, restoreContracts(ctx, neutronApp, restore, badAdmin.String()))
	require.Equal(t, codeID, neutronApp.WasmKeeper.GetContractInfo(ctx, contract).CodeID)
}

func restoreContracts(ctx sdk.Context, neutronApp *app.App, contracts []app.ContractRestore, attacker string) error {
	return app.RestoreContractAdmins(
		ctx.WithChainID("neutron-1").WithBlockHeight(61635575),
		neutronApp.WasmKeeper,
		neutronApp.AppCodec(),
		ctx.KVStore(neutronApp.GetKey(wasmtypes.StoreKey)),
		contracts,
		attacker,
	)
}

func contractStore(ctx sdk.Context, neutronApp *app.App, contract sdk.AccAddress) prefix.Store {
	return prefix.NewStore(ctx.KVStore(neutronApp.GetKey(wasmtypes.StoreKey)), wasmtypes.GetContractStorePrefix(contract))
}

func setCW2(ctx sdk.Context, neutronApp *app.App, contract sdk.AccAddress, cw2 string) {
	contractStore(ctx, neutronApp, contract).Set([]byte("contract_info"), []byte(cw2))
}

// getCW2 reads through the wasm keeper, so a wrong storage prefix in the restore fails the test.
func getCW2(ctx sdk.Context, neutronApp *app.App, contract sdk.AccAddress) string {
	return string(neutronApp.WasmKeeper.QueryRaw(ctx, contract, []byte("contract_info")))
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
