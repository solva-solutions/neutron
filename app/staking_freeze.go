package app

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ErrStakingFrozen is returned when a transaction would change stake or create a validator.
var ErrStakingFrozen = errorsmod.Register(
	"neutron-staking-freeze",
	1,
	"staking is frozen",
)

// StakingFreezeDecorator rejects messages that add new bonded stake on neutron-1
// after the halt.
type StakingFreezeDecorator struct{}

// NewStakingFreezeDecorator returns the ante decorator that freezes staking messages.
func NewStakingFreezeDecorator() StakingFreezeDecorator {
	return StakingFreezeDecorator{}
}

// AnteHandle implements sdk.AnteDecorator.
func (StakingFreezeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if Proposal9ProtectionsActive(ctx) {
		if err := rejectFrozenStakingMsgs(tx.GetMsgs()); err != nil {
			return ctx, err
		}
	}
	return next(ctx, tx, simulate)
}

func rejectFrozenStakingMsgs(msgs []sdk.Msg) error {
	for _, msg := range msgs {
		if isFrozenStakingMsg(msg) {
			return ErrStakingFrozen.Wrapf("%s", sdk.MsgTypeURL(msg))
		}
		switch m := msg.(type) {
		case *authz.MsgExec:
			inner, err := m.GetMessages()
			if err != nil {
				return fmt.Errorf("authz messages: %w", err)
			}
			if err := rejectFrozenStakingMsgs(inner); err != nil {
				return err
			}
		case *govv1.MsgSubmitProposal:
			inner, err := m.GetMsgs()
			if err != nil {
				return fmt.Errorf("proposal messages: %w", err)
			}
			if err := rejectFrozenStakingMsgs(inner); err != nil {
				return err
			}
		}
	}
	return nil
}

// frozenStakingMsgs are the message types rejected while staking is frozen: the
// ones that add new bonded stake, and with it governance voting power.
// Redelegating, undelegating, and unjailing stay allowed.
var frozenStakingMsgs = map[string]struct{}{
	sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}):                  {},
	sdk.MsgTypeURL(&stakingtypes.MsgCancelUnbondingDelegation{}): {},
	sdk.MsgTypeURL(&stakingtypes.MsgCreateValidator{}):           {},
}

func isFrozenStakingMsg(msg sdk.Msg) bool {
	_, frozen := frozenStakingMsgs[sdk.MsgTypeURL(msg)]
	return frozen
}

// StakingFreezeCircuit is the message-router circuit breaker for the staking
// freeze. The ante decorator only sees transaction messages; this also rejects
// frozen messages dispatched by contracts, interchain accounts, authz, and gov.
type StakingFreezeCircuit struct{}

// IsAllowed implements baseapp.CircuitBreaker.
func (StakingFreezeCircuit) IsAllowed(ctx context.Context, typeURL string) (bool, error) {
	if _, frozen := frozenStakingMsgs[typeURL]; !frozen {
		return true, nil
	}
	sdkCtx, ok := ctx.(sdk.Context)
	if !ok {
		sdkCtx, ok = ctx.Value(sdk.SdkContextKey).(sdk.Context)
	}
	if !ok {
		return false, ErrStakingFrozen.Wrapf("%s: no block context", typeURL)
	}
	if Proposal9ProtectionsActive(sdkCtx) {
		return false, ErrStakingFrozen.Wrapf("%s", typeURL)
	}
	return true, nil
}
