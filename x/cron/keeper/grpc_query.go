package keeper

import (
	"github.com/solva-solutions/neutron/v11/x/cron/types"
)

var _ types.QueryServer = Keeper{}
