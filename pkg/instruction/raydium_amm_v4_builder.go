// Raydium AMM V4 instruction builder - Production-grade implementation
// 100% port from Rust sol-trade-sdk

package instruction

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"

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
	RaydiumAmmV4SwapBaseInV2Discriminator  = []byte{16}
	RaydiumAmmV4SwapBaseOutV2Discriminator = []byte{17}
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
	Amm                                  solana.PublicKey
	AmmOpenOrders                        solana.PublicKey
	AmmTargetOrders                      solana.PublicKey
	TokenCoin                            solana.PublicKey
	TokenPc                              solana.PublicKey
	SerumProgram                         solana.PublicKey
	SerumMarket                          solana.PublicKey
	SerumBids                            solana.PublicKey
	SerumAsks                            solana.PublicKey
	SerumEventQueue                      solana.PublicKey
	SerumCoinVaultAccount                solana.PublicKey
	SerumPcVaultAccount                  solana.PublicKey
	SerumVaultSigner                     solana.PublicKey
	CoinMint                             solana.PublicKey
	PcMint                               solana.PublicKey
	CoinReserve                          uint64
	PcReserve                            uint64
	SwapFeeNumerator, SwapFeeDenominator *uint64
}

// RaydiumAmmV4BuildBuyParams contains parameters for building buy instructions
type RaydiumAmmV4BuildBuyParams struct {
	Payer               solana.PublicKey
	OutputMint          solana.PublicKey
	InputMint           solana.PublicKey
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

func ammV4Pair(pp *RaydiumAmmV4Params, mint solana.PublicKey, buy bool) (solana.PublicKey, solana.PublicKey, bool, error) {
	if pp == nil || pp.CoinMint == pp.PcMint || pp.Amm.IsZero() || pp.CoinMint.IsZero() || pp.PcMint.IsZero() || pp.TokenCoin.IsZero() || pp.TokenPc.IsZero() {
		return solana.PublicKey{}, solana.PublicKey{}, false, ErrInvalidPool
	}
	if mint == constants.SOL_TOKEN_ACCOUNT {
		mint = constants.WSOL_TOKEN_ACCOUNT
	}
	if mint != pp.CoinMint && mint != pp.PcMint {
		return solana.PublicKey{}, solana.PublicKey{}, false, fmt.Errorf("input/output mint must match the Raydium AMM v4 pool side")
	}
	coinIn := mint == pp.CoinMint
	if buy {
		coinIn = mint == pp.PcMint
	}
	if coinIn {
		return pp.CoinMint, pp.PcMint, true, nil
	}
	return pp.PcMint, pp.CoinMint, false, nil
}
func ammV4Swap(pp *RaydiumAmmV4Params, payer, im, om solana.PublicKey, amount, slippage uint64, coinIn bool, fixed *uint64) (solana.Instruction, error) {
	if amount == 0 {
		return nil, ErrInvalidAmount
	}
	var minimum uint64
	if fixed != nil {
		if *fixed == 0 {
			return nil, ErrInvalidAmount
		}
		minimum = *fixed
	} else {
		numerator, denominator := uint64(25), uint64(10000)
		if pp.SwapFeeNumerator != nil {
			numerator = *pp.SwapFeeNumerator
		}
		if pp.SwapFeeDenominator != nil {
			denominator = *pp.SwapFeeDenominator
		}
		if denominator == 0 || numerator >= denominator || pp.CoinReserve == 0 || pp.PcReserve == 0 {
			return nil, fmt.Errorf("invalid AMM v4 reserves or swap fee")
		}
		n := func(a uint64) *big.Int { return new(big.Int).SetUint64(a) }
		fee := new(big.Int).Quo(new(big.Int).Add(new(big.Int).Mul(n(amount), n(numerator)), n(denominator-1)), n(denominator)).Uint64()
		net := amount - fee
		i, o := pp.CoinReserve, pp.PcReserve
		if !coinIn {
			i, o = o, i
		}
		out := new(big.Int).Quo(new(big.Int).Mul(n(o), n(net)), new(big.Int).Add(n(i), n(net))).Uint64()
		if slippage > 9999 {
			slippage = 9999
		}
		minimum = new(big.Int).Quo(new(big.Int).Mul(n(out), n(10000-slippage)), n(10000)).Uint64()
	}
	data := make([]byte, 17)
	data[0] = 16
	if fixed != nil {
		data[0] = 17
	}
	binary.LittleEndian.PutUint64(data[1:], amount)
	binary.LittleEndian.PutUint64(data[9:], minimum)
	keys := []solana.PublicKey{constants.TOKEN_PROGRAM, pp.Amm, RAYDIUM_AMM_V4_AUTHORITY, pp.TokenCoin, pp.TokenPc, GetAssociatedTokenAddress(payer, im, constants.TOKEN_PROGRAM), GetAssociatedTokenAddress(payer, om, constants.TOKEN_PROGRAM), payer}
	metas := []solana.AccountMeta{}
	for i, k := range keys {
		metas = append(metas, solana.AccountMeta{PublicKey: k, IsSigner: i == 7, IsWritable: i == 1 || i >= 3 && i <= 6})
	}
	return newInstruction(RAYDIUM_AMM_V4_PROGRAM, metas, data), nil
}

// Independent V2 buy. Supplied reserves must already exclude pending PnL; no RPC.
func RaydiumAmmV4BuildBuyInstructions(p *RaydiumAmmV4BuildBuyParams) ([]solana.Instruction, error) {
	if p == nil {
		return nil, ErrInvalidAmount
	}
	im, om, coinIn, e := ammV4Pair(p.ProtocolParams, p.OutputMint, true)
	if e != nil {
		return nil, e
	}
	if !p.InputMint.IsZero() && p.InputMint != im && !(p.InputMint == constants.SOL_TOKEN_ACCOUNT && im == constants.WSOL_TOKEN_ACCOUNT) {
		return nil, fmt.Errorf("InputMint must match the Raydium AMM v4 pool side")
	}
	swap, e := ammV4Swap(p.ProtocolParams, p.Payer, im, om, p.InputAmount, p.SlippageBasisPoints, coinIn, p.FixedOutputAmount)
	if e != nil {
		return nil, e
	}
	result := []solana.Instruction{}
	if p.CreateInputMintAta {
		if im == constants.WSOL_TOKEN_ACCOUNT {
			result = append(result, HandleWsol(p.Payer, p.InputAmount)...)
		} else {
			result = append(result, CreateAssociatedTokenAccountIdempotent(p.Payer, p.Payer, im, constants.TOKEN_PROGRAM))
		}
	}
	if p.CreateOutputMintAta {
		result = append(result, CreateAssociatedTokenAccountIdempotent(p.Payer, p.Payer, om, constants.TOKEN_PROGRAM))
	}
	result = append(result, swap)
	if p.CloseInputMintAta && im == constants.WSOL_TOKEN_ACCOUNT {
		result = append(result, CloseWsol(p.Payer))
	}
	return result, nil
}

// Independent V2 sell for either side, including arbitrary stock/token pairs.
func RaydiumAmmV4BuildSellInstructions(p *RaydiumAmmV4BuildSellParams) ([]solana.Instruction, error) {
	if p == nil {
		return nil, ErrInvalidAmount
	}
	im, om, coinIn, e := ammV4Pair(p.ProtocolParams, p.InputMint, false)
	if e != nil {
		return nil, e
	}
	if !p.OutputMint.IsZero() && p.OutputMint != om && !(p.OutputMint == constants.SOL_TOKEN_ACCOUNT && om == constants.WSOL_TOKEN_ACCOUNT) {
		return nil, fmt.Errorf("OutputMint must match the Raydium AMM v4 pool side")
	}
	swap, e := ammV4Swap(p.ProtocolParams, p.Payer, im, om, p.InputAmount, p.SlippageBasisPoints, coinIn, p.FixedOutputAmount)
	if e != nil {
		return nil, e
	}
	result := []solana.Instruction{}
	if p.CreateOutputMintAta {
		result = append(result, CreateAssociatedTokenAccountIdempotent(p.Payer, p.Payer, om, constants.TOKEN_PROGRAM))
	}
	result = append(result, swap)
	if p.CloseOutputMintAta && om == constants.WSOL_TOKEN_ACCOUNT {
		result = append(result, CloseWsol(p.Payer))
	}
	if p.CloseInputMintAta {
		result = append(result, token.NewCloseAccountInstruction(GetAssociatedTokenAddress(p.Payer, im, constants.TOKEN_PROGRAM), p.Payer, p.Payer, nil).Build())
	}
	return result, nil
}

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
	SwapCoinInAmount    *big.Int
	SwapPcOutAmount     *big.Int
	SwapTakePcFee       uint64
	SwapPcInAmount      *big.Int
	SwapCoinOutAmount   *big.Int
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

	readU128 := func() *big.Int {
		raw := append([]byte{}, data[offset:offset+16]...)
		for i, j := 0, 15; i < j; i, j = i+1, j-1 {
			raw[i], raw[j] = raw[j], raw[i]
		}
		offset += 16
		return new(big.Int).SetBytes(raw)
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
		SwapCoinInAmount:    readU128(),
		SwapPcOutAmount:     readU128(),
		SwapTakePcFee:       readU64(),
		SwapPcInAmount:      readU128(),
		SwapCoinOutAmount:   readU128(),
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
