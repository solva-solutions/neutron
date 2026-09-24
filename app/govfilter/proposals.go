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

// ErrOnlySoftwareUpgradeProposals is returned when a governance proposal is not a
// software upgrade or a text proposal.
var ErrOnlySoftwareUpgradeProposals = errorsmod.Register(
	"neutron-gov",
	1,
	"only software upgrade and text proposals are allowed",
)

// IsSoftwareUpgradeMsg reports whether msg is MsgSoftwareUpgrade.
// MsgCancelUpgrade is not allowed.
func IsSoftwareUpgradeMsg(msg sdk.Msg) bool {
	_, ok := msg.(*upgradetypes.MsgSoftwareUpgrade)
	return ok
}

// IsSoftwareUpgradeTypeURL reports whether typeURL is MsgSoftwareUpgrade.
func IsSoftwareUpgradeTypeURL(typeURL string) bool {
	return typeURL == sdk.MsgTypeURL(&upgradetypes.MsgSoftwareUpgrade{})
}

// isTextProposalMsg reports whether msg is a legacy text proposal wrapped in
// MsgExecLegacyContent. An empty message list is also a text proposal.
func isTextProposalMsg(msg sdk.Msg) bool {
	legacy, ok := msg.(*govv1.MsgExecLegacyContent)
	if !ok || legacy.Content == nil {
		return false
	}
	if content, err := govv1.LegacyContentFromMessage(legacy); err == nil {
		_, ok = content.(*govv1beta1.TextProposal)
		return ok
	}
	return legacy.Content.TypeUrl == sdk.MsgTypeURL(&govv1beta1.TextProposal{})
}

// allowedByRouter reports whether governance may route and execute msg.
func allowedByRouter(msg sdk.Msg) bool {
	return IsSoftwareUpgradeMsg(msg) || isTextProposalMsg(msg)
}

// ValidateProposalMessages allows a software-upgrade proposal or a text proposal.
// A text proposal has no messages, or only legacy text messages. Mixing the two,
// and every other message including MsgCancelUpgrade, is rejected.
func ValidateProposalMessages(msgs []sdk.Msg) error {
	if len(msgs) == 0 {
		return nil
	}
	upgrades, texts := true, true
	for _, msg := range msgs {
		if !IsSoftwareUpgradeMsg(msg) {
			upgrades = false
		}
		if !isTextProposalMsg(msg) {
			texts = false
		}
		if !upgrades && !texts {
			return ErrOnlySoftwareUpgradeProposals.Wrapf("message type %s is not allowed", sdk.MsgTypeURL(msg))
		}
	}
	return nil
}

// ValidateTxMessages rejects transactions that submit a governance proposal which
// is not a software upgrade or a text proposal. Nested authz executions are checked as well.
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

// Active reports whether the proposal restriction applies in ctx. When it returns
// false, every filter in this package behaves as if it were not installed and
// consumes no gas, so blocks from before the restriction replay unchanged.
type Active func(ctx sdk.Context) bool

// ProposalFilterDecorator rejects proposal transactions that are not software upgrades.
type ProposalFilterDecorator struct {
	active Active
}

// NewProposalFilterDecorator returns an ante decorator that enforces the software-upgrade proposal restriction.
func NewProposalFilterDecorator(active Active) ProposalFilterDecorator {
	return ProposalFilterDecorator{active: active}
}

// AnteHandle implements sdk.AnteDecorator.
func (d ProposalFilterDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if d.active(ctx) {
		if err := ValidateTxMessages(tx.GetMsgs()); err != nil {
			return ctx, err
		}
	}
	return next(ctx, tx, simulate)
}

// softwareUpgradeRouter refuses to execute every message except software-upgrade
// and legacy text-proposal messages while the restriction is active.
type softwareUpgradeRouter struct {
	inner  baseapp.MessageRouter
	active Active
}

// legacyTextRouter is the v1beta1 router used by MsgExecLegacyContent. While the
// restriction is active its routes accept text proposals and execute nothing.
// Before that it panics like the nil legacy router v11.3.0 ran with.
type legacyTextRouter struct {
	active Active
}

// NewLegacyTextRouter returns the v1beta1 router used by MsgExecLegacyContent.
func NewLegacyTextRouter(active Active) govv1beta1.Router {
	return legacyTextRouter{active: active}
}

func (legacyTextRouter) AddRoute(string, govv1beta1.Handler) govv1beta1.Router {
	panic("legacy text router does not accept routes")
}

// HasRoute reports true for every route, so the handler decides with the context.
func (legacyTextRouter) HasRoute(string) bool { return true }

func (r legacyTextRouter) GetRoute(string) govv1beta1.Handler {
	return func(ctx sdk.Context, content govv1beta1.Content) error {
		if !r.active(ctx) {
			panic("governance has no legacy proposal router")
		}
		if _, ok := content.(*govv1beta1.TextProposal); !ok {
			return ErrOnlySoftwareUpgradeProposals.Wrapf("legacy content %T is not a text proposal", content)
		}
		return nil
	}
}

func (legacyTextRouter) Seal() {}

// NewSoftwareUpgradeRouter wraps inner so governance can only execute software
// upgrades and legacy text proposals while active reports true. The router has
// no context when a handler is looked up, so submissions are refused by
// ProposalHooks and execution is refused by the returned handler.
func NewSoftwareUpgradeRouter(inner baseapp.MessageRouter, active Active) baseapp.MessageRouter {
	return softwareUpgradeRouter{inner: inner, active: active}
}

func (r softwareUpgradeRouter) Handler(msg sdk.Msg) baseapp.MsgServiceHandler {
	return r.guard(r.inner.Handler(msg))
}

func (r softwareUpgradeRouter) HandlerByTypeURL(typeURL string) baseapp.MsgServiceHandler {
	return r.guard(r.inner.HandlerByTypeURL(typeURL))
}

func (r softwareUpgradeRouter) guard(handler baseapp.MsgServiceHandler) baseapp.MsgServiceHandler {
	if handler == nil {
		return nil
	}
	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		if r.active(ctx) && !allowedByRouter(msg) {
			return nil, ErrOnlySoftwareUpgradeProposals.Wrapf("governance cannot execute %s", sdk.MsgTypeURL(msg))
		}
		return handler(ctx, msg)
	}
}

// ProposalHooks rejects newly submitted proposals that are not software upgrades
// or text proposals. This covers submission paths that do not pass through the
// ante handler, such as contracts and authz.
type ProposalHooks struct {
	keeper *govkeeper.Keeper
	active Active
}

// NewProposalHooks returns governance hooks bound to keeper.
func NewProposalHooks(k *govkeeper.Keeper, active Active) ProposalHooks {
	return ProposalHooks{keeper: k, active: active}
}

// AfterProposalSubmission implements govtypes.GovHooks.
func (h ProposalHooks) AfterProposalSubmission(ctx context.Context, proposalID uint64) error {
	if !h.active(sdk.UnwrapSDKContext(ctx)) {
		return nil
	}
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
