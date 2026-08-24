package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testkeeper "github.com/solva-solutions/neutron/v11/testutil/contractmanager/keeper"
	"github.com/solva-solutions/neutron/v11/x/contractmanager/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.ContractManagerKeeper(t, nil)
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
