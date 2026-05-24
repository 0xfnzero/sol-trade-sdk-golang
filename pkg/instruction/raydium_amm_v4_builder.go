// Raydium AMM V4 instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
)

// ===== Raydium AMM V4 Program Constants from Rust: src/instruction/utils/raydium_amm_v4.rs =====

var (
	// RAYDIUM_AMM_V4_PROGRAM is the Raydium AMM V4 program ID
	RAYDIUM_AMM_V4_PROGRAM = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
	// RAYDIUM_AMM_V4_AUTHORITY is the program authority
	RAYDIUM_AMM_V4_AUTHORITY = solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1")
)

// Discriminators - from Rust: src/instruction/utils/raydium_amm_v4.rs
var (
	// RaydiumAmmV4SwapBaseInDiscriminator is the discriminator for swap_base_in instruction
	RaydiumAmmV4SwapBaseInDiscriminator = []byte{9}
	// RaydiumAmmV4SwapBaseOutDiscriminator is the discriminator for swap_base_out instruction
	RaydiumAmmV4SwapBaseOutDiscriminator = []byte{11}
)

// Raydium AMM V4 Constants - from Rust: src/instruction/utils/raydium_amm_v4.rs accounts
const (
	RaydiumAmmV4TradeFeeNumerator   uint64 = 25
	RaydiumAmmV4TradeFeeDenominator uint64 = 10000
	RaydiumAmmV4SwapFeeNumerator    uint64 = 25
	RaydiumAmmV4SwapFeeDenominator  uint64 = 10000
)

// ===== Raydium AMM V4 Params =====

// RaydiumAmmV4Params contains parameters for Raydium AMM V4 operations
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

// RaydiumAmmV4BuildBuyParams contains parameters for building buy instructions
type RaydiumAmmV4BuildBuyParams struct {
	Payer               solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *RaydiumAmmV4Params
	CreateInputMintAta  bool
	CreateOutputMintAta bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// RaydiumAmmV4BuildSellParams contains parameters for building sell instructions
type RaydiumAmmV4BuildSellParams struct {
	Payer               solana.PublicKey
	InputMint           solana.PublicKey
	OutputMint          solana.PublicKey
	InputAmount         uint64
	SlippageBasisPoints uint64
	ProtocolParams      *RaydiumAmmV4Params
	CreateOutputMintAta bool
	CloseOutputMintAta  bool
	CloseInputMintAta   bool
	FixedOutputAmount   *uint64
}

// ===== Instruction Builders - 100% from Rust =====

func ensureRaydiumAmmV4MarketAccounts(pp *RaydiumAmmV4Params) error {
	required := []struct {
		name    string
		account solana.PublicKey
	}{
		{"AmmOpenOrders", pp.AmmOpenOrders},
		{"AmmTargetOrders", pp.AmmTargetOrders},
		{"SerumProgram", pp.SerumProgram},
		{"SerumMarket", pp.SerumMarket},
		{"SerumBids", pp.SerumBids},
		{"SerumAsks", pp.SerumAsks},
		{"SerumEventQueue", pp.SerumEventQueue},
		{"SerumCoinVaultAccount", pp.SerumCoinVaultAccount},
		{"SerumPcVaultAccount", pp.SerumPcVaultAccount},
		{"SerumVaultSigner", pp.SerumVaultSigner},
	}
	for _, item := range required {
		if item.account.IsZero() {
			return fmt.Errorf("Raydium AMM v4 requires %s; pass real market accounts from the AMM/market state", item.name)
		}
	}
	return nil
}

func raydiumAmmV4MintMatches(requested, expected solana.PublicKey) bool {
	return requested.Equals(expected) ||
		(expected.Equals(constants.WSOL_TOKEN_ACCOUNT) && requested.Equals(constants.SOL_TOKEN_ACCOUNT))
}

func ensureRaydiumAmmV4ExpectedMint(label string, requested, expected solana.PublicKey) error {
	if !requested.IsZero() && !raydiumAmmV4MintMatches(requested, expected) {
		return fmt.Errorf("%s must match the Raydium AMM v4 pool side (%s), got %s", label, expected.String(), requested.String())
	}
	return nil
}

// RaydiumAmmV4BuildBuyInstructions builds buy instructions for Raydium AMM V4
// 100% port from Rust: src/instruction/raydium_amm_v4.rs build_buy_instructions
func RaydiumAmmV4BuildBuyInstructions(params *RaydiumAmmV4BuildBuyParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	if err := ensureRaydiumAmmV4MarketAccounts(pp); err != nil {
		return nil, err
	}

	// Check if pool contains WSOL or USDC
	isWsol := pp.CoinMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.PcMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.CoinMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.PcMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Calculate swap amounts
	amountIn := params.InputAmount
	isBaseIn := pp.CoinMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.CoinMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		inputReserve := pp.PcReserve
		outputReserve := pp.CoinReserve
		if isBaseIn {
			inputReserve = pp.CoinReserve
			outputReserve = pp.PcReserve
		}
		result := calc.RaydiumAmmV4GetAmountOut(amountIn, inputReserve, outputReserve)
		minAmountOut, _ := calc.CalculateWithSlippageSell(result, params.SlippageBasisPoints)
		minimumAmountOut = minAmountOut
	}

	// Determine input/output mints
	inputMint := pp.PcMint
	if isBaseIn {
		inputMint = pp.CoinMint
	}
	outputMint := pp.CoinMint
	if isBaseIn {
		outputMint = pp.PcMint
	}
	if err := ensureRaydiumAmmV4ExpectedMint("OutputMint", params.OutputMint, outputMint); err != nil {
		return nil, err
	}

	// Get user token accounts
	userSourceTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, constants.TOKEN_PROGRAM)
	userDestinationTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 6)

	// Handle WSOL wrapping if needed
	if params.CreateInputMintAta {
		if inputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
			instructions = append(instructions, HandleWsol(params.Payer, amountIn)...)
		} else {
			instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
				params.Payer, params.Payer, inputMint, constants.TOKEN_PROGRAM,
			))
		}
	}

	// Create output ATA if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, constants.TOKEN_PROGRAM,
		))
	}

	// Build instruction data (17 bytes: 1 byte discriminator + 2x8 bytes amounts)
	data := make([]byte, 17)
	if params.FixedOutputAmount != nil {
		copy(data[0:1], RaydiumAmmV4SwapBaseOutDiscriminator)
	} else {
		copy(data[0:1], RaydiumAmmV4SwapBaseInDiscriminator)
	}
	binary.LittleEndian.PutUint64(data[1:9], amountIn)
	binary.LittleEndian.PutUint64(data[9:17], minimumAmountOut)

	// Build accounts array (18 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},    // 0: Token Program (readonly)
		{PublicKey: pp.Amm, IsSigner: false, IsWritable: true},                      // 1: Amm
		{PublicKey: RAYDIUM_AMM_V4_AUTHORITY, IsSigner: false, IsWritable: false},   // 2: Authority (readonly)
		{PublicKey: pp.AmmOpenOrders, IsSigner: false, IsWritable: true},            // 3: Amm Open Orders
		{PublicKey: pp.AmmTargetOrders, IsSigner: false, IsWritable: true},          // 4: Amm Target Orders
		{PublicKey: pp.TokenCoin, IsSigner: false, IsWritable: true},                // 5: Pool Coin Token Account
		{PublicKey: pp.TokenPc, IsSigner: false, IsWritable: true},                  // 6: Pool Pc Token Account
		{PublicKey: pp.SerumProgram, IsSigner: false, IsWritable: false},            // 7: Serum Program
		{PublicKey: pp.SerumMarket, IsSigner: false, IsWritable: true},              // 8: Serum Market
		{PublicKey: pp.SerumBids, IsSigner: false, IsWritable: true},                // 9: Serum Bids
		{PublicKey: pp.SerumAsks, IsSigner: false, IsWritable: true},                // 10: Serum Asks
		{PublicKey: pp.SerumEventQueue, IsSigner: false, IsWritable: true},          // 11: Serum Event Queue
		{PublicKey: pp.SerumCoinVaultAccount, IsSigner: false, IsWritable: true},    // 12: Serum Coin Vault Account
		{PublicKey: pp.SerumPcVaultAccount, IsSigner: false, IsWritable: true},      // 13: Serum Pc Vault Account
		{PublicKey: pp.SerumVaultSigner, IsSigner: false, IsWritable: false},        // 14: Serum Vault Signer
		{PublicKey: userSourceTokenAccount, IsSigner: false, IsWritable: true},      // 15: User Source Token Account
		{PublicKey: userDestinationTokenAccount, IsSigner: false, IsWritable: true}, // 16: User Destination Token Account
		{PublicKey: params.Payer, IsSigner: true, IsWritable: false},                // 17: User Source Owner
	}

	instructions = append(instructions, newInstruction(RAYDIUM_AMM_V4_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseInputMintAta && inputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	return instructions, nil
}

// RaydiumAmmV4BuildSellInstructions builds sell instructions for Raydium AMM V4
// 100% port from Rust: src/instruction/raydium_amm_v4.rs build_sell_instructions
func RaydiumAmmV4BuildSellInstructions(params *RaydiumAmmV4BuildSellParams) ([]solana.Instruction, error) {
	if params.InputAmount == 0 {
		return nil, ErrInvalidAmount
	}

	pp := params.ProtocolParams
	if err := ensureRaydiumAmmV4MarketAccounts(pp); err != nil {
		return nil, err
	}

	// Check if pool contains WSOL or USDC
	isWsol := pp.CoinMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.PcMint.Equals(constants.WSOL_TOKEN_ACCOUNT)
	isUsdc := pp.CoinMint.Equals(constants.USDC_TOKEN_ACCOUNT) || pp.PcMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	if !isWsol && !isUsdc {
		return nil, ErrInvalidPool
	}

	// Calculate swap amounts
	isBaseIn := pp.PcMint.Equals(constants.WSOL_TOKEN_ACCOUNT) || pp.PcMint.Equals(constants.USDC_TOKEN_ACCOUNT)
	var minimumAmountOut uint64
	if params.FixedOutputAmount != nil {
		minimumAmountOut = *params.FixedOutputAmount
	} else {
		inputReserve := pp.PcReserve
		outputReserve := pp.CoinReserve
		if isBaseIn {
			inputReserve = pp.CoinReserve
			outputReserve = pp.PcReserve
		}
		result := calc.RaydiumAmmV4GetAmountOut(params.InputAmount, inputReserve, outputReserve)
		minAmountOut, _ := calc.CalculateWithSlippageSell(result, params.SlippageBasisPoints)
		minimumAmountOut = minAmountOut
	}

	// Determine output mint
	outputMint := pp.CoinMint
	if isBaseIn {
		outputMint = pp.PcMint
	}
	if err := ensureRaydiumAmmV4ExpectedMint("OutputMint", params.OutputMint, outputMint); err != nil {
		return nil, err
	}
	inputMint := params.InputMint
	expectedInputMint := pp.PcMint
	if isBaseIn {
		expectedInputMint = pp.CoinMint
	}
	if err := ensureRaydiumAmmV4ExpectedMint("InputMint", inputMint, expectedInputMint); err != nil {
		return nil, err
	}
	inputMint = expectedInputMint

	// Get user token accounts
	userSourceTokenAccount := GetAssociatedTokenAddress(params.Payer, inputMint, constants.TOKEN_PROGRAM)
	userDestinationTokenAccount := GetAssociatedTokenAddress(params.Payer, outputMint, constants.TOKEN_PROGRAM)

	// Build instructions
	instructions := make([]solana.Instruction, 0, 3)

	// Create WSOL ATA for receiving if needed
	if params.CreateOutputMintAta {
		instructions = append(instructions, CreateAssociatedTokenAccountIdempotent(
			params.Payer, params.Payer, outputMint, constants.TOKEN_PROGRAM,
		))
	}

	// Build instruction data (17 bytes: 1 byte discriminator + 2x8 bytes amounts)
	data := make([]byte, 17)
	if params.FixedOutputAmount != nil {
		copy(data[0:1], RaydiumAmmV4SwapBaseOutDiscriminator)
	} else {
		copy(data[0:1], RaydiumAmmV4SwapBaseInDiscriminator)
	}
	binary.LittleEndian.PutUint64(data[1:9], params.InputAmount)
	binary.LittleEndian.PutUint64(data[9:17], minimumAmountOut)

	// Build accounts array (18 accounts)
	accounts := []solana.AccountMeta{
		{PublicKey: constants.TOKEN_PROGRAM, IsSigner: false, IsWritable: false},    // 0: Token Program (readonly)
		{PublicKey: pp.Amm, IsSigner: false, IsWritable: true},                      // 1: Amm
		{PublicKey: RAYDIUM_AMM_V4_AUTHORITY, IsSigner: false, IsWritable: false},   // 2: Authority (readonly)
		{PublicKey: pp.AmmOpenOrders, IsSigner: false, IsWritable: true},            // 3: Amm Open Orders
		{PublicKey: pp.AmmTargetOrders, IsSigner: false, IsWritable: true},          // 4: Amm Target Orders
		{PublicKey: pp.TokenCoin, IsSigner: false, IsWritable: true},                // 5: Pool Coin Token Account
		{PublicKey: pp.TokenPc, IsSigner: false, IsWritable: true},                  // 6: Pool Pc Token Account
		{PublicKey: pp.SerumProgram, IsSigner: false, IsWritable: false},            // 7: Serum Program
		{PublicKey: pp.SerumMarket, IsSigner: false, IsWritable: true},              // 8: Serum Market
		{PublicKey: pp.SerumBids, IsSigner: false, IsWritable: true},                // 9: Serum Bids
		{PublicKey: pp.SerumAsks, IsSigner: false, IsWritable: true},                // 10: Serum Asks
		{PublicKey: pp.SerumEventQueue, IsSigner: false, IsWritable: true},          // 11: Serum Event Queue
		{PublicKey: pp.SerumCoinVaultAccount, IsSigner: false, IsWritable: true},    // 12: Serum Coin Vault Account
		{PublicKey: pp.SerumPcVaultAccount, IsSigner: false, IsWritable: true},      // 13: Serum Pc Vault Account
		{PublicKey: pp.SerumVaultSigner, IsSigner: false, IsWritable: false},        // 14: Serum Vault Signer
		{PublicKey: userSourceTokenAccount, IsSigner: false, IsWritable: true},      // 15: User Source Token Account
		{PublicKey: userDestinationTokenAccount, IsSigner: false, IsWritable: true}, // 16: User Destination Token Account
		{PublicKey: params.Payer, IsSigner: true, IsWritable: false},                // 17: User Source Owner
	}

	instructions = append(instructions, newInstruction(RAYDIUM_AMM_V4_PROGRAM, accounts, data))

	// Close WSOL ATA if requested
	if params.CloseOutputMintAta && outputMint.Equals(constants.WSOL_TOKEN_ACCOUNT) {
		instructions = append(instructions, CloseWsol(params.Payer))
	}

	// Close input token account if requested
	if params.CloseInputMintAta {
		closeIx := token.NewCloseAccountInstruction(
			userSourceTokenAccount,
			params.Payer,
			params.Payer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, closeIx)
	}

	return instructions, nil
}

// Raydium AMM V4 error definitions
var (
	ErrRaydiumAmmV4InvalidPool = fmt.Errorf("raydium amm v4: invalid pool configuration")
)

// ===== AMM Info Decoder - from Rust: src/instruction/utils/raydium_amm_v4_types.rs =====

const AmmInfoSize = 752

// RaydiumAmmFees represents fee structure
type RaydiumAmmFees struct {
	MinSeparateNumerator   uint64
	MinSeparateDenominator uint64
	TradeFeeNumerator      uint64
	TradeFeeDenominator    uint64
	PnlNumerator           uint64
	PnlDenominator         uint64
	SwapFeeNumerator       uint64
	SwapFeeDenominator     uint64
}

// RaydiumAmmOutputData represents output data structure
type RaydiumAmmOutputData struct {
	NeedTakePnlCoin     uint64
	NeedTakePnlPc       uint64
	TotalPnlPc          uint64
	TotalPnlCoin        uint64
	PoolOpenTime        uint64
	PunishPcAmount      uint64
	PunishCoinAmount    uint64
	OrderbookToInitTime uint64
	SwapCoinInAmount    uint64
	SwapPcOutAmount     uint64
	SwapTakePcFee       uint64
	SwapPcInAmount      uint64
	SwapCoinOutAmount   uint64
	SwapTakeCoinFee     uint64
}

// RaydiumAmmInfo represents decoded AMM info
type RaydiumAmmInfo struct {
	Status             uint64
	Nonce              uint64
	OrderNum           uint64
	Depth              uint64
	CoinDecimals       uint64
	PcDecimals         uint64
	State              uint64
	ResetFlag          uint64
	MinSize            uint64
	VolMaxCutRatio     uint64
	AmountWave         uint64
	CoinLotSize        uint64
	PcLotSize          uint64
	MinPriceMultiplier uint64
	MaxPriceMultiplier uint64
	SysDecimalValue    uint64
	Fees               RaydiumAmmFees
	Output             RaydiumAmmOutputData
	TokenCoin          solana.PublicKey
	TokenPc            solana.PublicKey
	CoinMint           solana.PublicKey
	PcMint             solana.PublicKey
	LpMint             solana.PublicKey
	OpenOrders         solana.PublicKey
	Market             solana.PublicKey
	SerumDex           solana.PublicKey
	TargetOrders       solana.PublicKey
	WithdrawQueue      solana.PublicKey
	TokenTempLp        solana.PublicKey
	AmmOwner           solana.PublicKey
	LpAmount           uint64
	ClientOrderId      uint64
}

// DecodeAmmInfo decodes Raydium AMM v4 info from account data.
// 100% from Rust: src/instruction/utils/raydium_amm_v4_types.rs amm_info_decode
func DecodeAmmInfo(data []byte) *RaydiumAmmInfo {
	if len(data) < AmmInfoSize {
		return nil
	}

	info := &RaydiumAmmInfo{}
	offset := 0

	readU64 := func() uint64 {
		val := binary.LittleEndian.Uint64(data[offset:])
		offset += 8
		return val
	}

	// status: u64
	info.Status = readU64()
	// nonce: u64
	info.Nonce = readU64()
	// order_num: u64
	info.OrderNum = readU64()
	// depth: u64
	info.Depth = readU64()
	// coin_decimals: u64
	info.CoinDecimals = readU64()
	// pc_decimals: u64
	info.PcDecimals = readU64()
	// state: u64
	info.State = readU64()
	// reset_flag: u64
	info.ResetFlag = readU64()
	// min_size: u64
	info.MinSize = readU64()
	// vol_max_cut_ratio: u64
	info.VolMaxCutRatio = readU64()
	// amount_wave: u64
	info.AmountWave = readU64()
	// coin_lot_size: u64
	info.CoinLotSize = readU64()
	// pc_lot_size: u64
	info.PcLotSize = readU64()
	// min_price_multiplier: u64
	info.MinPriceMultiplier = readU64()
	// max_price_multiplier: u64
	info.MaxPriceMultiplier = readU64()
	// sys_decimal_value: u64
	info.SysDecimalValue = readU64()

	// fees: Fees (8 * u64)
	info.Fees = RaydiumAmmFees{
		MinSeparateNumerator:   readU64(),
		MinSeparateDenominator: readU64(),
		TradeFeeNumerator:      readU64(),
		TradeFeeDenominator:    readU64(),
		PnlNumerator:           readU64(),
		PnlDenominator:         readU64(),
		SwapFeeNumerator:       readU64(),
		SwapFeeDenominator:     readU64(),
	}

	// output: OutPutData
	info.Output = RaydiumAmmOutputData{
		NeedTakePnlCoin:     readU64(),
		NeedTakePnlPc:       readU64(),
		TotalPnlPc:          readU64(),
		TotalPnlCoin:        readU64(),
		PoolOpenTime:        readU64(),
		PunishPcAmount:      readU64(),
		PunishCoinAmount:    readU64(),
		OrderbookToInitTime: readU64(),
		SwapCoinInAmount:    readU64(),
		SwapPcOutAmount:     readU64(),
		SwapTakePcFee:       readU64(),
		SwapPcInAmount:      readU64(),
		SwapCoinOutAmount:   readU64(),
		SwapTakeCoinFee:     readU64(),
	}

	// token_coin: Pubkey
	copy(info.TokenCoin[:], data[offset:offset+32])
	offset += 32

	// token_pc: Pubkey
	copy(info.TokenPc[:], data[offset:offset+32])
	offset += 32

	// coin_mint: Pubkey
	copy(info.CoinMint[:], data[offset:offset+32])
	offset += 32

	// pc_mint: Pubkey
	copy(info.PcMint[:], data[offset:offset+32])
	offset += 32

	// lp_mint: Pubkey
	copy(info.LpMint[:], data[offset:offset+32])
	offset += 32

	// open_orders: Pubkey
	copy(info.OpenOrders[:], data[offset:offset+32])
	offset += 32

	// market: Pubkey
	copy(info.Market[:], data[offset:offset+32])
	offset += 32

	// serum_dex: Pubkey
	copy(info.SerumDex[:], data[offset:offset+32])
	offset += 32

	// target_orders: Pubkey
	copy(info.TargetOrders[:], data[offset:offset+32])
	offset += 32

	// withdraw_queue: Pubkey
	copy(info.WithdrawQueue[:], data[offset:offset+32])
	offset += 32

	// token_temp_lp: Pubkey
	copy(info.TokenTempLp[:], data[offset:offset+32])
	offset += 32

	// amm_owner: Pubkey
	copy(info.AmmOwner[:], data[offset:offset+32])
	offset += 32

	// lp_amount: u64
	info.LpAmount = readU64()

	// client_order_id: u64
	info.ClientOrderId = readU64()

	return info
}

// ===== Async Fetch Functions - from Rust: src/instruction/utils/raydium_amm_v4.rs =====

// AmmInfoFetcher defines interface for fetching AMM info from RPC
type AmmInfoFetcher interface {
	GetAccountInfo(pubkey solana.PublicKey) ([]byte, error)
}

// FetchAmmInfo fetches AMM info from RPC.
// 100% from Rust: src/instruction/utils/raydium_amm_v4.rs fetch_amm_info
func FetchAmmInfo(fetcher AmmInfoFetcher, amm solana.PublicKey) (*RaydiumAmmInfo, error) {
	data, err := fetcher.GetAccountInfo(amm)
	if err != nil {
		return nil, err
	}
	if len(data) < AmmInfoSize {
		return nil, fmt.Errorf("account data too short")
	}
	info := DecodeAmmInfo(data)
	if info == nil {
		return nil, fmt.Errorf("failed to decode amm info")
	}
	return info, nil
}
