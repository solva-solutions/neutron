package app

import (
	"fmt"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

const (
	// clawbackRecipient receives the seized funds.
	clawbackRecipient = "neutron1yr29fd7uzdjp2jsq8hrta8mvyd6ex7vumn0shy"
)

var clawbackTransfers = []struct {
	from string
	coin sdk.Coin
}{
	{attackerAddress, mustCoin("ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81", "1671301712957")},
	{attackerAddress, mustCoin("factory/neutron1k6hr0f83e7un2wjf29cspk7j69jrnskk65k3ek2nj9dztrlzpj6q00rtsa/udatom", "1592481671557")},
	{attackerAddress, mustCoin("untrn", "95196289988992")},
	{attackerAddress, mustCoin("factory/neutron1ffus553eet978k024lmssw0czsxwr97mggyv85lpcsdkft8v9ufsz3sa07/astro", "370318691388430")},
	{attackerAddress, mustCoin("ibc/A585C2D15DCD3B010849B453A2CFCB5E213208A5AB665691792684C26274304D", "14088467067160856016")},
	{attackerAddress, mustCoin("factory/neutron1ug740qrkquxzrk2hh29qrlx3sktkfml3je7juusc2te7xmvsscns0n2wry/wstETH", "293993098793")},
	{attackerAddress, mustCoin("factory/neutron10sr06r3qkhn7xzpw3339wuj77hu06mzna6uht0/eclip", "429157300")},
	{attackerAddress, mustCoin("ibc/F082B65C88E4B6D5EF1DB243CDA1D331D002759E938A0F5CD3FFDC5D53B3E349", "7141")},
	{attackerAddress, mustCoin("ibc/773B4D0A3CD667B2275D5A4A7A2F0909C0BA0F4059C0B9181E680DDF4965DCC7", "2100")},
	{attackerAddress, mustCoin("factory/neutron18c8qejysp4hgcfuxdpj4wf29mevzwllz5yh8uayjxamwtrs0n9fshq9vtv/astroport/share", "1000")},
	{attackerAddress, mustCoin("factory/neutron1nfns3ck2ykrs0fknckrzd9728cyf77devuzernhwcwrdxw7ssk2s3tjf8r/astroport/share", "1000")},
	{attackerAddress, mustCoin("factory/neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98/astroport/share", "1000")},
	{attackerAddress, mustCoin("factory/neutron1zlf3hutsa4qnmue53lz2tfxrutp8y2e3rj4nkghg3rupgl4mqy8s5jgxsn/xASTRO", "1000")},
	// attackerAddress2: the POSTHUMAN delegation is made liquid by the unstake
	// that runs before this clawback, then seized with the liquid balance and rewards.
	{attackerAddress2, mustCoin("untrn", "31620400000000")},
	{attackerAddress2, mustCoin("untrn", "7501863566")},
	{attackerAddress2, mustCoin("untrn", "19385160546")},
}

// ClawbackStolenFunds moves the stolen amounts to clawbackRecipient.
// Each denom is seized only when the spendable balance covers that amount, so the
// attacker's own NTRN stays put.
// Balances are updated directly so token-factory before-send hooks cannot block the seizure
// or run during BeginBlock.
func ClawbackStolenFunds(ctx sdk.Context, bank bankkeeper.BaseKeeper, ak keeper.AccountKeeper) error {
	to, err := sdk.AccAddressFromBech32(clawbackRecipient)
	if err != nil {
		return fmt.Errorf("invalid clawback recipient: %w", err)
	}
	if !ak.HasAccount(ctx, to) {
		ak.SetAccount(ctx, ak.NewAccountWithAddress(ctx, to))
	}

	for _, transfer := range clawbackTransfers {
		from, err := sdk.AccAddressFromBech32(transfer.from)
		if err != nil {
			return fmt.Errorf("invalid clawback source %s: %w", transfer.from, err)
		}
		if err := clawCoin(ctx, bank, from, to, transfer.coin); err != nil {
			return err
		}
	}
	return nil
}

func clawCoin(ctx sdk.Context, bank bankkeeper.BaseKeeper, from, to sdk.AccAddress, coin sdk.Coin) error {
	spendable := bank.SpendableCoin(ctx, from, coin.Denom)
	if spendable.Amount.LT(coin.Amount) {
		return nil
	}

	fromBalance := bank.GetBalance(ctx, from, coin.Denom)
	if err := setClawbackBalance(ctx, bank, from, sdk.NewCoin(coin.Denom, fromBalance.Amount.Sub(coin.Amount))); err != nil {
		return fmt.Errorf("debit %s from %s: %w", coin, from, err)
	}

	toBalance := bank.GetBalance(ctx, to, coin.Denom)
	if err := setClawbackBalance(ctx, bank, to, toBalance.Add(coin)); err != nil {
		return fmt.Errorf("credit %s to %s: %w", coin, to, err)
	}

	ctx.Logger().Info("clawed back stolen funds", "from", from.String(), "to", to.String(), "amount", coin.String())
	return nil
}

func setClawbackBalance(ctx sdk.Context, bank bankkeeper.BaseKeeper, addr sdk.AccAddress, balance sdk.Coin) error {
	key := collections.Join(addr, balance.Denom)
	if balance.IsZero() {
		return bank.Balances.Remove(ctx, key)
	}
	return bank.Balances.Set(ctx, key, balance.Amount)
}

func mustCoin(denom, amount string) sdk.Coin {
	amt, ok := sdkmath.NewIntFromString(amount)
	if !ok {
		panic("invalid clawback amount " + amount)
	}
	return sdk.NewCoin(denom, amt)
}
