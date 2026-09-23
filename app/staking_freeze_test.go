package app_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/solva-solutions/neutron/v11/app"
)

func TestStakingFreezeDecorator(t *testing.T) {
	decorator := app.NewStakingFreezeDecorator()
	neutronCtx := sdk.Context{}.WithChainID("neutron-1")
	otherChain := sdk.Context{}.WithChainID("testing")
	next := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	}

	delegate := &stakingtypes.MsgDelegate{
		DelegatorAddress: "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605",
		ValidatorAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
		Amount:           sdk.NewInt64Coin("untrn", 1),
	}
	_, err := decorator.AnteHandle(neutronCtx, freezeTx{msgs: []sdk.Msg{delegate}}, false, next)
	require.ErrorIs(t, err, app.ErrStakingFrozen)

	send := &banktypes.MsgSend{
		FromAddress: "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605",
		ToAddress:   "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap",
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("untrn", 1)),
	}
	_, err = decorator.AnteHandle(neutronCtx, freezeTx{msgs: []sdk.Msg{send}}, false, next)
	require.NoError(t, err)

	_, err = decorator.AnteHandle(otherChain, freezeTx{msgs: []sdk.Msg{delegate}}, false, next)
	require.NoError(t, err)

	undelegate := &stakingtypes.MsgUndelegate{
		DelegatorAddress: "neutron1ekgfga6vv4zdrrjn3dux6f62fuzektfndgaehm",
		ValidatorAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
		Amount:           sdk.NewInt64Coin("untrn", 1),
	}
	exec := authz.NewMsgExec(mustAccAddress(t, "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap"), []sdk.Msg{undelegate})
	_, err = decorator.AnteHandle(neutronCtx, freezeTx{msgs: []sdk.Msg{&exec}}, false, next)
	require.ErrorIs(t, err, app.ErrStakingFrozen)
}

func TestFreezeValidatorUpdates(t *testing.T) {
	updates := []abci.ValidatorUpdate{{}}

	require.Nil(t, app.FreezeValidatorUpdates(sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635574), updates))
	require.Nil(t, app.FreezeValidatorUpdates(sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635576), updates))
	require.Equal(t, updates, app.FreezeValidatorUpdates(sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635575), updates))
	require.Equal(t, updates, app.FreezeValidatorUpdates(sdk.Context{}.WithChainID("testing").WithBlockHeight(1), updates))
}

type freezeTx struct {
	msgs []sdk.Msg
}

func (m freezeTx) GetMsgs() []sdk.Msg                    { return m.msgs }
func (m freezeTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
