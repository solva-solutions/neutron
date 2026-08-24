package keeper

import (
	"github.com/solva-solutions/neutron/v11/x/interchaintxs/types"
)

var _ types.QueryServer = Keeper{}
