package app

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ErrStakingFrozen is returned when a transaction would change stake or create a validator.
var ErrStakingFrozen = errorsmod.Register(
	"neutron-staking-freeze",
	1,
	"staking is frozen",
)

// StakingFreezeDecorator rejects delegation, redelegation, validator-creation,
// and unjail messages on neutron-1 after the halt.
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

// frozenStakingMsgs are the message types rejected while staking is frozen.
// MsgUndelegate stays allowed so holders can exit.
var frozenStakingMsgs = map[string]struct{}{
	sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}):                  {},
	sdk.MsgTypeURL(&stakingtypes.MsgBeginRedelegate{}):           {},
	sdk.MsgTypeURL(&stakingtypes.MsgCancelUnbondingDelegation{}): {},
	sdk.MsgTypeURL(&stakingtypes.MsgCreateValidator{}):           {},
	sdk.MsgTypeURL(&slashingtypes.MsgUnjail{}):                   {},
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

// FreezeValidatorUpdates drops Tendermint voting-power changes on neutron-1 after
// the proposal 9 recovery block. Every earlier block, including the recovery
// block with the POSTHUMAN unstake, keeps its updates, so the Tendermint set stays
// equal to the staking set as of the end of block 61635575. Staking has already
// written its power index; only this returned set reaches Tendermint.
func FreezeValidatorUpdates(ctx sdk.Context, updates []abci.ValidatorUpdate) []abci.ValidatorUpdate {
	if ctx.ChainID() == neutronChainID && ctx.BlockHeight() > proposal9RecoveryHeight {
		return nil
	}
	return updates
}
