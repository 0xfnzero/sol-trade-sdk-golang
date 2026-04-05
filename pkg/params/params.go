package params

import (
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

// PumpSwapParams represents PumpSwap protocol specific parameters
type PumpSwapParams struct {
	Pool                    solana.PublicKey
	BaseMint                solana.PublicKey
	QuoteMint               solana.PublicKey
	PoolBaseTokenAccount    solana.PublicKey
	PoolQuoteTokenAccount   solana.PublicKey
	PoolBaseTokenReserves   uint64
	PoolQuoteTokenReserves  uint64
	CoinCreatorVaultATA     solana.PublicKey
	CoinCreatorVaultAuth    solana.PublicKey
	BaseTokenProgram        solana.PublicKey
	QuoteTokenProgram       solana.PublicKey
	IsMayhemMode            bool
	IsCashbackCoin          bool
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
	Amm         solana.PublicKey
	CoinMint    solana.PublicKey
	PcMint      solana.PublicKey
	TokenCoin   solana.PublicKey
	TokenPc     solana.PublicKey
	CoinReserve uint64
	PcReserve   uint64
}

// NewRaydiumAmmV4Params creates new Raydium AMM V4 params
func NewRaydiumAmmV4Params(
	amm, coinMint, pcMint, tokenCoin, tokenPc solana.PublicKey,
	coinReserve, pcReserve uint64,
) *RaydiumAmmV4Params {
	return &RaydiumAmmV4Params{
		Amm:         amm,
		CoinMint:    coinMint,
		PcMint:      pcMint,
		TokenCoin:   tokenCoin,
		TokenPc:     tokenPc,
		CoinReserve: coinReserve,
		PcReserve:   pcReserve,
	}
}

// MeteoraDammV2Params represents Meteora DAMM V2 protocol specific parameters
type MeteoraDammV2Params struct {
	Pool           solana.PublicKey
	TokenAVault    solana.PublicKey
	TokenBVault    solana.PublicKey
	TokenAMint     solana.PublicKey
	TokenBMint     solana.PublicKey
	TokenAProgram  solana.PublicKey
	TokenBProgram  solana.PublicKey
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
