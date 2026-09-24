package app

import (
	"bytes"
	"fmt"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	astroportSatellite = "neutron1ffus553eet978k024lmssw0czsxwr97mggyv85lpcsdkft8v9ufsz3sa07"

	// cw2Key is the contract storage key of the cw2 contract name and version.
	cw2Key = "contract_info"

	// AttackerCW2 is the cw2 value code 5399's migrate wrote into every contract
	// it migrated.
	AttackerCW2 = `{"contract":"neutron-migration-test-vault","version":"2.0.0"}`
)

// ContractRestore is a proposal 9 contract with its admin and exact cw2 bytes
// before the attack.
type ContractRestore struct {
	Address string
	Admin   string
	CW2     string
}

// proposal9Contracts are the contracts whose admin, code ID, and cw2 proposal 9's
// attacker changed. Admins and cw2 are read from neutron-1 height 61633004, the
// last block before proposal 9 executed.
var proposal9Contracts = []ContractRestore{
	{"neutron1nfns3ck2ykrs0fknckrzd9728cyf77devuzernhwcwrdxw7ssk2s3tjf8r", astroportSatellite, `{"contract":"astroport-pair-concentrated-duality","version":"4.3.1"}`},
	{"neutron1ns2tcunrlrk5yk62fpl74ycazanceyfmmq7dlj6sq8n0rnkuvk7szstkyx", astroportSatellite, `{"contract":"astroport-pair-transmuter","version":"1.0.0"}`},
	{"neutron1j4xpv03fw664mvntlhqnzp5hjqk2nfw00vrgx9qlq97rxc9fu3lqvmszl2", astroportSatellite, `{"contract":"astroport-pair-concentrated","version":"1.2.13"}`},
	{"neutron1yw0a7nxa8jdgzmdsme4gwxhj76n44z305qgwrzvlfavgna9epcys3k9m2f", astroportSatellite, `{"contract":"astroport-pair-concentrated","version":"1.2.15"}`},
	{"neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98", astroportSatellite, `{"contract":"astroport-pair-concentrated","version":"4.0.2"}`},
	{"neutron18c8qejysp4hgcfuxdpj4wf29mevzwllz5yh8uayjxamwtrs0n9fshq9vtv", astroportSatellite, `{"contract":"astroport-pair-concentrated-duality","version":"4.3.1"}`},
	{"neutron15024sa6het7qrgccrx9wtxff7mh9qrcu9m6gvvlvez6em46ylscqk4kqj9", "neutron1xm4xgfv4xz4ccv0tjvlfac5gqwjnv9zzx4l47t7ve7j2sn4k7gwqkg947d", `{"contract":"crates.io:drop-staking__drop-converter","version":"1.0.0"}`},
	{"neutron1gcklauurx4ckyp0t8a6axl0500vy3n09y5utpspjg8ruxnln8erq2uumck", "neutron1rtydk5vlppj2nmw98ctpgee6hxe5va7mc9x3lng8xm75p3dus80s7pkvmu", `{"contract":"crates.io:drop-staking__drop-withdrawal-manager","version":"1.0.0"}`},
	{"neutron19sq60cxtsjcx7vw25c63wyt27fxevuh6l7vxcn04u0t0rueyfpvq4mc75l", astroportSatellite, `{"contract":"astroport-whitelist","version":"2.0.0"}`},
	{"neutron1zlf3hutsa4qnmue53lz2tfxrutp8y2e3rj4nkghg3rupgl4mqy8s5jgxsn", astroportSatellite, `{"contract":"astroport-staking","version":"2.3.0"}`},
	{"neutron14q5elxj4ghktt7d7d0uw0cs0gqyeay25h5fkree897gjm38gevxqmvqsq5", "neutron1d9m09dzfvjzep2jaypg9a80zslvr7jhcary57a", `{"contract":"neutron-vesting-investors","version":"1.1.0"}`},
}

// RestoreContractAdmins gives each contract back its admin, points it at the last
// code ID that attacker did not upload, and writes back its cw2 value. It changes
// state only in neutron-1 block 61635575. Values already restored are left
// unchanged. A missing contract, an admin other than attacker, or a cw2 value
// other than AttackerCW2 returns an error.
func RestoreContractAdmins(
	ctx sdk.Context,
	wasmKeeper wasmkeeper.Keeper,
	cdc codec.BinaryCodec,
	store storetypes.KVStore,
	contracts []ContractRestore,
	attacker string,
) error {
	if !proposal9RecoveryDue(ctx) {
		return nil
	}
	if _, err := sdk.AccAddressFromBech32(attacker); err != nil {
		return fmt.Errorf("invalid attacker address %s: %w", attacker, err)
	}

	permissioned := wasmkeeper.NewGovPermissionKeeper(&wasmKeeper)
	for _, contract := range contracts {
		contractAddr, err := sdk.AccAddressFromBech32(contract.Address)
		if err != nil {
			return fmt.Errorf("invalid contract address %s: %w", contract.Address, err)
		}
		newAdmin, err := sdk.AccAddressFromBech32(contract.Admin)
		if err != nil {
			return fmt.Errorf("contract %s: invalid restored admin %s: %w", contract.Address, contract.Admin, err)
		}

		info := wasmKeeper.GetContractInfo(ctx, contractAddr)
		if info == nil {
			return fmt.Errorf("contract %s not found", contract.Address)
		}
		if err := restoreContractCodeID(ctx, wasmKeeper, cdc, store, contractAddr, info, attacker); err != nil {
			return err
		}
		if err := restoreCW2(ctx, store, contractAddr, contract.CW2); err != nil {
			return err
		}
		if info.Admin == contract.Admin {
			continue
		}
		if info.Admin != attacker {
			return fmt.Errorf("contract %s admin is %s, expected %s", contract.Address, info.Admin, attacker)
		}

		if err := permissioned.UpdateContractAdmin(ctx, contractAddr, nil, newAdmin); err != nil {
			return fmt.Errorf("restore admin of %s: %w", contract.Address, err)
		}
		ctx.Logger().Info("restored contract admin", "contract", contract.Address, "admin", contract.Admin)
	}
	return nil
}

// restoreCW2 writes cw2 back into the contract's storage when it holds AttackerCW2.
func restoreCW2(ctx sdk.Context, store storetypes.KVStore, contractAddr sdk.AccAddress, cw2 string) error {
	contractStore := prefix.NewStore(store, wasmtypes.GetContractStorePrefix(contractAddr))
	current := contractStore.Get([]byte(cw2Key))
	switch {
	case bytes.Equal(current, []byte(cw2)):
		return nil
	case bytes.Equal(current, []byte(AttackerCW2)):
		contractStore.Set([]byte(cw2Key), []byte(cw2))
		ctx.Logger().Info("restored contract cw2", "contract", contractAddr.String(), "cw2", cw2)
		return nil
	default:
		return fmt.Errorf("contract %s cw2 is %q, expected %q or %q", contractAddr, current, AttackerCW2, cw2)
	}
}

// restoreContractCodeID points contractAddr at the latest code ID in its history
// that was not uploaded by attackerAdmin. The wasm migrate entrypoint is not called.
func restoreContractCodeID(
	ctx sdk.Context,
	wasmKeeper wasmkeeper.Keeper,
	cdc codec.BinaryCodec,
	store storetypes.KVStore,
	contractAddr sdk.AccAddress,
	info *wasmtypes.ContractInfo,
	attackerAdmin string,
) error {
	history := wasmKeeper.GetContractHistory(ctx, contractAddr)
	if len(history) == 0 {
		return fmt.Errorf("contract %s has no code history", contractAddr)
	}

	targetCodeID, changed, err := legitimateCodeID(ctx, wasmKeeper, history, attackerAdmin)
	if err != nil {
		return fmt.Errorf("contract %s: %w", contractAddr, err)
	}
	if !changed {
		return nil
	}

	last := history[len(history)-1]
	store.Delete(wasmtypes.GetContractByCreatedSecondaryIndexKey(contractAddr, last))

	entry := info.AddMigration(ctx, targetCodeID, nil)
	infoBz, err := cdc.Marshal(info)
	if err != nil {
		return fmt.Errorf("contract %s: marshal contract info: %w", contractAddr, err)
	}
	store.Set(wasmtypes.GetContractAddressKey(contractAddr), infoBz)

	pos, ok := lastHistoryPosition(store, contractAddr)
	if !ok {
		return fmt.Errorf("contract %s: missing code history position", contractAddr)
	}
	entryBz, err := cdc.Marshal(&entry)
	if err != nil {
		return fmt.Errorf("contract %s: marshal code history: %w", contractAddr, err)
	}
	store.Set(wasmtypes.GetContractCodeHistoryElementKey(contractAddr, pos+1), entryBz)
	store.Set(wasmtypes.GetContractByCreatedSecondaryIndexKey(contractAddr, entry), []byte{})

	ctx.Logger().Info("restored contract code id", "contract", contractAddr.String(), "code_id", targetCodeID)
	return nil
}

// legitimateCodeID returns the newest code ID in history that attackerAdmin did not
// upload. changed is false when the contract already uses that code ID.
func legitimateCodeID(ctx sdk.Context, wasmKeeper wasmkeeper.Keeper, history []wasmtypes.ContractCodeHistoryEntry, attackerAdmin string) (uint64, bool, error) {
	current := history[len(history)-1].CodeID
	for i := len(history) - 1; i >= 0; i-- {
		codeID := history[i].CodeID
		codeInfo := wasmKeeper.GetCodeInfo(ctx, codeID)
		if codeInfo == nil {
			return 0, false, fmt.Errorf("code id %d not found", codeID)
		}
		if codeInfo.Creator == attackerAdmin {
			continue
		}
		return codeID, codeID != current, nil
	}
	return 0, false, fmt.Errorf("code id %d was uploaded by %s and no previous code id exists", current, attackerAdmin)
}

func lastHistoryPosition(store storetypes.KVStore, contractAddr sdk.AccAddress) (uint64, bool) {
	prefix := wasmtypes.GetContractCodeHistoryElementPrefix(contractAddr)
	end := storetypes.PrefixEndBytes(prefix)
	iter := store.ReverseIterator(prefix, end)
	defer func() { _ = iter.Close() }()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) != len(prefix)+8 {
			continue
		}
		return sdk.BigEndianToUint64(key[len(prefix):]), true
	}
	return 0, false
}
