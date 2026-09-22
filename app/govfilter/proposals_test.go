package govfilter_test

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/solva-solutions/neutron/v11/app/govfilter"
)

func TestValidateProposalMessages(t *testing.T) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{Authority: "neutron1gov", Plan: upgradetypes.Plan{Name: "v11.5.0", Height: 10}}
	cancelMsg := &upgradetypes.MsgCancelUpgrade{Authority: "neutron1gov"}
	sendMsg := &banktypes.MsgSend{FromAddress: "neutron1gov", ToAddress: "neutron1user", Amount: sdk.NewCoins(sdk.NewInt64Coin("untrn", 1))}

	require.NoError(t, govfilter.ValidateProposalMessages([]sdk.Msg{upgradeMsg}))
	require.NoError(t, govfilter.ValidateProposalMessages([]sdk.Msg{cancelMsg, upgradeMsg}))
	require.ErrorIs(t, govfilter.ValidateProposalMessages(nil), govfilter.ErrOnlySoftwareUpgradeProposals)
	require.ErrorIs(t, govfilter.ValidateProposalMessages([]sdk.Msg{sendMsg}), govfilter.ErrOnlySoftwareUpgradeProposals)
	require.ErrorIs(t, govfilter.ValidateProposalMessages([]sdk.Msg{upgradeMsg, sendMsg}), govfilter.ErrOnlySoftwareUpgradeProposals)
}

func TestValidateTxMessages(t *testing.T) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{Authority: "neutron1gov", Plan: upgradetypes.Plan{Name: "v11.5.0", Height: 10}}
	sendMsg := &banktypes.MsgSend{FromAddress: "neutron1gov", ToAddress: "neutron1user", Amount: sdk.NewCoins(sdk.NewInt64Coin("untrn", 1))}

	allowed, err := govv1.NewMsgSubmitProposal([]sdk.Msg{upgradeMsg}, nil, "neutron1user", "", "upgrade", "schedule", false)
	require.NoError(t, err)
	require.NoError(t, govfilter.ValidateTxMessages([]sdk.Msg{allowed}))

	denied, err := govv1.NewMsgSubmitProposal([]sdk.Msg{sendMsg}, nil, "neutron1user", "", "send", "nope", false)
	require.NoError(t, err)
	require.ErrorIs(t, govfilter.ValidateTxMessages([]sdk.Msg{denied}), govfilter.ErrOnlySoftwareUpgradeProposals)

	legacy, err := govv1beta1.NewMsgSubmitProposal(govv1beta1.NewTextProposal("title", "desc"), nil, sdk.AccAddress("neutron1user"))
	require.NoError(t, err)
	require.ErrorIs(t, govfilter.ValidateTxMessages([]sdk.Msg{legacy}), govfilter.ErrOnlySoftwareUpgradeProposals)

	exec := authz.NewMsgExec(sdk.AccAddress("neutron1user"), []sdk.Msg{denied})
	require.ErrorIs(t, govfilter.ValidateTxMessages([]sdk.Msg{&exec}), govfilter.ErrOnlySoftwareUpgradeProposals)

	require.NoError(t, govfilter.ValidateTxMessages([]sdk.Msg{sendMsg}))
}

type mockTx struct {
	msgs []sdk.Msg
}

func (m mockTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m mockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func TestProposalFilterDecorator(t *testing.T) {
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{Authority: "neutron1gov", Plan: upgradetypes.Plan{Name: "v11.5.0", Height: 10}}
	allowed, err := govv1.NewMsgSubmitProposal([]sdk.Msg{upgradeMsg}, nil, "neutron1user", "", "upgrade", "schedule", false)
	require.NoError(t, err)

	called := false
	next := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		called = true
		return ctx, nil
	}

	decorator := govfilter.NewProposalFilterDecorator()
	_, err = decorator.AnteHandle(sdk.Context{}, mockTx{msgs: []sdk.Msg{allowed}}, false, next)
	require.NoError(t, err)
	require.True(t, called)

	sendMsg := &banktypes.MsgSend{FromAddress: "neutron1gov", ToAddress: "neutron1user", Amount: sdk.NewCoins(sdk.NewInt64Coin("untrn", 1))}
	denied, err := govv1.NewMsgSubmitProposal([]sdk.Msg{sendMsg}, nil, "neutron1user", "", "send", "nope", false)
	require.NoError(t, err)
	called = false
	_, err = decorator.AnteHandle(sdk.Context{}, mockTx{msgs: []sdk.Msg{denied}}, false, next)
	require.ErrorIs(t, err, govfilter.ErrOnlySoftwareUpgradeProposals)
	require.False(t, called)
}
