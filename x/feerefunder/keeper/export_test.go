package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/solva-solutions/neutron/v11/x/feerefunder/types"
)

func (k Keeper) CheckFees(ctx sdk.Context, fees types.Fee) error {
	return k.checkFees(ctx, fees)
}
