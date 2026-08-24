package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/solva-solutions/neutron/v11/testutil/contractmanager/keeper"
	"github.com/solva-solutions/neutron/v11/x/contractmanager/types"
)

func TestMsgUpdateParamsValidate(t *testing.T) {
	k, ctx := keeper.ContractManagerKeeper(t, nil)

	tests := []struct {
		name        string
		msg         types.MsgUpdateParams
		expectedErr string
	}{
		{
			"empty authority",
			types.MsgUpdateParams{
				Authority: "",
			},
			"authority is invalid",
		},
		{
			"invalid authority",
			types.MsgUpdateParams{
				Authority: "invalid authority",
			},
			"authority is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := k.UpdateParams(ctx, &tt.msg)
			require.ErrorContains(t, err, tt.expectedErr)
			require.Nil(t, resp)
		})
	}
}
