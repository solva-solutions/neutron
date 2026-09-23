package app

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	neutronChainID = "neutron-1"

	// govModuleAdmin is the x/gov module account. It was the contract admin
	// before governance proposal 9.
	govModuleAdmin = "neutron10d07y265gmmuvt4z0w9aw880jnsr700j7a68v5"

	// proposal9Admin is the admin set by governance proposal 9, and the address
	// that uploaded the replacement contract code.
	proposal9Admin = "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605"
)

// proposal9Contracts are the contracts whose admin and code ID proposal 9's
// attacker changed.
var proposal9Contracts = []string{
	"neutron1nfns3ck2ykrs0fknckrzd9728cyf77devuzernhwcwrdxw7ssk2s3tjf8r",
	"neutron1ns2tcunrlrk5yk62fpl74ycazanceyfmmq7dlj6sq8n0rnkuvk7szstkyx",
	"neutron1j4xpv03fw664mvntlhqnzp5hjqk2nfw00vrgx9qlq97rxc9fu3lqvmszl2",
	"neutron1yw0a7nxa8jdgzmdsme4gwxhj76n44z305qgwrzvlfavgna9epcys3k9m2f",
	"neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
	"neutron18c8qejysp4hgcfuxdpj4wf29mevzwllz5yh8uayjxamwtrs0n9fshq9vtv",
	"neutron15024sa6het7qrgccrx9wtxff7mh9qrcu9m6gvvlvez6em46ylscqk4kqj9",
	"neutron1gcklauurx4ckyp0t8a6axl0500vy3n09y5utpspjg8ruxnln8erq2uumck",
	"neutron19sq60cxtsjcx7vw25c63wyt27fxevuh6l7vxcn04u0t0rueyfpvq4mc75l",
	"neutron1zlf3hutsa4qnmue53lz2tfxrutp8y2e3rj4nkghg3rupgl4mqy8s5jgxsn",
	"neutron14q5elxj4ghktt7d7d0uw0cs0gqyeay25h5fkree897gjm38gevxqmvqsq5",
}

// RestoreContractAdmins sets each contract's admin from fromAdmin back to toAdmin
// and points the contract back at the last code ID that fromAdmin did not upload.
// It changes state only in neutron-1 block 61635575. Contracts that already have
// toAdmin are left unchanged. A contract that is missing or has any other admin
// returns an error.
func RestoreContractAdmins(
	ctx sdk.Context,
	wasmKeeper wasmkeeper.Keeper,
	cdc codec.BinaryCodec,
	store storetypes.KVStore,
	contracts []string,
	fromAdmin, toAdmin string,
) error {
	if !proposal9RecoveryDue(ctx) {
		return nil
	}

	newAdmin, err := sdk.AccAddressFromBech32(toAdmin)
	if err != nil {
		return fmt.Errorf("invalid restored admin %s: %w", toAdmin, err)
	}
	if _, err := sdk.AccAddressFromBech32(fromAdmin); err != nil {
		return fmt.Errorf("invalid current admin %s: %w", fromAdmin, err)
	}

	permissioned := wasmkeeper.NewGovPermissionKeeper(&wasmKeeper)
	for _, contract := range contracts {
		contractAddr, err := sdk.AccAddressFromBech32(contract)
		if err != nil {
			return fmt.Errorf("invalid contract address %s: %w", contract, err)
		}

		info := wasmKeeper.GetContractInfo(ctx, contractAddr)
		if info == nil {
			return fmt.Errorf("contract %s not found", contract)
		}
		if err := restoreContractCodeID(ctx, wasmKeeper, cdc, store, contractAddr, info, fromAdmin); err != nil {
			return err
		}
		if info.Admin == toAdmin {
			continue
		}
		if info.Admin != fromAdmin {
			return fmt.Errorf("contract %s admin is %s, expected %s", contract, info.Admin, fromAdmin)
		}

		if err := permissioned.UpdateContractAdmin(ctx, contractAddr, nil, newAdmin); err != nil {
			return fmt.Errorf("restore admin of %s: %w", contract, err)
		}
		ctx.Logger().Info("restored contract admin", "contract", contract, "admin", toAdmin)
	}
	return nil
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
