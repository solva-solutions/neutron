package app_test

import (
	"testing"
	"time"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/solva-solutions/neutron/v11/app"
	"github.com/solva-solutions/neutron/v11/testutil"
)

func TestGovRejectsNonSoftwareUpgradeProposals(t *testing.T) {
	neutronApp := testutil.Setup(t).(*app.App)
	ctx := neutronApp.NewUncachedContext(false, cmtproto.Header{Time: time.Now().UTC()})

	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	proposer := mustAccAddress(t, "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605")
	recipient := mustAccAddress(t, "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap")

	upgrade := &upgradetypes.MsgSoftwareUpgrade{
		Authority: govAddr,
		Plan:      upgradetypes.Plan{Name: "v11.5.0", Height: 10},
	}
	cancel := &upgradetypes.MsgCancelUpgrade{Authority: govAddr}

	_, err := neutronApp.GovKeeper.SubmitProposal(ctx, []sdk.Msg{upgrade}, "", "upgrade", "schedule an upgrade", proposer, false)
	require.NoError(t, err)

	_, err = neutronApp.GovKeeper.SubmitProposal(ctx, []sdk.Msg{cancel}, "", "cancel", "cancel an upgrade", proposer, false)
	require.NoError(t, err)

	send := &banktypes.MsgSend{
		FromAddress: govAddr,
		ToAddress:   recipient.String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("untrn", 1)),
	}
	// MsgUpdateAdmin is the message governance proposal 9 used.
	updateAdmin := &wasmtypes.MsgUpdateAdmin{
		Sender:   govAddr,
		NewAdmin: proposer.String(),
		Contract: "neutron1nfns3ck2ykrs0fknckrzd9728cyf77devuzernhwcwrdxw7ssk2s3tjf8r",
	}

	for _, msgs := range [][]sdk.Msg{
		{send},
		{updateAdmin},
		{upgrade, send},
	} {
		_, err := neutronApp.GovKeeper.SubmitProposal(ctx, msgs, "", "denied", "not a software upgrade", proposer, false)
		require.ErrorIs(t, err, govtypes.ErrUnroutableProposalMsg)
	}
}
