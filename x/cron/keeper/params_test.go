package keeper_test

import (
	"testing"

	"github.com/solva-solutions/neutron/v11/app/config"

	"github.com/solva-solutions/neutron/v11/testutil"

	testkeeper "github.com/solva-solutions/neutron/v11/testutil/cron/keeper"

	"github.com/stretchr/testify/require"

	"github.com/solva-solutions/neutron/v11/x/cron/types"
)

func TestGetParams(t *testing.T) {
	_ = config.GetDefaultConfig()

	k, ctx := testkeeper.CronKeeper(t, nil, nil)
	params := types.Params{
		SecurityAddress: testutil.TestOwnerAddress,
		Limit:           5,
	}

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
