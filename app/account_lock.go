package app

import (
	"bytes"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/gogoproto/proto"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
)

// ErrAccountLocked is returned when a transaction is signed, paid for, or
// authorized by a locked account.
var ErrAccountLocked = errorsmod.Register(
	"neutron-lock",
	1,
	"account is locked",
)

// lockedAccounts cannot sign, pay for, or authorize transactions on neutron-1.
var lockedAccounts = []string{
	attackerAddress,
	attackerAddress2,
}

// msgSignerCodec resolves the accounts that must sign a message.
type msgSignerCodec interface {
	GetMsgV1Signers(msg proto.Message) ([][]byte, protov2.Message, error)
}

// LockedAccountDecorator rejects transactions from lockedAccounts on neutron-1
// after the halt.
type LockedAccountDecorator struct {
	accounts []sdk.AccAddress
	cdc      msgSignerCodec
}

// NewLockedAccountDecorator returns an ante decorator that rejects transactions
// from lockedAccounts. cdc is used to see signers of nested authz messages.
func NewLockedAccountDecorator(cdc msgSignerCodec) (LockedAccountDecorator, error) {
	if cdc == nil {
		return LockedAccountDecorator{}, fmt.Errorf("message signer codec is required")
	}
	accounts := make([]sdk.AccAddress, len(lockedAccounts))
	for i, bech32 := range lockedAccounts {
		addr, err := sdk.AccAddressFromBech32(bech32)
		if err != nil {
			return LockedAccountDecorator{}, fmt.Errorf("invalid locked account %s: %w", bech32, err)
		}
		accounts[i] = addr
	}
	return LockedAccountDecorator{accounts: accounts, cdc: cdc}, nil
}

// AnteHandle implements sdk.AnteDecorator.
func (d LockedAccountDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if !Proposal9ProtectionsActive(ctx) {
		return next(ctx, tx, simulate)
	}
	if err := d.rejectLockedAccount(tx); err != nil {
		return ctx, err
	}
	return next(ctx, tx, simulate)
}

func (d LockedAccountDecorator) rejectLockedAccount(tx sdk.Tx) error {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid tx type")
	}
	signers, err := sigTx.GetSigners()
	if err != nil {
		return err
	}
	if locked, ok := d.lockedAccount(signers...); ok {
		return ErrAccountLocked.Wrapf("%s cannot sign transactions", locked)
	}

	if feeTx, ok := tx.(sdk.FeeTx); ok {
		if locked, ok := d.lockedAccount(feeTx.FeePayer(), feeTx.FeeGranter()); ok {
			return ErrAccountLocked.Wrapf("%s cannot pay transaction fees", locked)
		}
	}

	return d.rejectLockedMessageSigners(tx.GetMsgs())
}

func (d LockedAccountDecorator) rejectLockedMessageSigners(msgs []sdk.Msg) error {
	for _, msg := range msgs {
		signers, _, err := d.cdc.GetMsgV1Signers(msg)
		if err != nil {
			return fmt.Errorf("message signers: %w", err)
		}
		if locked, ok := d.lockedAccount(signers...); ok {
			return ErrAccountLocked.Wrapf("%s cannot authorize %s", locked, sdk.MsgTypeURL(msg))
		}
		exec, ok := msg.(*authz.MsgExec)
		if !ok {
			continue
		}
		inner, err := exec.GetMessages()
		if err != nil {
			return fmt.Errorf("authz messages: %w", err)
		}
		if err := d.rejectLockedMessageSigners(inner); err != nil {
			return err
		}
	}
	return nil
}

func (d LockedAccountDecorator) lockedAccount(addrs ...[]byte) (sdk.AccAddress, bool) {
	for _, addr := range addrs {
		for _, locked := range d.accounts {
			if bytes.Equal(addr, locked) {
				return locked, true
			}
		}
	}
	return nil, false
}
