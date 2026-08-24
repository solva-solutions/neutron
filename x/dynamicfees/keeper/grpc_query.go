package keeper

import (
	"github.com/solva-solutions/neutron/v11/x/dynamicfees/types"
)

var _ types.QueryServer = Keeper{}
