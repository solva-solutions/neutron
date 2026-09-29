package app_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

func TestStakingFreezeDecorator(t *testing.T) {
	decorator := app.NewStakingFreezeDecorator()
	neutronCtx := sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635574)
	otherChain := sdk.Context{}.WithChainID("testing").WithBlockHeight(61635574)
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

	_, err = decorator.AnteHandle(neutronCtx.WithBlockHeight(61635573), freezeTx{msgs: []sdk.Msg{delegate}}, false, next)
	require.NoError(t, err)

	// Moving or removing existing stake still works.
	for _, msg := range []sdk.Msg{
		&stakingtypes.MsgUndelegate{
			DelegatorAddress: "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap",
			ValidatorAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
			Amount:           sdk.NewInt64Coin("untrn", 1),
		},
		&stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap",
			ValidatorSrcAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
			ValidatorDstAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
			Amount:              sdk.NewInt64Coin("untrn", 1),
		},
		&slashingtypes.MsgUnjail{ValidatorAddr: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa"},
	} {
		_, err = decorator.AnteHandle(neutronCtx, freezeTx{msgs: []sdk.Msg{msg}}, false, next)
		require.NoError(t, err, sdk.MsgTypeURL(msg))
	}

	cancel := &stakingtypes.MsgCancelUnbondingDelegation{
		DelegatorAddress: "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap",
		ValidatorAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
		Amount:           sdk.NewInt64Coin("untrn", 1),
		CreationHeight:   1,
	}
	exec := authz.NewMsgExec(mustAccAddress(t, "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap"), []sdk.Msg{cancel})
	_, err = decorator.AnteHandle(neutronCtx, freezeTx{msgs: []sdk.Msg{&exec}}, false, next)
	require.ErrorIs(t, err, app.ErrStakingFrozen)
}

func TestStakingFreezeCircuit(t *testing.T) {
	circuit := app.StakingFreezeCircuit{}
	after := sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635574)

	for _, msg := range []sdk.Msg{
		&stakingtypes.MsgDelegate{},
		&stakingtypes.MsgCancelUnbondingDelegation{},
		&stakingtypes.MsgCreateValidator{},
	} {
		typeURL := sdk.MsgTypeURL(msg)
		allowed, err := circuit.IsAllowed(after, typeURL)
		require.False(t, allowed, typeURL)
		require.ErrorIs(t, err, app.ErrStakingFrozen)

		// Replayed blocks and other chains are not frozen.
		for _, ctx := range []sdk.Context{after.WithBlockHeight(61635573), after.WithChainID("testing")} {
			allowed, err = circuit.IsAllowed(ctx, typeURL)
			require.NoError(t, err)
			require.True(t, allowed, typeURL)
		}
	}

	for _, msg := range []sdk.Msg{
		&stakingtypes.MsgUndelegate{},
		&stakingtypes.MsgBeginRedelegate{},
		&slashingtypes.MsgUnjail{},
		&banktypes.MsgSend{},
	} {
		allowed, err := circuit.IsAllowed(after, sdk.MsgTypeURL(msg))
		require.NoError(t, err)
		require.True(t, allowed)
	}
}

// Contracts, interchain accounts, and authz dispatch through the message router,
// not the ante handler.
func TestStakingFreezeRouter(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{}).WithChainID("neutron-1").WithBlockHeight(61635576)

	delegate := &stakingtypes.MsgDelegate{
		DelegatorAddress: "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap",
		ValidatorAddress: "neutronvaloper1md0k6m8y58w8u98x82kjah7r5zcajw7c5v5ypa",
		Amount:           sdk.NewInt64Coin("untrn", 1),
	}
	handler := neutronApp.MsgServiceRouter().Handler(delegate)
	require.NotNil(t, handler)
	_, err := handler(ctx, delegate)
	require.ErrorIs(t, err, app.ErrStakingFrozen)

	_, err = handler(ctx.WithChainID("testing"), delegate)
	require.NotErrorIs(t, err, app.ErrStakingFrozen)
}

type freezeTx struct {
	msgs []sdk.Msg
}

func (m freezeTx) GetMsgs() []sdk.Msg                    { return m.msgs }
func (m freezeTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
