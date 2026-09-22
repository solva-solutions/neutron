package govfilter

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// ErrOnlySoftwareUpgradeProposals is returned when a governance proposal contains
// anything other than x/upgrade software-upgrade messages.
var ErrOnlySoftwareUpgradeProposals = errorsmod.Register(
	"neutron-gov",
	1,
	"only software upgrade proposals are allowed",
)

// IsSoftwareUpgradeMsg reports whether msg is a software-upgrade governance message.
// MsgCancelUpgrade is included so governance can still clear a scheduled upgrade.
func IsSoftwareUpgradeMsg(msg sdk.Msg) bool {
	switch msg.(type) {
	case *upgradetypes.MsgSoftwareUpgrade, *upgradetypes.MsgCancelUpgrade:
		return true
	default:
		return false
	}
}

// IsSoftwareUpgradeTypeURL reports whether typeURL is a software-upgrade message type.
func IsSoftwareUpgradeTypeURL(typeURL string) bool {
	switch typeURL {
	case sdk.MsgTypeURL(&upgradetypes.MsgSoftwareUpgrade{}),
		sdk.MsgTypeURL(&upgradetypes.MsgCancelUpgrade{}):
		return true
	default:
		return false
	}
}

// ValidateProposalMessages requires every message to be a software-upgrade message
// and rejects empty proposals.
func ValidateProposalMessages(msgs []sdk.Msg) error {
	if len(msgs) == 0 {
		return ErrOnlySoftwareUpgradeProposals.Wrap("proposal has no messages")
	}
	for _, msg := range msgs {
		if !IsSoftwareUpgradeMsg(msg) {
			return ErrOnlySoftwareUpgradeProposals.Wrapf("message type %s is not allowed", sdk.MsgTypeURL(msg))
		}
	}
	return nil
}

// ValidateTxMessages rejects transactions that submit a governance proposal which
// is not a software upgrade. Nested authz executions are checked as well.
func ValidateTxMessages(msgs []sdk.Msg) error {
	for _, msg := range msgs {
		switch m := msg.(type) {
		case *govv1.MsgSubmitProposal:
			inner, err := m.GetMsgs()
			if err != nil {
				return ErrOnlySoftwareUpgradeProposals.Wrap(err.Error())
			}
			if err := ValidateProposalMessages(inner); err != nil {
				return err
			}
		case *govv1beta1.MsgSubmitProposal:
			return ErrOnlySoftwareUpgradeProposals.Wrap("legacy v1beta1 proposals are not allowed")
		case *authz.MsgExec:
			inner, err := m.GetMessages()
			if err != nil {
				return ErrOnlySoftwareUpgradeProposals.Wrap(err.Error())
			}
			if err := ValidateTxMessages(inner); err != nil {
				return err
			}
		}
	}
	return nil
}

// ProposalFilterDecorator rejects proposal transactions that are not software upgrades.
type ProposalFilterDecorator struct{}

// NewProposalFilterDecorator returns an ante decorator that enforces the software-upgrade proposal restriction.
func NewProposalFilterDecorator() ProposalFilterDecorator {
	return ProposalFilterDecorator{}
}

// AnteHandle implements sdk.AnteDecorator.
func (ProposalFilterDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if err := ValidateTxMessages(tx.GetMsgs()); err != nil {
		return ctx, err
	}
	return next(ctx, tx, simulate)
}

// softwareUpgradeRouter hides every message handler except software-upgrade messages
// from the governance keeper, so those messages cannot be submitted or executed
// through a proposal.
type softwareUpgradeRouter struct {
	inner baseapp.MessageRouter
}

// NewSoftwareUpgradeRouter wraps inner so governance can only route software-upgrade messages.
func NewSoftwareUpgradeRouter(inner baseapp.MessageRouter) baseapp.MessageRouter {
	return softwareUpgradeRouter{inner: inner}
}

func (r softwareUpgradeRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler {
	if !IsSoftwareUpgradeMsg(msg) {
		return nil
	}
	return r.inner.Handler(msg)
}

func (r softwareUpgradeRouter) HandlerByTypeURL(typeURL string) baseapp.MsgServiceHandler {
	if !IsSoftwareUpgradeTypeURL(typeURL) {
		return nil
	}
	return r.inner.HandlerByTypeURL(typeURL)
}

// ProposalHooks rejects newly submitted proposals that are not software upgrades.
// This covers submission paths that do not pass through the ante handler, such as
// contracts and authz.
type ProposalHooks struct {
	keeper *govkeeper.Keeper
}

// NewProposalHooks returns governance hooks bound to keeper.
func NewProposalHooks(k *govkeeper.Keeper) ProposalHooks {
	return ProposalHooks{keeper: k}
}

// AfterProposalSubmission implements govtypes.GovHooks.
func (h ProposalHooks) AfterProposalSubmission(ctx context.Context, proposalID uint64) error {
	proposal, err := h.keeper.Proposals.Get(ctx, proposalID)
	if err != nil {
		return err
	}
	msgs, err := proposal.GetMsgs()
	if err != nil {
		return ErrOnlySoftwareUpgradeProposals.Wrapf("failed to unpack proposal %d messages: %s", proposalID, err.Error())
	}
	return ValidateProposalMessages(msgs)
}

// AfterProposalDeposit implements govtypes.GovHooks.
func (ProposalHooks) AfterProposalDeposit(context.Context, uint64, sdk.AccAddress) error {
	return nil
}

// AfterProposalVote implements govtypes.GovHooks.
func (ProposalHooks) AfterProposalVote(context.Context, uint64, sdk.AccAddress) error {
	return nil
}

// AfterProposalFailedMinDeposit implements govtypes.GovHooks.
func (ProposalHooks) AfterProposalFailedMinDeposit(context.Context, uint64) error {
	return nil
}

// AfterProposalVotingPeriodEnded implements govtypes.GovHooks.
func (ProposalHooks) AfterProposalVotingPeriodEnded(context.Context, uint64) error {
	return nil
}

var _ govtypes.GovHooks = ProposalHooks{}
