package app_test

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/solva-solutions/neutron/v11/app"
)

const (
	attackerBech32 = "neutron1dd25c4sshelrpfs0433apg24c5phrhk8l6n605"
	otherBech32    = "neutron1eeyfuy3xv2xf35aa3gctyaajvvtj2z7gkwdjap"
)

func TestLockedAccountDecorator(t *testing.T) {
	decorator, err := app.NewLockedAccountDecorator(signerCodecStub{})
	require.NoError(t, err)

	attacker := mustAccAddress(t, attackerBech32)
	other := mustAccAddress(t, otherBech32)
	send := &banktypes.MsgSend{
		FromAddress: otherBech32,
		ToAddress:   attackerBech32,
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("untrn", 1)),
	}

	neutronCtx := sdk.Context{}.WithChainID("neutron-1").WithBlockHeight(61635574)
	otherChain := sdk.Context{}.WithChainID("testing").WithBlockHeight(61635574)

	t.Run("replayed block before the halt", func(t *testing.T) {
		_, err := decorator.AnteHandle(neutronCtx.WithBlockHeight(61635573), lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{attacker}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.NoError(t, err)
	})

	t.Run("check tx after restart", func(t *testing.T) {
		_, err := decorator.AnteHandle(neutronCtx.WithBlockHeight(0).WithIsCheckTx(true), lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{attacker}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.ErrorIs(t, err, app.ErrAccountLocked)
	})

	t.Run("second locked account", func(t *testing.T) {
		second := mustAccAddress(t, "neutron1ekgfga6vv4zdrrjn3dux6f62fuzektfndgaehm")
		_, err := decorator.AnteHandle(neutronCtx, lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{second}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.ErrorIs(t, err, app.ErrAccountLocked)
	})

	t.Run("attacker signature", func(t *testing.T) {
		called := false
		_, err := decorator.AnteHandle(neutronCtx, lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{attacker}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			called = true
			return ctx, nil
		})
		require.ErrorIs(t, err, app.ErrAccountLocked)
		require.False(t, called)
	})

	t.Run("other account", func(t *testing.T) {
		called := false
		_, err := decorator.AnteHandle(neutronCtx, lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{other}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			called = true
			return ctx, nil
		})
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("other chain", func(t *testing.T) {
		_, err := decorator.AnteHandle(otherChain, lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{attacker}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.NoError(t, err)
	})

	t.Run("fee granter", func(t *testing.T) {
		_, err := decorator.AnteHandle(neutronCtx, lockTx{msgs: []sdk.Msg{send}, signers: [][]byte{other}, granter: attacker}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.ErrorIs(t, err, app.ErrAccountLocked)
	})

	t.Run("authz", func(t *testing.T) {
		inner := &banktypes.MsgSend{
			FromAddress: attackerBech32,
			ToAddress:   otherBech32,
			Amount:      sdk.NewCoins(sdk.NewInt64Coin("untrn", 1)),
		}
		exec := authz.NewMsgExec(other, []sdk.Msg{inner})
		_, err := decorator.AnteHandle(neutronCtx, lockTx{msgs: []sdk.Msg{&exec}, signers: [][]byte{other}}, false, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			return ctx, nil
		})
		require.ErrorIs(t, err, app.ErrAccountLocked)
	})
}

type signerCodecStub struct{}

func (signerCodecStub) GetMsgV1Signers(msg proto.Message) ([][]byte, protov2.Message, error) {
	switch m := msg.(type) {
	case *banktypes.MsgSend:
		addr, err := sdk.AccAddressFromBech32(m.FromAddress)
		return [][]byte{addr}, nil, err
	case *authz.MsgExec:
		addr, err := sdk.AccAddressFromBech32(m.Grantee)
		return [][]byte{addr}, nil, err
	default:
		return nil, nil, nil
	}
}

type lockTx struct {
	msgs    []sdk.Msg
	signers [][]byte
	granter []byte
}

func (m lockTx) GetMsgs() []sdk.Msg                        { return m.msgs }
func (m lockTx) GetMsgsV2() ([]protov2.Message, error)     { return nil, nil }
func (m lockTx) GetSigners() ([][]byte, error)             { return m.signers, nil }
func (m lockTx) GetPubKeys() ([]cryptotypes.PubKey, error) { return nil, nil }
func (m lockTx) GetSignaturesV2() ([]signing.SignatureV2, error) {
	return nil, nil
}
func (m lockTx) GetGas() uint64    { return 0 }
func (m lockTx) GetFee() sdk.Coins { return nil }
func (m lockTx) FeePayer() []byte {
	if len(m.signers) == 0 {
		return nil
	}
	return m.signers[0]
}
func (m lockTx) FeeGranter() []byte { return m.granter }
