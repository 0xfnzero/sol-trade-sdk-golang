package calc

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
)

// ===== Error Definitions =====

var (
	ErrOverflow       = errors.New("integer overflow")
	ErrUnderflow      = errors.New("integer underflow")
	ErrDivisionByZero = errors.New("division by zero")
	ErrInvalidInput   = errors.New("invalid input")
)

// ===== Validation Functions =====

// MaxSlippageBasisPoints is the maximum allowed slippage (99.99% = 9999 bps)
// This prevents the wrap amount from doubling when slippage is 100%
const MaxSlippageBasisPoints uint64 = 9999

// validateAmount checks if amount is valid
func validateAmount(amount uint64, name string) error {
	if amount < 0 {
		return ErrInvalidInput
	}
	return nil
}

// validateBasisPoints checks if basis points are within valid range (0-10000)
func validateBasisPoints(basisPoints uint64) error {
	if basisPoints > 10000 {
		return ErrInvalidInput
	}
	return nil
}

// checkMulOverflow checks if multiplication would overflow
func checkMulOverflow(a, b uint64) error {
	if a == 0 || b == 0 {
		return nil
	}
	hi, _ := bits.Mul64(a, b)
	if hi != 0 {
		return ErrOverflow
	}
	return nil
}

// checkAddOverflow checks if addition would overflow
func checkAddOverflow(a, b uint64) error {
	if a > math.MaxUint64-b {
		return ErrOverflow
	}
	return nil
}

// ===== Common Calculation Functions =====

// ComputeFee calculates transaction fee based on amount and fee basis points
// Includes overflow protection
func ComputeFee(amount uint64, feeBasisPoints uint64) (uint64, error) {
	if err := validateAmount(amount, "amount"); err != nil {
		return 0, err
	}
	if err := validateBasisPoints(feeBasisPoints); err != nil {
		return 0, err
	}
	if err := checkMulOverflow(amount, feeBasisPoints); err != nil {
		return 0, err
	}
	return CeilDiv(amount*feeBasisPoints, 10000)
}

// CeilDiv performs ceiling division with zero check
func CeilDiv(a, b uint64) (uint64, error) {
	if b == 0 {
		return 0, ErrDivisionByZero
	}
	if err := validateAmount(a, "dividend"); err != nil {
		return 0, err
	}
	if err := validateAmount(b, "divisor"); err != nil {
		return 0, err
	}
	if err := checkAddOverflow(a, b-1); err != nil {
		return 0, err
	}
	return (a + b - 1) / b, nil
}

// CalculateWithSlippageBuy calculates buy amount with slippage protection
// Includes overflow protection
//
// Note: Basis points are clamped to MaxSlippageBasisPoints (9999 = 99.99%)
// to prevent the amount from doubling when basisPoints = 10000.
func CalculateWithSlippageBuy(amount uint64, basisPoints uint64) (uint64, error) {
	if err := validateAmount(amount, "amount"); err != nil {
		return 0, err
	}

	// Clamp basis points to max 9999 (99.99%) to prevent amount doubling at 100%
	bps := basisPoints
	if bps > MaxSlippageBasisPoints {
		bps = MaxSlippageBasisPoints
	}

	if err := checkMulOverflow(amount, bps); err != nil {
		return 0, err
	}
	slippageAmount := (amount * bps) / 10000
	if err := checkAddOverflow(amount, slippageAmount); err != nil {
		return 0, err
	}
	return amount + slippageAmount, nil
}

// CalculateWithSlippageSell calculates sell amount with slippage protection.
// Includes underflow protection.
//
// 100% from Rust: src/utils/calc/common.rs calculate_with_slippage_sell
//
// Note: Returns 1 if amount <= basisPoints / 10000 to ensure minimum output.
func CalculateWithSlippageSell(amount uint64, basisPoints uint64) (uint64, error) {
	if err := validateAmount(amount, "amount"); err != nil {
		return 0, err
	}
	if err := validateBasisPoints(basisPoints); err != nil {
		return 0, err
	}

	// Rust: if amount <= basis_points / 10000 { 1 } else { ... }
	if amount <= basisPoints/10000 {
		return 1, nil
	}

	if err := checkMulOverflow(amount, basisPoints); err != nil {
		return 0, err
	}
	slippageAmount := (amount * basisPoints) / 10000
	return amount - slippageAmount, nil
}

// ===== PumpFun Calculations =====

// PumpFun Constants from Rust: src/instruction/utils/pumpfun.rs global_constants
const (
	PumpFunFeeBasisPoints      uint64 = 95 // Protocol fee (NOT 100!)
	PumpFunCreatorFee          uint64 = 30 // Creator fee (NOT 50!)
	PumpFunInitialVirtualToken        = 1073000000000000
	PumpFunInitialVirtualSol          = 30000000000
	PumpFunInitialRealToken           = 793100000000000 // Fixed: was 793000000000000
	PumpFunTokenTotalSupply           = 1000000000000000
)

// GetBuyTokenAmountFromSolAmount calculates tokens received for SOL input on PumpFun
func GetBuyTokenAmountFromSolAmount(
	virtualTokenReserves uint64,
	virtualSolReserves uint64,
	realTokenReserves uint64,
	hasCreator bool,
	amount uint64,
) uint64 {
	if amount == 0 || virtualTokenReserves == 0 {
		return 0
	}

	totalFeeBasisPoints := PumpFunFeeBasisPoints
	if hasCreator {
		totalFeeBasisPoints += PumpFunCreatorFee
	}

	inputAmount := (uint64(amount) * 10000) / (totalFeeBasisPoints + 10000)
	denominator := virtualSolReserves + inputAmount

	tokensReceived := (inputAmount * virtualTokenReserves) / denominator

	if tokensReceived > realTokenReserves {
		tokensReceived = realTokenReserves
	}

	// Minimum token protection
	if tokensReceived <= 100*1000000 {
		if amount > 10000000 { // > 0.01 SOL
			tokensReceived = 25547619 * 1000000
		} else {
			tokensReceived = 255476 * 1000000
		}
	}

	return tokensReceived
}

// GetSellSolAmountFromTokenAmount calculates SOL received for token input on PumpFun
func GetSellSolAmountFromTokenAmount(
	virtualTokenReserves uint64,
	virtualSolReserves uint64,
	hasCreator bool,
	amount uint64,
) uint64 {
	if amount == 0 || virtualTokenReserves == 0 {
		return 0
	}

	numerator := uint64(amount) * uint64(virtualSolReserves)
	denominator := uint64(virtualTokenReserves) + uint64(amount)

	solCost := numerator / denominator

	totalFeeBasisPoints := PumpFunFeeBasisPoints
	if hasCreator {
		totalFeeBasisPoints += PumpFunCreatorFee
	}

	fee, err := ComputeFee(solCost, totalFeeBasisPoints)
	if err != nil {
		return 0
	}

	if solCost < fee {
		return 0
	}
	return solCost - fee
}

// ===== PumpSwap Calculations =====

// PumpSwap Constants from Rust: src/instruction/utils/pumpswap.rs accounts
const (
	PumpSwapLPFeeBasisPoints          uint64 = 25 // 0.25% (was 20)
	PumpSwapProtocolFeeBasisPoints    uint64 = 5  // 0.05% (was 20)
	PumpSwapCoinCreatorFeeBasisPoints uint64 = 5  // 0.05% (was 10)
)

var (
	pumpSwapI128Min = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
	pumpSwapI128Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
)

type PumpSwapFeeBasisPoints struct {
	LPFeeBasisPoints          uint64
	ProtocolFeeBasisPoints    uint64
	CoinCreatorFeeBasisPoints uint64
}

func LegacyPumpSwapFeeBasisPoints(hasCoinCreator bool) PumpSwapFeeBasisPoints {
	coinCreatorFee := uint64(0)
	if hasCoinCreator {
		coinCreatorFee = PumpSwapCoinCreatorFeeBasisPoints
	}
	return PumpSwapFeeBasisPoints{
		LPFeeBasisPoints:          PumpSwapLPFeeBasisPoints,
		ProtocolFeeBasisPoints:    PumpSwapProtocolFeeBasisPoints,
		CoinCreatorFeeBasisPoints: coinCreatorFee,
	}
}

// BuyBaseInputResult contains results for buying base tokens with base amount input
type BuyBaseInputResult struct {
	InternalQuoteAmount uint64
	UIQuote             uint64
	MaxQuote            uint64
}

// BuyQuoteInputResult contains results for buying base tokens with quote amount input
type BuyQuoteInputResult struct {
	Base                     uint64
	InternalQuoteWithoutFees uint64
	MaxQuote                 uint64
}

// SellBaseInputResult contains results for selling base tokens with base amount input
type SellBaseInputResult struct {
	UIQuote                uint64
	MinQuote               uint64
	InternalQuoteAmountOut uint64
}

// SellQuoteInputResult contains results for selling base tokens with quote amount input
type SellQuoteInputResult struct {
	InternalRawQuote uint64
	Base             uint64
	MinQuote         uint64
}

// EffectiveQuoteReserves returns the PumpSwap reserves used for pricing.
// virtualQuoteReserves is a signed i128 on chain; nil is treated as zero.
func EffectiveQuoteReserves(quoteVaultBalance uint64, virtualQuoteReserves *big.Int) (uint64, error) {
	effective := new(big.Int).SetUint64(quoteVaultBalance)
	if virtualQuoteReserves != nil {
		if virtualQuoteReserves.Cmp(pumpSwapI128Min) < 0 || virtualQuoteReserves.Cmp(pumpSwapI128Max) > 0 {
			return 0, fmt.Errorf("virtual quote reserves are outside the signed i128 range: %s", virtualQuoteReserves)
		}
		effective.Add(effective, virtualQuoteReserves)
	}
	if effective.Sign() <= 0 || !effective.IsUint64() {
		virtual := "0"
		if virtualQuoteReserves != nil {
			virtual = virtualQuoteReserves.String()
		}
		return 0, fmt.Errorf("invalid effective quote reserves: raw=%d, virtual=%s", quoteVaultBalance, virtual)
	}
	return effective.Uint64(), nil
}

// BuyBaseInputInternal calculates quote needed to buy base tokens on PumpSwap
func BuyBaseInputInternal(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	hasCoinCreator bool,
) (*BuyBaseInputResult, error) {
	fees := LegacyPumpSwapFeeBasisPoints(hasCoinCreator)
	return BuyBaseInputInternalWithFees(base, slippageBasisPoints, baseReserve, quoteReserve, virtualQuoteReserves, fees)
}

func BuyBaseInputInternalWithFees(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	feeBasisPoints PumpSwapFeeBasisPoints,
) (*BuyBaseInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	effectiveQuoteReserve, err := EffectiveQuoteReserves(quoteReserve, virtualQuoteReserves)
	if err != nil {
		return nil, err
	}
	if base > baseReserve {
		return nil, ErrInsufficientReserves
	}

	// Use 128-bit multiplication to avoid overflow
	numerator := mul128(effectiveQuoteReserve, base)
	denominator := baseReserve - base

	if denominator == 0 {
		return nil, ErrPoolDepleted
	}

	quoteAmountIn := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		quoteAmountIn++
	}

	lpFee, _ := ComputeFee(quoteAmountIn, feeBasisPoints.LPFeeBasisPoints)
	protocolFee, _ := ComputeFee(quoteAmountIn, feeBasisPoints.ProtocolFeeBasisPoints)
	coinCreatorFee, _ := ComputeFee(quoteAmountIn, feeBasisPoints.CoinCreatorFeeBasisPoints)

	totalQuote := quoteAmountIn + lpFee + protocolFee + coinCreatorFee
	maxQuote, _ := CalculateWithSlippageBuy(totalQuote, slippageBasisPoints)

	return &BuyBaseInputResult{
		InternalQuoteAmount: quoteAmountIn,
		UIQuote:             totalQuote,
		MaxQuote:            maxQuote,
	}, nil
}

// BuyQuoteInputInternal calculates base tokens received for quote input on PumpSwap
func BuyQuoteInputInternal(
	quote uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	hasCoinCreator bool,
) (*BuyQuoteInputResult, error) {
	fees := LegacyPumpSwapFeeBasisPoints(hasCoinCreator)
	return BuyQuoteInputInternalWithFees(quote, slippageBasisPoints, baseReserve, quoteReserve, virtualQuoteReserves, fees)
}

func BuyQuoteInputInternalWithFees(
	quote uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	feeBasisPoints PumpSwapFeeBasisPoints,
) (*BuyQuoteInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	effectiveQuoteReserve, err := EffectiveQuoteReserves(quoteReserve, virtualQuoteReserves)
	if err != nil {
		return nil, err
	}

	totalFeeBps := feeBasisPoints.LPFeeBasisPoints + feeBasisPoints.ProtocolFeeBasisPoints + feeBasisPoints.CoinCreatorFeeBasisPoints
	denominator := 10000 + totalFeeBps

	// Use 128-bit arithmetic
	effectiveQuote := div128(mul128(quote, 10000), uint64(denominator))
	lpFee, _ := ComputeFee(effectiveQuote, feeBasisPoints.LPFeeBasisPoints)
	protocolFee, _ := ComputeFee(effectiveQuote, feeBasisPoints.ProtocolFeeBasisPoints)
	coinCreatorFee, _ := ComputeFee(effectiveQuote, feeBasisPoints.CoinCreatorFeeBasisPoints)
	totalWithFees := effectiveQuote + lpFee + protocolFee + coinCreatorFee
	if totalWithFees > quote {
		effectiveQuote -= totalWithFees - quote
	}
	inputAmount := uint64(0)
	if effectiveQuote > 0 {
		inputAmount = effectiveQuote - 1
	}

	// numerator = baseReserve * effectiveQuote
	numerator := mul128(baseReserve, inputAmount)
	// denominatorEffective = quoteReserve + effectiveQuote
	denominatorEffective := effectiveQuoteReserve + inputAmount
	if denominatorEffective < effectiveQuoteReserve {
		return nil, ErrOverflow
	}

	if denominatorEffective == 0 {
		return nil, ErrPoolDepleted
	}

	baseAmountOut := div128(numerator, denominatorEffective)
	maxQuote, _ := CalculateWithSlippageBuy(quote, slippageBasisPoints)

	return &BuyQuoteInputResult{
		Base:                     baseAmountOut,
		InternalQuoteWithoutFees: effectiveQuote,
		MaxQuote:                 maxQuote,
	}, nil
}

// SellBaseInputInternal calculates quote received for selling base tokens on PumpSwap
func SellBaseInputInternal(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	hasCoinCreator bool,
) (*SellBaseInputResult, error) {
	fees := LegacyPumpSwapFeeBasisPoints(hasCoinCreator)
	return SellBaseInputInternalWithFees(base, slippageBasisPoints, baseReserve, quoteReserve, virtualQuoteReserves, fees)
}

func SellBaseInputInternalWithFees(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	feeBasisPoints PumpSwapFeeBasisPoints,
) (*SellBaseInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	effectiveQuoteReserve, err := EffectiveQuoteReserves(quoteReserve, virtualQuoteReserves)
	if err != nil {
		return nil, err
	}

	// Use 128-bit arithmetic: (quoteReserve * base) / (baseReserve + base)
	numerator := mul128(effectiveQuoteReserve, base)
	denominator := baseReserve + base
	if denominator == 0 {
		return nil, ErrPoolDepleted
	}
	quoteAmountOutUint := div128(numerator, denominator)

	lpFee, _ := ComputeFee(quoteAmountOutUint, feeBasisPoints.LPFeeBasisPoints)
	protocolFee, _ := ComputeFee(quoteAmountOutUint, feeBasisPoints.ProtocolFeeBasisPoints)
	coinCreatorFee, _ := ComputeFee(quoteAmountOutUint, feeBasisPoints.CoinCreatorFeeBasisPoints)

	totalFees := lpFee + protocolFee + coinCreatorFee
	if totalFees > quoteAmountOutUint {
		return nil, ErrFeesExceedOutput
	}
	quoteVaultOutflow := quoteAmountOutUint - lpFee
	if quoteVaultOutflow > quoteReserve {
		return nil, errors.New("insufficient real quote reserves to cover the sell output")
	}
	finalQuote := quoteAmountOutUint - totalFees
	minQuote, _ := CalculateWithSlippageSell(finalQuote, slippageBasisPoints)

	return &SellBaseInputResult{
		UIQuote:                finalQuote,
		MinQuote:               minQuote,
		InternalQuoteAmountOut: quoteAmountOutUint,
	}, nil
}

// SellQuoteInputInternal calculates base needed to receive quote amount on PumpSwap
func SellQuoteInputInternal(
	quote uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	hasCoinCreator bool,
) (*SellQuoteInputResult, error) {
	fees := LegacyPumpSwapFeeBasisPoints(hasCoinCreator)
	return SellQuoteInputInternalWithFees(quote, slippageBasisPoints, baseReserve, quoteReserve, virtualQuoteReserves, fees)
}

func SellQuoteInputInternalWithFees(
	quote uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	virtualQuoteReserves *big.Int,
	feeBasisPoints PumpSwapFeeBasisPoints,
) (*SellQuoteInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	if quote > quoteReserve {
		return nil, ErrInsufficientReserves
	}
	effectiveQuoteReserve, err := EffectiveQuoteReserves(quoteReserve, virtualQuoteReserves)
	if err != nil {
		return nil, err
	}

	rawQuote := calculateQuoteAmountOut(
		quote,
		feeBasisPoints.LPFeeBasisPoints,
		feeBasisPoints.ProtocolFeeBasisPoints,
		feeBasisPoints.CoinCreatorFeeBasisPoints,
	)

	lpFee, err := ComputeFee(rawQuote, feeBasisPoints.LPFeeBasisPoints)
	if err != nil {
		return nil, err
	}
	if rawQuote < lpFee || rawQuote-lpFee > quoteReserve {
		return nil, errors.New("insufficient real quote reserves to cover the sell output")
	}

	if rawQuote >= effectiveQuoteReserve {
		return nil, ErrInvalidInputCalc
	}

	// Use 128-bit arithmetic for ceiling division
	numerator := mul128(baseReserve, rawQuote)
	denominator := effectiveQuoteReserve - rawQuote
	baseAmountIn := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		baseAmountIn++
	}
	minQuote, _ := CalculateWithSlippageSell(quote, slippageBasisPoints)

	return &SellQuoteInputResult{
		InternalRawQuote: rawQuote,
		Base:             baseAmountIn,
		MinQuote:         minQuote,
	}, nil
}

func calculateQuoteAmountOut(
	userQuoteAmountOut uint64,
	lpFeeBasisPoints uint64,
	protocolFeeBasisPoints uint64,
	coinCreatorFeeBasisPoints uint64,
) uint64 {
	totalFeeBasisPoints := lpFeeBasisPoints + protocolFeeBasisPoints + coinCreatorFeeBasisPoints
	denominator := 10000 - totalFeeBasisPoints
	// Use 128-bit arithmetic
	numerator := mul128(userQuoteAmountOut, 10000)
	result := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		result++
	}
	return result
}

// ===== Bonk Calculations =====

// Bonk Constants - 100% from Rust: src/instruction/utils/bonk.rs accounts
const (
	BonkProtocolFeeRate     uint64 = 25  // 0.25%
	BonkPlatformFeeRate     uint64 = 100 // 1%
	BonkShareFeeRate        uint64 = 0   // 0%
	BonkDefaultVirtualBase  uint64 = 1073025605596382
	BonkDefaultVirtualQuote uint64 = 30000852951
)

// GetBonkAmountOut calculates output amount for Bonk
func GetBonkAmountOut(
	amountIn uint64,
	protocolFeeRate uint64,
	platformFeeRate uint64,
	shareFeeRate uint64,
	virtualBase uint64,
	virtualQuote uint64,
	realBase uint64,
	realQuote uint64,
	feeDirection int, // 0 = fee on output, 1 = fee on input
) uint64 {
	// Simplified Bonk calculation
	if virtualBase == 0 || virtualQuote == 0 {
		return 0
	}

	// Use 128-bit arithmetic
	amountOut := div128(mul128(amountIn, virtualQuote), virtualBase)
	return amountOut
}

// GetBonkAmountIn calculates input amount needed for desired output on Bonk
func GetBonkAmountIn(
	amountOut uint64,
	protocolFeeRate uint64,
	platformFeeRate uint64,
	shareFeeRate uint64,
	virtualBase uint64,
	virtualQuote uint64,
	realBase uint64,
	realQuote uint64,
) uint64 {
	if virtualBase == 0 || virtualQuote == 0 {
		return 0
	}

	totalFeeRate := protocolFeeRate + platformFeeRate + shareFeeRate
	// Use 128-bit arithmetic
	amountIn := div128(mul128(amountOut, 10000), 10000-totalFeeRate)
	amountIn = div128(mul128(amountIn, virtualBase), virtualQuote)

	return amountIn
}

// GetBonkAmountInNet calculates net input amount after fees
func GetBonkAmountInNet(
	amountIn uint64,
	protocolFeeRate uint64,
	platformFeeRate uint64,
	shareFeeRate uint64,
) uint64 {
	totalFeeRate := protocolFeeRate + platformFeeRate + shareFeeRate
	return amountIn * (10000 - totalFeeRate) / 10000
}

// ===== Raydium CPMM Calculations =====

// RaydiumCPMMGetAmountOut calculates output amount for Raydium CPMM
func RaydiumCPMMGetAmountOut(
	amountIn uint64,
	inputReserve uint64,
	outputReserve uint64,
	hasFee bool,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 {
		return 0
	}

	// Use 128-bit arithmetic
	numerator := mul128(amountIn, outputReserve)
	denominator := inputReserve + amountIn
	amountOut := div128(numerator, denominator)

	if hasFee {
		// Apply fee
		amountOut = amountOut * 997 / 1000 // 0.3% fee
	}

	return amountOut
}

// RaydiumCPMMGetAmountIn calculates input amount needed for Raydium CPMM
func RaydiumCPMMGetAmountIn(
	amountOut uint64,
	inputReserve uint64,
	outputReserve uint64,
	hasFee bool,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 || amountOut >= outputReserve {
		return 0
	}

	if hasFee {
		amountOut = amountOut * 1000 / 997
	}

	// Use 128-bit arithmetic
	numerator := mul128(inputReserve, amountOut)
	denominator := outputReserve - amountOut
	result := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		result++
	}
	return result
}

// ===== Raydium AMM V4 Calculations =====

// RaydiumAmmV4GetAmountOut calculates output amount for Raydium AMM V4
func RaydiumAmmV4GetAmountOut(
	amountIn uint64,
	inputReserve uint64,
	outputReserve uint64,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 {
		return 0
	}

	// Apply 0.25% fee - use 128-bit arithmetic
	// numerator = amountIn * 9975 * outputReserve
	numeratorFullHi, numeratorFullLo := bits.Mul64(amountIn, 9975)
	numeratorHi, numeratorLo := bits.Mul64(numeratorFullLo, outputReserve)
	// For full 128x64 multiplication, we'd need to handle numeratorFullHi
	// For Solana amounts, numeratorFullHi should be 0 (amounts < 2^64)
	_ = numeratorFullHi // Assume 0 for valid Solana amounts

	// denominator = inputReserve * 10000 + amountIn * 9975
	amountInWithFee := numeratorFullLo
	denominator := inputReserve*10000 + amountInWithFee

	return div128(&Uint128Result{Hi: numeratorHi, Lo: numeratorLo}, denominator)
}

// RaydiumAmmV4GetAmountIn calculates input amount needed for Raydium AMM V4
func RaydiumAmmV4GetAmountIn(
	amountOut uint64,
	inputReserve uint64,
	outputReserve uint64,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 || amountOut >= outputReserve {
		return 0
	}

	// Use 128-bit arithmetic
	numerator := mul128(inputReserve, amountOut)
	numerator = mul128(div128(numerator, 1), 10000)
	denominator := (outputReserve - amountOut) * 9975

	result := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		result++
	}
	return result
}

// ===== Price Calculations =====

// CalculatePriceImpact calculates price impact percentage
func CalculatePriceImpact(reserveIn uint64, amountIn uint64) float64 {
	if reserveIn == 0 {
		return 0
	}
	return float64(amountIn) * 100 / float64(reserveIn)
}

// CalculatePrice calculates price from reserves
func CalculatePrice(quoteReserve uint64, baseReserve uint64, quoteDecimals uint8, baseDecimals uint8) float64 {
	if baseReserve == 0 {
		return 0
	}
	quoteAdjusted := float64(quoteReserve) / math.Pow10(int(quoteDecimals))
	baseAdjusted := float64(baseReserve) / math.Pow10(int(baseDecimals))
	return quoteAdjusted / baseAdjusted
}

// ===== Meteora DAMM V2 Calculations =====

// MeteoraSwapResult contains the result of a Meteora swap calculation
type MeteoraSwapResult struct {
	AmountOut    uint64
	MinAmountOut uint64
}

// MeteoraDammV2ComputeSwapAmount calculates swap amount for Meteora DAMM V2
func MeteoraDammV2ComputeSwapAmount(
	tokenAReserve uint64,
	tokenBReserve uint64,
	isAToB bool,
	amountIn uint64,
	slippageBasisPoints uint64,
) *MeteoraSwapResult {
	if amountIn == 0 {
		return &MeteoraSwapResult{AmountOut: 0, MinAmountOut: 0}
	}

	var amountOut uint64

	if isAToB {
		// Swapping token A for token B
		if tokenAReserve == 0 {
			return &MeteoraSwapResult{AmountOut: 0, MinAmountOut: 0}
		}

		// Constant product: b_out = (b_reserve * a_in) / (a_reserve + a_in)
		// Use 128-bit arithmetic
		numerator := mul128(tokenBReserve, amountIn)
		denominator := tokenAReserve + amountIn

		if denominator == 0 {
			return &MeteoraSwapResult{AmountOut: 0, MinAmountOut: 0}
		}

		amountOut = div128(numerator, denominator)
	} else {
		// Swapping token B for token A
		if tokenBReserve == 0 {
			return &MeteoraSwapResult{AmountOut: 0, MinAmountOut: 0}
		}

		// Constant product: a_out = (a_reserve * b_in) / (b_reserve + b_in)
		// Use 128-bit arithmetic
		numerator := mul128(tokenAReserve, amountIn)
		denominator := tokenBReserve + amountIn

		if denominator == 0 {
			return &MeteoraSwapResult{AmountOut: 0, MinAmountOut: 0}
		}

		amountOut = div128(numerator, denominator)
	}

	// Apply slippage
	minAmountOut, _ := CalculateWithSlippageSell(amountOut, slippageBasisPoints)

	return &MeteoraSwapResult{
		AmountOut:    amountOut,
		MinAmountOut: minAmountOut,
	}
}

// MeteoraDammV2CalculatePrice calculates current price (token B per token A)
func MeteoraDammV2CalculatePrice(tokenAReserve uint64, tokenBReserve uint64) float64 {
	if tokenAReserve == 0 {
		return 0.0
	}
	return float64(tokenBReserve) / float64(tokenAReserve)
}

// MeteoraDammV2CalculateLiquidity calculates liquidity (geometric mean of reserves)
func MeteoraDammV2CalculateLiquidity(tokenAReserve uint64, tokenBReserve uint64) uint64 {
	if tokenAReserve == 0 || tokenBReserve == 0 {
		return 0
	}
	return uint64(math.Sqrt(float64(tokenAReserve) * float64(tokenBReserve)))
}

// MeteoraDammV2GetAmountOut calculates output amount with fee consideration
func MeteoraDammV2GetAmountOut(
	amountIn uint64,
	inputReserve uint64,
	outputReserve uint64,
	feeBasisPoints uint64,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 || amountIn == 0 {
		return 0
	}

	// Apply fee - use 128-bit arithmetic
	amountInAfterFee := amountIn * (10000 - feeBasisPoints) / 10000

	// numerator = amountInAfterFee * outputReserve
	numerator := mul128(amountInAfterFee, outputReserve)
	denominator := inputReserve + amountInAfterFee

	return div128(numerator, denominator)
}

// MeteoraDammV2GetAmountIn calculates input amount needed for desired output
func MeteoraDammV2GetAmountIn(
	amountOut uint64,
	inputReserve uint64,
	outputReserve uint64,
	feeBasisPoints uint64,
) uint64 {
	if inputReserve == 0 || outputReserve == 0 || amountOut >= outputReserve {
		return 0
	}

	// Use 128-bit arithmetic
	// numerator = inputReserve * amountOut * 10000
	numeratorStep1 := mul128(inputReserve, amountOut)
	numerator := mul128(div128(numeratorStep1, 1), 10000)
	denominator := (outputReserve - amountOut) * (10000 - feeBasisPoints)

	result := div128(numerator, denominator)
	// Add 1 for ceiling division
	if numerator.Lo%denominator != 0 || numerator.Hi != 0 {
		result++
	}
	return result
}

// ===== Utility Types =====

// Uint128Result holds a 128-bit result from multiplication
type Uint128Result struct {
	Hi uint64
	Lo uint64
}

// mul128 performs 128-bit multiplication of two uint64 values
func mul128(a, b uint64) *Uint128Result {
	hi, lo := bits.Mul64(a, b)
	return &Uint128Result{Hi: hi, Lo: lo}
}

// div128 divides a 128-bit value by a 64-bit value
func div128(n *Uint128Result, d uint64) uint64 {
	if d == 0 {
		return 0
	}
	quo, _ := bits.Div64(n.Hi, n.Lo, d)
	return quo
}

// add128 adds two 128-bit values
func add128(a, b *Uint128Result) *Uint128Result {
	lo, carry := bits.Add64(a.Lo, b.Lo, 0)
	hi, _ := bits.Add64(a.Hi, b.Hi, carry)
	return &Uint128Result{Hi: hi, Lo: lo}
}

// Error definitions - using CalcError for detailed error reporting
var (
	ErrInvalidReserves      = &CalcError{Code: 1, Message: "invalid reserves"}
	ErrInsufficientReserves = &CalcError{Code: 2, Message: "insufficient reserves"}
	ErrPoolDepleted         = &CalcError{Code: 3, Message: "pool depleted"}
	ErrFeesExceedOutput     = &CalcError{Code: 4, Message: "fees exceed output"}
	// ErrInvalidInputCalc is used for calculation-specific invalid input errors
	ErrInvalidInputCalc = &CalcError{Code: 5, Message: "invalid input"}
)

// CalcError represents a calculation error
type CalcError struct {
	Code    int
	Message string
}

func (e *CalcError) Error() string {
	return e.Message
}

// ===== Price Calculation Functions - from Rust: src/utils/price/bonk.rs =====

const (
	DefaultTokenDecimals = 6
	SolDecimals          = 9
)

// PriceTokenInWsol calculates the price of token in WSOL.
// 100% from Rust: src/utils/price/bonk.rs price_token_in_wsol
func PriceTokenInWsol(virtualBase, virtualQuote, realBase, realQuote uint64) float64 {
	return PriceBaseInQuote(virtualBase, virtualQuote, realBase, realQuote, DefaultTokenDecimals, SolDecimals)
}

// PriceBaseInQuote calculates the price of base in quote.
// 100% from Rust: src/utils/price/bonk.rs price_base_in_quote
func PriceBaseInQuote(virtualBase, virtualQuote, realBase, realQuote uint64, baseDecimals, quoteDecimals int) float64 {
	// Calculate decimal places difference
	decimalDiff := quoteDecimals - baseDecimals
	var decimalFactor float64
	if decimalDiff >= 0 {
		decimalFactor = math.Pow(10, float64(decimalDiff))
	} else {
		decimalFactor = 1.0 / math.Pow(10, float64(-decimalDiff))
	}

	// Calculate reserves state before price calculation
	quoteReserves := virtualQuote + realQuote
	var baseReserves uint64
	if virtualBase > realBase {
		baseReserves = virtualBase - realBase
	}

	if baseReserves == 0 {
		return 0.0
	}

	if decimalFactor == 0.0 {
		return 0.0
	}

	// Use floating point calculation to avoid precision loss
	price := (float64(quoteReserves) / float64(baseReserves)) / decimalFactor

	return price
}

// ===== Price Calculation Functions - from Rust: src/utils/price/common.rs =====

// PriceBaseInQuoteFromReserves calculates the token price in quote based on base and quote reserves.
// 100% from Rust: src/utils/price/common.rs price_base_in_quote
func PriceBaseInQuoteFromReserves(baseReserve, quoteReserve uint64, baseDecimals, quoteDecimals uint8) float64 {
	base := float64(baseReserve) / math.Pow10(int(baseDecimals))
	quote := float64(quoteReserve) / math.Pow10(int(quoteDecimals))
	if base == 0.0 {
		return 0.0
	}
	return quote / base
}

// PriceQuoteInBase calculates the token price in base based on base and quote reserves.
// 100% from Rust: src/utils/price/common.rs price_quote_in_base
func PriceQuoteInBase(baseReserve, quoteReserve uint64, baseDecimals, quoteDecimals uint8) float64 {
	base := float64(baseReserve) / math.Pow10(int(baseDecimals))
	quote := float64(quoteReserve) / math.Pow10(int(quoteDecimals))
	if quote == 0.0 {
		return 0.0
	}
	return base / quote
}

// ===== Price Calculation Functions - from Rust: src/utils/price/pumpfun.rs =====

const (
	LamportsPerSol = 1_000_000_000
	Scale          = 1_000_000_000 // PumpFun scale factor
)

// PriceTokenInSol calculates the token price in SOL based on virtual reserves.
// 100% from Rust: src/utils/price/pumpfun.rs price_token_in_sol
func PriceTokenInSol(virtualSolReserves, virtualTokenReserves uint64) float64 {
	vSol := float64(virtualSolReserves) / float64(LamportsPerSol)
	vTokens := float64(virtualTokenReserves) / float64(Scale)
	if vTokens == 0.0 {
		return 0.0
	}
	return vSol / vTokens
}

// ===== Raydium CLMM Price Calculations - from Rust: src/utils/price/raydium_clmm.rs =====

// PriceToken0InToken1 calculates the price of token0 in token1 from sqrt price.
// 100% from Rust: src/utils/price/raydium_clmm.rs price_token0_in_token1
func PriceToken0InToken1(sqrtPriceX64 uint64, decimalsToken0, decimalsToken1 uint8) float64 {
	sqrtPrice := float64(sqrtPriceX64) / float64(1<<64) // Q64.64 to float
	priceRaw := sqrtPrice * sqrtPrice                   // Price without decimal adjustment
	scale := math.Pow(10, float64(decimalsToken0)-float64(decimalsToken1))
	return priceRaw * scale
}

// PriceToken1InToken0 calculates the price of token1 in token0 from sqrt price.
// 100% from Rust: src/utils/price/raydium_clmm.rs price_token1_in_token0
func PriceToken1InToken0(sqrtPriceX64 uint64, decimalsToken0, decimalsToken1 uint8) float64 {
	if sqrtPriceX64 == 0 {
		return 0.0
	}
	return 1.0 / PriceToken0InToken1(sqrtPriceX64, decimalsToken0, decimalsToken1)
}
