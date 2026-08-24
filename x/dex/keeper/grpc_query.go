package keeper

import (
	"github.com/solva-solutions/neutron/v11/x/dex/types"
)

var _ types.QueryServer = Keeper{}
