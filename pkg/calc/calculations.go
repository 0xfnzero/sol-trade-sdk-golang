package calc

import (
	"errors"
	"math"
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
	_, overflow := bits.Mul64(a, b)
	if overflow {
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
func CalculateWithSlippageBuy(amount uint64, basisPoints uint64) (uint64, error) {
	if err := validateAmount(amount, "amount"); err != nil {
		return 0, err
	}
	if err := validateBasisPoints(basisPoints); err != nil {
		return 0, err
	}
	if err := checkMulOverflow(amount, basisPoints); err != nil {
		return 0, err
	}
	slippageAmount := (amount * basisPoints) / 10000
	if err := checkAddOverflow(amount, slippageAmount); err != nil {
		return 0, err
	}
	return amount + slippageAmount, nil
}

// CalculateWithSlippageSell calculates sell amount with slippage protection
// Includes underflow protection
func CalculateWithSlippageSell(amount uint64, basisPoints uint64) (uint64, error) {
	if err := validateAmount(amount, "amount"); err != nil {
		return 0, err
	}
	if err := validateBasisPoints(basisPoints); err != nil {
		return 0, err
	}
	if err := checkMulOverflow(amount, basisPoints); err != nil {
		return 0, err
	}
	slippageAmount := (amount * basisPoints) / 10000
	if slippageAmount >= amount {
		return 0, ErrUnderflow
	}
	return amount - slippageAmount, nil
}

// ===== PumpFun Calculations =====

// PumpFun Constants
const (
	PumpFunFeeBasisPoints     uint64 = 100  // 1%
	PumpFunCreatorFee         uint64 = 50   // 0.5%
	PumpFunInitialVirtualToken        = 1073000000000000
	PumpFunInitialVirtualSol          = 30000000000
	PumpFunInitialRealToken           = 793000000000000
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

	fee := ComputeFee(solCost, totalFeeBasisPoints)

	if solCost < fee {
		return 0
	}
	return solCost - fee
}

// ===== PumpSwap Calculations =====

// PumpSwap Constants
const (
	PumpSwapLPFeeBasisPoints         uint64 = 20   // 0.2%
	PumpSwapProtocolFeeBasisPoints   uint64 = 20   // 0.2%
	PumpSwapCoinCreatorFeeBasisPoints uint64 = 10  // 0.1%
)

// BuyBaseInputResult contains results for buying base tokens with base amount input
type BuyBaseInputResult struct {
	InternalQuoteAmount uint64
	UIQuote             uint64
	MaxQuote            uint64
}

// BuyQuoteInputResult contains results for buying base tokens with quote amount input
type BuyQuoteInputResult struct {
	Base                      uint64
	InternalQuoteWithoutFees uint64
	MaxQuote                  uint64
}

// SellBaseInputResult contains results for selling base tokens with base amount input
type SellBaseInputResult struct {
	UIQuote                   uint64
	MinQuote                  uint64
	InternalQuoteAmountOut    uint64
}

// SellQuoteInputResult contains results for selling base tokens with quote amount input
type SellQuoteInputResult struct {
	InternalRawQuote uint64
	Base             uint64
	MinQuote         uint64
}

// BuyBaseInputInternal calculates quote needed to buy base tokens on PumpSwap
func BuyBaseInputInternal(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	hasCoinCreator bool,
) (*BuyBaseInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	if base > baseReserve {
		return nil, ErrInsufficientReserves
	}

	numerator := uint128(quoteReserve) * uint128(base)
	denominator := baseReserve - base

	if denominator == 0 {
		return nil, ErrPoolDepleted
	}

	quoteAmountIn := CeilDiv(uint64(numerator), uint64(denominator))

	lpFee := ComputeFee(quoteAmountIn, PumpSwapLPFeeBasisPoints)
	protocolFee := ComputeFee(quoteAmountIn, PumpSwapProtocolFeeBasisPoints)
	coinCreatorFee := uint64(0)
	if hasCoinCreator {
		coinCreatorFee = ComputeFee(quoteAmountIn, PumpSwapCoinCreatorFeeBasisPoints)
	}

	totalQuote := quoteAmountIn + lpFee + protocolFee + coinCreatorFee
	maxQuote := CalculateWithSlippageBuy(totalQuote, slippageBasisPoints)

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
	hasCoinCreator bool,
) (*BuyQuoteInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}

	totalFeeBps := PumpSwapLPFeeBasisPoints + PumpSwapProtocolFeeBasisPoints
	if hasCoinCreator {
		totalFeeBps += PumpSwapCoinCreatorFeeBasisPoints
	}
	denominator := 10000 + totalFeeBps

	effectiveQuote := (uint128(quote) * 10000) / uint128(denominator)

	numerator := uint128(baseReserve) * effectiveQuote
	denominatorEffective := uint128(quoteReserve) + effectiveQuote

	if denominatorEffective == 0 {
		return nil, ErrPoolDepleted
	}

	baseAmountOut := uint64(numerator / denominatorEffective)
	maxQuote := CalculateWithSlippageBuy(quote, slippageBasisPoints)

	return &BuyQuoteInputResult{
		Base:                      baseAmountOut,
		InternalQuoteWithoutFees: uint64(effectiveQuote),
		MaxQuote:                  maxQuote,
	}, nil
}

// SellBaseInputInternal calculates quote received for selling base tokens on PumpSwap
func SellBaseInputInternal(
	base uint64,
	slippageBasisPoints uint64,
	baseReserve uint64,
	quoteReserve uint64,
	hasCoinCreator bool,
) (*SellBaseInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}

	quoteAmountOut := (uint128(quoteReserve) * uint128(base)) / (uint128(baseReserve) + uint128(base))
	quoteAmountOutUint := uint64(quoteAmountOut)

	lpFee := ComputeFee(quoteAmountOutUint, PumpSwapLPFeeBasisPoints)
	protocolFee := ComputeFee(quoteAmountOutUint, PumpSwapProtocolFeeBasisPoints)
	coinCreatorFee := uint64(0)
	if hasCoinCreator {
		coinCreatorFee = ComputeFee(quoteAmountOutUint, PumpSwapCoinCreatorFeeBasisPoints)
	}

	totalFees := lpFee + protocolFee + coinCreatorFee
	if totalFees > quoteAmountOutUint {
		return nil, ErrFeesExceedOutput
	}
	finalQuote := quoteAmountOutUint - totalFees
	minQuote := CalculateWithSlippageSell(finalQuote, slippageBasisPoints)

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
	hasCoinCreator bool,
) (*SellQuoteInputResult, error) {
	if baseReserve == 0 || quoteReserve == 0 {
		return nil, ErrInvalidReserves
	}
	if quote > quoteReserve {
		return nil, ErrInsufficientReserves
	}

	coinCreatorFee := uint64(0)
	if hasCoinCreator {
		coinCreatorFee = PumpSwapCoinCreatorFeeBasisPoints
	}

	rawQuote := calculateQuoteAmountOut(quote, PumpSwapLPFeeBasisPoints, PumpSwapProtocolFeeBasisPoints, coinCreatorFee)

	if rawQuote >= quoteReserve {
		return nil, ErrInvalidInput
	}

	baseAmountIn := CeilDiv(uint128(baseReserve)*uint128(rawQuote), uint128(quoteReserve-rawQuote))
	minQuote := CalculateWithSlippageSell(quote, slippageBasisPoints)

	return &SellQuoteInputResult{
		InternalRawQuote: rawQuote,
		Base:             uint64(baseAmountIn),
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
	return CeilDiv(uint128(userQuoteAmountOut)*10000, uint128(denominator))
}

// ===== Bonk Calculations =====

// Bonk Constants
const (
	BonkProtocolFeeRate    uint64 = 25   // 0.25%
	BonkPlatformFeeRate    uint64 = 50   // 0.5%
	BonkShareFeeRate       uint64 = 25   // 0.25%
	BonkDefaultVirtualBase uint64 = 1073025605596382
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

	amountOut := (uint128(amountIn) * uint128(virtualQuote)) / uint128(virtualBase)
	return uint64(amountOut)
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
	amountIn := (uint128(amountOut) * 10000) / (10000 - uint128(totalFeeRate))
	amountIn = (amountIn * uint128(virtualBase)) / uint128(virtualQuote)

	return uint64(amountIn)
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

	amountOut := (uint128(amountIn) * uint128(outputReserve)) / (uint128(inputReserve) + uint128(amountIn))
	
	if hasFee {
		// Apply fee
		amountOut = amountOut * 997 / 1000 // 0.3% fee
	}
	
	return uint64(amountOut)
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

	numerator := uint128(inputReserve) * uint128(amountOut)
	denominator := uint128(outputReserve) - uint128(amountOut)

	return uint64(CeilDiv(uint64(numerator), uint64(denominator)))
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

	// Apply 0.25% fee
	amountInWithFee := uint128(amountIn) * 9975
	numerator := amountInWithFee * uint128(outputReserve)
	denominator := uint128(inputReserve)*10000 + amountInWithFee

	return uint64(numerator / denominator)
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

	numerator := uint128(inputReserve) * uint128(amountOut) * 10000
	denominator := (uint128(outputReserve) - uint128(amountOut)) * 9975

	return uint64(CeilDiv(uint64(numerator), uint64(denominator)))
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

// ===== Utility Types =====

type uint128 = uint64 // Simplified for Go - full implementation would use math/big

// Error definitions
var (
	ErrInvalidReserves    = &CalcError{Code: 1, Message: "invalid reserves"}
	ErrInsufficientReserves = &CalcError{Code: 2, Message: "insufficient reserves"}
	ErrPoolDepleted       = &CalcError{Code: 3, Message: "pool depleted"}
	ErrFeesExceedOutput   = &CalcError{Code: 4, Message: "fees exceed output"}
	ErrInvalidInput       = &CalcError{Code: 5, Message: "invalid input"}
)

// CalcError represents a calculation error
type CalcError struct {
	Code    int
	Message string
}

func (e *CalcError) Error() string {
	return e.Message
}
