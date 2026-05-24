package params

import (
	"fmt"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/gagliardetto/solana-go"
)

// BondingCurveAccount represents the bonding curve state
type BondingCurveAccount struct {
	Discriminator        uint8
	Account              solana.PublicKey
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	RealSolReserves      uint64
	TokenTotalSupply     uint64
	Complete             bool
	Creator              solana.PublicKey
	IsMayhemMode         bool
	IsCashbackCoin       bool
}

// PumpFunParams represents PumpFun protocol specific parameters
type PumpFunParams struct {
	BondingCurve              *BondingCurveAccount
	AssociatedBondingCurve    solana.PublicKey
	CreatorVault              solana.PublicKey
	TokenProgram              solana.PublicKey
	CloseTokenAccountWhenSell *bool
	ObservedTradeCreator      solana.PublicKey
	FeeSharingCreatorVault    solana.PublicKey
	FeeRecipient              solana.PublicKey
	QuoteMint                 solana.PublicKey
}

// NewPumpFunParams creates new PumpFun params
func NewPumpFunParams(
	bondingCurve *BondingCurveAccount,
	associatedBondingCurve solana.PublicKey,
	creatorVault solana.PublicKey,
	tokenProgram solana.PublicKey,
) *PumpFunParams {
	return &PumpFunParams{
		BondingCurve:           bondingCurve,
		AssociatedBondingCurve: associatedBondingCurve,
		CreatorVault:           creatorVault,
		TokenProgram:           tokenProgram,
	}
}

// WithCloseTokenAccount sets the close token account flag
func (p *PumpFunParams) WithCloseTokenAccount(close bool) *PumpFunParams {
	p.CloseTokenAccountWhenSell = &close
	return p
}

// WithCreatorVault overrides the creator vault
func (p *PumpFunParams) WithCreatorVault(vault solana.PublicKey) *PumpFunParams {
	p.CreatorVault = vault
	return p
}

// WithQuoteMint sets the PumpFun quote mint. A non-zero quote mint selects V2 instructions.
func (p *PumpFunParams) WithQuoteMint(quoteMint solana.PublicKey) *PumpFunParams {
	p.QuoteMint = quoteMint
	return p
}

// WithFeeRecipient sets the observed PumpFun fee recipient from parser/grpc events.
func (p *PumpFunParams) WithFeeRecipient(feeRecipient solana.PublicKey) *PumpFunParams {
	p.FeeRecipient = feeRecipient
	return p
}

// NewPumpFunParamsFromTrade builds PumpFun params from sol-parser-sdk trade event fields.
// Pass VirtualQuoteReserves/RealQuoteReserves from the parser event for quote-aware slippage.
func NewPumpFunParamsFromTrade(
	bondingCurve, associatedBondingCurve, mint, quoteMint,
	creator, creatorVault solana.PublicKey,
	virtualTokenReserves, virtualQuoteReserves,
	realTokenReserves, realQuoteReserves uint64,
	closeTokenAccountWhenSell *bool,
	feeRecipient, tokenProgram solana.PublicKey,
	isCashbackCoin bool,
	mayhemMode bool,
) *PumpFunParams {
	return &PumpFunParams{
		BondingCurve: &BondingCurveAccount{
			Account:              bondingCurve,
			VirtualTokenReserves: virtualTokenReserves,
			VirtualSolReserves:   virtualQuoteReserves,
			RealTokenReserves:    realTokenReserves,
			RealSolReserves:      realQuoteReserves,
			Creator:              creator,
			IsMayhemMode:         mayhemMode,
			IsCashbackCoin:       isCashbackCoin,
		},
		AssociatedBondingCurve:    associatedBondingCurve,
		CreatorVault:              creatorVault,
		TokenProgram:              tokenProgram,
		CloseTokenAccountWhenSell: closeTokenAccountWhenSell,
		ObservedTradeCreator:      creator,
		FeeRecipient:              feeRecipient,
		QuoteMint:                 quoteMint,
	}
}

type PumpFunParserTradeEvent struct {
	BondingCurve            string
	AssociatedBondingCurve  string
	Mint                    string
	QuoteMint               string
	Creator                 string
	CreatorVault            string
	VirtualTokenReserves    uint64
	VirtualSolReserves      uint64
	VirtualQuoteReserves    uint64
	RealTokenReserves       uint64
	RealSolReserves         uint64
	RealQuoteReserves       uint64
	CloseTokenAccountOnSell *bool
	FeeRecipient            string
	TokenProgram            string
	IsCashbackCoin          bool
	MayhemMode              bool
}

func NewPumpFunParamsFromParserTrade(event PumpFunParserTradeEvent) (*PumpFunParams, error) {
	bondingCurve, err := parserPubkey(event.BondingCurve)
	if err != nil {
		return nil, fmt.Errorf("bonding_curve: %w", err)
	}
	associatedBondingCurve, err := parserPubkey(event.AssociatedBondingCurve)
	if err != nil {
		return nil, fmt.Errorf("associated_bonding_curve: %w", err)
	}
	mint, err := parserPubkey(event.Mint)
	if err != nil {
		return nil, fmt.Errorf("mint: %w", err)
	}
	quoteMint, err := parserPubkey(event.QuoteMint)
	if err != nil {
		return nil, fmt.Errorf("quote_mint: %w", err)
	}
	creator, err := parserPubkey(event.Creator)
	if err != nil {
		return nil, fmt.Errorf("creator: %w", err)
	}
	creatorVault, err := parserPubkey(event.CreatorVault)
	if err != nil {
		return nil, fmt.Errorf("creator_vault: %w", err)
	}
	feeRecipient, err := parserPubkey(event.FeeRecipient)
	if err != nil {
		return nil, fmt.Errorf("fee_recipient: %w", err)
	}
	tokenProgram, err := parserPubkey(event.TokenProgram)
	if err != nil {
		return nil, fmt.Errorf("token_program: %w", err)
	}
	virtualQuoteReserves := event.VirtualQuoteReserves
	if quoteMint.IsZero() {
		virtualQuoteReserves = event.VirtualSolReserves
	}
	realQuoteReserves := event.RealQuoteReserves
	if quoteMint.IsZero() {
		realQuoteReserves = event.RealSolReserves
	}
	return NewPumpFunParamsFromTrade(
		bondingCurve,
		associatedBondingCurve,
		mint,
		quoteMint,
		creator,
		creatorVault,
		event.VirtualTokenReserves,
		virtualQuoteReserves,
		event.RealTokenReserves,
		realQuoteReserves,
		event.CloseTokenAccountOnSell,
		feeRecipient,
		tokenProgram,
		event.IsCashbackCoin,
		event.MayhemMode,
	), nil
}

// PumpSwapParams represents PumpSwap protocol specific parameters
type PumpSwapParams struct {
	Pool                   solana.PublicKey
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	PoolBaseTokenAccount   solana.PublicKey
	PoolQuoteTokenAccount  solana.PublicKey
	PoolBaseTokenReserves  uint64
	PoolQuoteTokenReserves uint64
	CoinCreatorVaultATA    solana.PublicKey
	CoinCreatorVaultAuth   solana.PublicKey
	BaseTokenProgram       solana.PublicKey
	QuoteTokenProgram      solana.PublicKey
	IsMayhemMode           bool
	IsCashbackCoin         bool
}

// NewPumpSwapParams creates new PumpSwap params
func NewPumpSwapParams(
	pool, baseMint, quoteMint,
	poolBaseTokenAccount, poolQuoteTokenAccount solana.PublicKey,
	poolBaseTokenReserves, poolQuoteTokenReserves uint64,
	coinCreatorVaultATA, coinCreatorVaultAuth solana.PublicKey,
	baseTokenProgram, quoteTokenProgram solana.PublicKey,
	isMayhemMode, isCashbackCoin bool,
) *PumpSwapParams {
	return &PumpSwapParams{
		Pool:                   pool,
		BaseMint:               baseMint,
		QuoteMint:              quoteMint,
		PoolBaseTokenAccount:   poolBaseTokenAccount,
		PoolQuoteTokenAccount:  poolQuoteTokenAccount,
		PoolBaseTokenReserves:  poolBaseTokenReserves,
		PoolQuoteTokenReserves: poolQuoteTokenReserves,
		CoinCreatorVaultATA:    coinCreatorVaultATA,
		CoinCreatorVaultAuth:   coinCreatorVaultAuth,
		BaseTokenProgram:       baseTokenProgram,
		QuoteTokenProgram:      quoteTokenProgram,
		IsMayhemMode:           isMayhemMode,
		IsCashbackCoin:         isCashbackCoin,
	}
}

type PumpSwapParserTradeEvent struct {
	Pool                      string
	BaseMint                  string
	QuoteMint                 string
	PoolBaseTokenAccount      string
	PoolQuoteTokenAccount     string
	PoolBaseTokenReserves     uint64
	PoolQuoteTokenReserves    uint64
	CoinCreatorVaultATA       string
	CoinCreatorVaultAuthority string
	BaseTokenProgram          string
	QuoteTokenProgram         string
	IsMayhemMode              bool
	IsCashbackCoin            bool
}

func NewPumpSwapParamsFromParserTrade(event PumpSwapParserTradeEvent) (*PumpSwapParams, error) {
	pool, err := parserPubkey(event.Pool)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}
	baseMint, err := parserPubkey(event.BaseMint)
	if err != nil {
		return nil, fmt.Errorf("base_mint: %w", err)
	}
	quoteMint, err := parserPubkey(event.QuoteMint)
	if err != nil {
		return nil, fmt.Errorf("quote_mint: %w", err)
	}
	poolBaseTokenAccount, err := parserPubkey(event.PoolBaseTokenAccount)
	if err != nil {
		return nil, fmt.Errorf("pool_base_token_account: %w", err)
	}
	poolQuoteTokenAccount, err := parserPubkey(event.PoolQuoteTokenAccount)
	if err != nil {
		return nil, fmt.Errorf("pool_quote_token_account: %w", err)
	}
	coinCreatorVaultATA, err := parserPubkey(event.CoinCreatorVaultATA)
	if err != nil {
		return nil, fmt.Errorf("coin_creator_vault_ata: %w", err)
	}
	coinCreatorVaultAuthority, err := parserPubkey(event.CoinCreatorVaultAuthority)
	if err != nil {
		return nil, fmt.Errorf("coin_creator_vault_authority: %w", err)
	}
	baseTokenProgram, err := parserPubkey(event.BaseTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("base_token_program: %w", err)
	}
	quoteTokenProgram, err := parserPubkey(event.QuoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("quote_token_program: %w", err)
	}
	return NewPumpSwapParams(
		pool,
		baseMint,
		quoteMint,
		poolBaseTokenAccount,
		poolQuoteTokenAccount,
		event.PoolBaseTokenReserves,
		event.PoolQuoteTokenReserves,
		coinCreatorVaultATA,
		coinCreatorVaultAuthority,
		baseTokenProgram,
		quoteTokenProgram,
		event.IsMayhemMode,
		event.IsCashbackCoin,
	), nil
}

func parserPubkey(value string) (solana.PublicKey, error) {
	if value == "" || value == constants.SYSTEM_PROGRAM.String() {
		return solana.PublicKey{}, nil
	}
	return solana.PublicKeyFromBase58(value)
}

// BonkParams represents Bonk protocol specific parameters
type BonkParams struct {
	VirtualBase               uint128
	VirtualQuote              uint128
	RealBase                  uint128
	RealQuote                 uint128
	PoolState                 solana.PublicKey
	BaseVault                 solana.PublicKey
	QuoteVault                solana.PublicKey
	MintTokenProgram          solana.PublicKey
	PlatformConfig            solana.PublicKey
	PlatformAssociatedAccount solana.PublicKey
	CreatorAssociatedAccount  solana.PublicKey
	GlobalConfig              solana.PublicKey
}

// uint128 represents a 128-bit unsigned integer
type uint128 struct {
	Hi uint64
	Lo uint64
}

// RaydiumCpmmParams represents Raydium CPMM protocol specific parameters
type RaydiumCpmmParams struct {
	PoolState         solana.PublicKey
	AmmConfig         solana.PublicKey
	BaseMint          solana.PublicKey
	QuoteMint         solana.PublicKey
	BaseReserve       uint64
	QuoteReserve      uint64
	BaseVault         solana.PublicKey
	QuoteVault        solana.PublicKey
	BaseTokenProgram  solana.PublicKey
	QuoteTokenProgram solana.PublicKey
	ObservationState  solana.PublicKey
}

// NewRaydiumCpmmParams creates new Raydium CPMM params
func NewRaydiumCpmmParams(
	poolState, ammConfig, baseMint, quoteMint,
	baseVault, quoteVault solana.PublicKey,
	baseReserve, quoteReserve uint64,
	baseTokenProgram, quoteTokenProgram, observationState solana.PublicKey,
) *RaydiumCpmmParams {
	return &RaydiumCpmmParams{
		PoolState:         poolState,
		AmmConfig:         ammConfig,
		BaseMint:          baseMint,
		QuoteMint:         quoteMint,
		BaseReserve:       baseReserve,
		QuoteReserve:      quoteReserve,
		BaseVault:         baseVault,
		QuoteVault:        quoteVault,
		BaseTokenProgram:  baseTokenProgram,
		QuoteTokenProgram: quoteTokenProgram,
		ObservationState:  observationState,
	}
}

// RaydiumAmmV4Params represents Raydium AMM V4 protocol specific parameters
type RaydiumAmmV4Params struct {
	Amm                   solana.PublicKey
	AmmOpenOrders         solana.PublicKey
	AmmTargetOrders       solana.PublicKey
	TokenCoin             solana.PublicKey
	TokenPc               solana.PublicKey
	SerumProgram          solana.PublicKey
	SerumMarket           solana.PublicKey
	SerumBids             solana.PublicKey
	SerumAsks             solana.PublicKey
	SerumEventQueue       solana.PublicKey
	SerumCoinVaultAccount solana.PublicKey
	SerumPcVaultAccount   solana.PublicKey
	SerumVaultSigner      solana.PublicKey
	CoinMint              solana.PublicKey
	PcMint                solana.PublicKey
	CoinReserve           uint64
	PcReserve             uint64
}

// NewRaydiumAmmV4Params creates new Raydium AMM V4 params
func NewRaydiumAmmV4Params(
	amm, coinMint, pcMint, tokenCoin, tokenPc solana.PublicKey,
	ammOpenOrders, ammTargetOrders, serumProgram, serumMarket solana.PublicKey,
	serumBids, serumAsks, serumEventQueue solana.PublicKey,
	serumCoinVaultAccount, serumPcVaultAccount, serumVaultSigner solana.PublicKey,
	coinReserve, pcReserve uint64,
) *RaydiumAmmV4Params {
	return &RaydiumAmmV4Params{
		Amm:                   amm,
		CoinMint:              coinMint,
		PcMint:                pcMint,
		TokenCoin:             tokenCoin,
		TokenPc:               tokenPc,
		AmmOpenOrders:         ammOpenOrders,
		AmmTargetOrders:       ammTargetOrders,
		SerumProgram:          serumProgram,
		SerumMarket:           serumMarket,
		SerumBids:             serumBids,
		SerumAsks:             serumAsks,
		SerumEventQueue:       serumEventQueue,
		SerumCoinVaultAccount: serumCoinVaultAccount,
		SerumPcVaultAccount:   serumPcVaultAccount,
		SerumVaultSigner:      serumVaultSigner,
		CoinReserve:           coinReserve,
		PcReserve:             pcReserve,
	}
}

// MeteoraDammV2Params represents Meteora DAMM V2 protocol specific parameters
type MeteoraDammV2Params struct {
	Pool          solana.PublicKey
	TokenAVault   solana.PublicKey
	TokenBVault   solana.PublicKey
	TokenAMint    solana.PublicKey
	TokenBMint    solana.PublicKey
	TokenAProgram solana.PublicKey
	TokenBProgram solana.PublicKey
	TokenAReserve uint64
	TokenBReserve uint64
}

// NewMeteoraDammV2Params creates new Meteora DAMM V2 params
func NewMeteoraDammV2Params(
	pool, tokenAVault, tokenBVault, tokenAMint, tokenBMint,
	tokenAProgram, tokenBProgram solana.PublicKey,
) *MeteoraDammV2Params {
	return &MeteoraDammV2Params{
		Pool:          pool,
		TokenAVault:   tokenAVault,
		TokenBVault:   tokenBVault,
		TokenAMint:    tokenAMint,
		TokenBMint:    tokenBMint,
		TokenAProgram: tokenAProgram,
		TokenBProgram: tokenBProgram,
	}
}
