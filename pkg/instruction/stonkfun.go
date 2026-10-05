package instruction

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/gagliardetto/solana-go"
)

var stonkFunStandard = solana.MustPublicKeyFromBase58("4E876qZTE9FJMrBzgVtBrSrzz2TLivB5Y5QXPjB4gZL7")
var stonkFunReward = solana.MustPublicKeyFromBase58("6BwHHDg3u1854jC8PDLXvR4spTcLNaoBxLJNGC4nTESt")

func stonkConfig(k solana.PublicKey) bool { return k == stonkFunStandard || k == stonkFunReward }

type StonkFunCurveAccounts struct{ Pool, BaseMint, QuoteMint, BaseVault, QuoteVault, GlobalConfig, PlatformConfig, BaseTokenProgram, QuoteTokenProgram, PlatformAssociatedAccount, CreatorAssociatedAccount solana.PublicKey }

func stonkATA(payer, mint, program solana.PublicKey) (solana.PublicKey, error) {
	key, _, err := solana.FindProgramAddress([][]byte{payer[:], program[:], mint[:]}, solana.SPLAssociatedTokenAccountProgramID)
	return key, err
}

// BuildStonkFunCurveExactIn builds one direct stock/meme swap without any RPC.
// ATA creation, funding and SOL wrapping remain explicit caller operations.
func BuildStonkFunCurveExactIn(p StonkFunCurveAccounts, payer solana.PublicKey, amountIn, minimumAmountOut uint64, buy bool, shareFeeRate uint64) (solana.Instruction, error) {
	if !stonkConfig(p.PlatformConfig) {
		return nil, errors.New("unverified StonkFun platform config")
	}
	return BuildLaunchLabCurveExactIn(p, payer, amountIn, minimumAmountOut, buy, shareFeeRate)
}

// BuildLaunchLabCurveExactIn supports pools from any validated LaunchLab platform.
func BuildLaunchLabCurveExactIn(p StonkFunCurveAccounts, payer solana.PublicKey, amountIn, minimumAmountOut uint64, buy bool, shareFeeRate uint64) (solana.Instruction, error) {
	if amountIn == 0 {
		return nil, errors.New("amount cannot be zero")
	}
	if p.BaseMint == p.QuoteMint {
		return nil, errors.New("identical base and quote")
	}
	for _, key := range []solana.PublicKey{p.Pool, p.BaseMint, p.QuoteMint, p.BaseVault, p.QuoteVault, p.GlobalConfig, p.PlatformConfig, p.BaseTokenProgram, p.QuoteTokenProgram, p.PlatformAssociatedAccount, p.CreatorAssociatedAccount} {
		if key.IsZero() {
			return nil, errors.New("missing StonkFun account")
		}
	}
	userBase, err := stonkATA(payer, p.BaseMint, p.BaseTokenProgram)
	if err != nil {
		return nil, err
	}
	userQuote, err := stonkATA(payer, p.QuoteMint, p.QuoteTokenProgram)
	if err != nil {
		return nil, err
	}
	keys := []solana.PublicKey{payer, BONK_AUTHORITY, p.GlobalConfig, p.PlatformConfig, p.Pool, userBase, userQuote, p.BaseVault, p.QuoteVault, p.BaseMint, p.QuoteMint, p.BaseTokenProgram, p.QuoteTokenProgram, BONK_EVENT_AUTHORITY, BONK_PROGRAM, solana.SystemProgramID, p.PlatformAssociatedAccount, p.CreatorAssociatedAccount}
	metas := solana.AccountMetaSlice{}
	for i, key := range keys {
		w := i == 0 || (i >= 4 && i <= 8) || i >= 16
		metas = append(metas, &solana.AccountMeta{PublicKey: key, IsSigner: i == 0, IsWritable: w})
	}
	data := make([]byte, 32)
	disc := BonkBuyExactInDiscriminator
	if !buy {
		disc = BonkSellExactInDiscriminator
	}
	copy(data, disc)
	binary.LittleEndian.PutUint64(data[8:], amountIn)
	binary.LittleEndian.PutUint64(data[16:], minimumAmountOut)
	binary.LittleEndian.PutUint64(data[24:], shareFeeRate)
	return solana.NewInstruction(BONK_PROGRAM, metas, data), nil
}

type LaunchLabAccountBytes struct {
	Pubkey, Owner solana.PublicKey
	Data          []byte
}

// DecodeStonkFunCurve uses full subscription state and current epoch mint fees.
func DecodeStonkFunCurve(pool, global, platform LaunchLabAccountBytes, baseTokenProgram, quoteTokenProgram solana.PublicKey, baseTransferFee, quoteTransferFee calc.TokenTransferFee) (StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
	if !stonkConfig(platform.Pubkey) {
		return StonkFunCurveAccounts{}, calc.LaunchLabQuoteState{}, errors.New("LaunchLab config identity mismatch")
	}
	return DecodeLaunchLabCurve(pool, global, platform, baseTokenProgram, quoteTokenProgram, baseTransferFee, quoteTransferFee)
}

// DecodeLaunchLabCurve validates on-chain config relationships without platform attribution.
func DecodeLaunchLabCurve(pool, global, platform LaunchLabAccountBytes, baseTokenProgram, quoteTokenProgram solana.PublicKey, baseTransferFee, quoteTransferFee calc.TokenTransferFee) (StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
	fail := func(s string) (StonkFunCurveAccounts, calc.LaunchLabQuoteState, error) {
		return StonkFunCurveAccounts{}, calc.LaunchLabQuoteState{}, errors.New(s)
	}
	for _, a := range []LaunchLabAccountBytes{pool, global, platform} {
		if a.Owner != BONK_PROGRAM {
			return fail("unexpected LaunchLab account owner")
		}
	}
	d, g, p := pool.Data, global.Data, platform.Data
	if len(d) < 429 || !bytes.Equal(d[:8], []byte{247, 237, 227, 245, 215, 195, 222, 70}) || len(g) < 35 || !bytes.Equal(g[:8], []byte{149, 8, 156, 202, 160, 252, 176, 217}) || len(p) < 728 || !bytes.Equal(p[:8], []byte{160, 78, 128, 0, 248, 83, 230, 160}) {
		return fail("invalid LaunchLab state bytes")
	}
	key := func(o int) solana.PublicKey { var k solana.PublicKey; copy(k[:], d[o:o+32]); return k }
	number := func(b []byte, o int) uint64 { return binary.LittleEndian.Uint64(b[o:]) }
	if key(141) != global.Pubkey || key(173) != platform.Pubkey {
		return fail("LaunchLab config identity mismatch")
	}
	if d[17] != 0 {
		return fail("LaunchLab curve is not trading")
	}
	quote, creator := key(237), key(333)
	associated := func(k solana.PublicKey) (solana.PublicKey, error) {
		a, _, err := solana.FindProgramAddress([][]byte{k[:], quote[:]}, BONK_PROGRAM)
		return a, err
	}
	platformATA, err := associated(platform.Pubkey)
	if err != nil {
		return fail(err.Error())
	}
	creatorATA, err := associated(creator)
	if err != nil {
		return fail(err.Error())
	}
	accounts := StonkFunCurveAccounts{pool.Pubkey, key(205), quote, key(269), key(301), global.Pubkey, platform.Pubkey, baseTokenProgram, quoteTokenProgram, platformATA, creatorATA}
	state := calc.LaunchLabQuoteState{VirtualBase: number(d, 37), VirtualQuote: number(d, 45), RealBase: number(d, 53), RealQuote: number(d, 61), TotalBaseSell: number(d, 29), CurveType: g[16], TradeFeeRate: number(g, 27), PlatformFeeRate: number(p, 104), CreatorFeeRate: number(p, 720), BaseTransferFee: baseTransferFee, QuoteTransferFee: quoteTransferFee}
	return accounts, state, nil
}
