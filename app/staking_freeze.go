package app

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"

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

// StakingFreezeDecorator rejects delegation and validator-creation messages on neutron-1.
type StakingFreezeDecorator struct{}

// NewStakingFreezeDecorator returns the ante decorator that freezes staking messages.
func NewStakingFreezeDecorator() StakingFreezeDecorator {
	return StakingFreezeDecorator{}
}

// AnteHandle implements sdk.AnteDecorator.
func (StakingFreezeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if ctx.ChainID() == neutronChainID {
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

func isFrozenStakingMsg(msg sdk.Msg) bool {
	switch msg.(type) {
	case *stakingtypes.MsgDelegate,
		*stakingtypes.MsgUndelegate,
		*stakingtypes.MsgBeginRedelegate,
		*stakingtypes.MsgCancelUnbondingDelegation,
		*stakingtypes.MsgCreateValidator:
		return true
	default:
		return false
	}
}

// FreezeValidatorUpdates drops Tendermint voting-power changes on neutron-1 except
// in the proposal 9 recovery block. That block applies the POSTHUMAN unstake.
// Staking has already written its power index; only this returned set reaches Tendermint.
func FreezeValidatorUpdates(ctx sdk.Context, updates []abci.ValidatorUpdate) []abci.ValidatorUpdate {
	if ctx.ChainID() == neutronChainID && ctx.BlockHeight() != proposal9RecoveryHeight {
		return nil
	}
	return updates
}
