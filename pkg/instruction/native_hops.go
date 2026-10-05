package instruction

// Direct conversion instructions. State and quoted amounts are caller-supplied.
import (
	"encoding/binary"
	"errors"
	"github.com/gagliardetto/solana-go"
	"math/big"
)

var hopMemo = solana.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr")
var hopCLMM = solana.MustPublicKeyFromBase58("CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK")
var hopWhirlpool = solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")
var hopDLMM = solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")
var hopDlmmEvent = solana.MustPublicKeyFromBase58("D1ZN9Wj1fRSUQfCjhvnu1hqDMT7hzjzBBpi12nVniYD6")

type SwapV2Args struct {
	Amount, OtherAmountThreshold uint64
	SqrtPriceLimit               *big.Int
	AmountSpecifiedIsInput       bool
}

func hopDataV2(args SwapV2Args) ([]byte, error) {
	limit := args.SqrtPriceLimit
	if limit == nil {
		limit = new(big.Int)
	}
	if limit.Sign() < 0 || limit.BitLen() > 128 {
		return nil, errors.New("sqrt price outside u128")
	}
	d := make([]byte, 41)
	copy(d, []byte{43, 4, 237, 11, 26, 201, 30, 98})
	binary.LittleEndian.PutUint64(d[8:], args.Amount)
	binary.LittleEndian.PutUint64(d[16:], args.OtherAmountThreshold)
	b := limit.Bytes()
	for i, v := range b {
		d[24+len(b)-i-1] = v
	}
	if args.AmountSpecifiedIsInput {
		d[40] = 1
	}
	return d, nil
}
func hopInstruction(program solana.PublicKey, data []byte, keys []solana.PublicKey, signer int, writable func(int) bool) solana.Instruction {
	metas := solana.AccountMetaSlice{}
	for i, k := range keys {
		metas = append(metas, &solana.AccountMeta{PublicKey: k, IsSigner: i == signer, IsWritable: writable(i)})
	}
	return solana.NewInstruction(program, metas, data)
}

type RaydiumClmmSwapV2Accounts struct {
	Payer, AmmConfig, PoolState, InputTokenAccount, OutputTokenAccount, InputVault, OutputVault, ObservationState, TokenProgram, TokenProgram2022, InputVaultMint, OutputVaultMint solana.PublicKey
	TickArrayBitmapExtension                                                                                                                                                       *solana.PublicKey
	TickArrays                                                                                                                                                                     []solana.PublicKey
}

func BuildRaydiumClmmSwapV2(a RaydiumClmmSwapV2Accounts, args SwapV2Args) (solana.Instruction, error) {
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("pool_tick_array_bitmap_extension"), a.PoolState[:]}, hopCLMM)
	if err != nil {
		return nil, err
	}
	if a.TickArrayBitmapExtension != nil && *a.TickArrayBitmapExtension != pda {
		return nil, errors.New("CLMM bitmap identity mismatch")
	}
	bitmap := a.TickArrayBitmapExtension != nil && *a.TickArrayBitmapExtension == pda
	ticks := []solana.PublicKey{}
	for _, k := range a.TickArrays {
		if k == pda {
			bitmap = true
		} else {
			ticks = append(ticks, k)
		}
	}
	if len(ticks) == 0 {
		return nil, errors.New("CLMM requires at least one tick array")
	}
	keys := []solana.PublicKey{a.Payer, a.AmmConfig, a.PoolState, a.InputTokenAccount, a.OutputTokenAccount, a.InputVault, a.OutputVault, a.ObservationState, a.TokenProgram, a.TokenProgram2022, hopMemo, a.InputVaultMint, a.OutputVaultMint}
	if bitmap {
		keys = append(keys, pda)
	}
	keys = append(keys, ticks...)
	d, err := hopDataV2(args)
	if err != nil {
		return nil, err
	}
	return hopInstruction(hopCLMM, d, keys, 0, func(i int) bool { return (i >= 2 && i <= 7) || i >= 13 }), nil
}

type WhirlpoolSwapV2Accounts struct {
	TokenProgramA, TokenProgramB, TokenAuthority, Whirlpool, MintA, MintB, OwnerA, VaultA, OwnerB, VaultB solana.PublicKey
	TickArrays                                                                                            []solana.PublicKey
}

func BuildWhirlpoolSwapV2(a WhirlpoolSwapV2Accounts, args SwapV2Args, aToB bool) (solana.Instruction, error) {
	if len(a.TickArrays) < 3 || len(a.TickArrays) > 6 {
		return nil, errors.New("Whirlpool requires 3..6 tick arrays")
	}
	oracle, _, err := solana.FindProgramAddress([][]byte{[]byte("oracle"), a.Whirlpool[:]}, hopWhirlpool)
	if err != nil {
		return nil, err
	}
	keys := []solana.PublicKey{a.TokenProgramA, a.TokenProgramB, hopMemo, a.TokenAuthority, a.Whirlpool, a.MintA, a.MintB, a.OwnerA, a.VaultA, a.OwnerB, a.VaultB}
	keys = append(keys, a.TickArrays[:3]...)
	keys = append(keys, oracle)
	keys = append(keys, a.TickArrays[3:]...)
	if args.SqrtPriceLimit == nil || args.SqrtPriceLimit.Sign() == 0 {
		limit := "79226673515401279992447579055"
		if aToB {
			limit = "4295048016"
		}
		args.SqrtPriceLimit, _ = new(big.Int).SetString(limit, 10)
	}
	d, err := hopDataV2(args)
	if err != nil {
		return nil, err
	}
	direction := byte(0)
	if aToB {
		direction = 1
	}
	d = append(d, direction)
	if len(a.TickArrays) > 3 {
		d = append(d, 1, 1, 0, 0, 0, 6, byte(len(a.TickArrays)-3))
	} else {
		d = append(d, 0)
	}
	return hopInstruction(hopWhirlpool, d, keys, 3, func(i int) bool { return i == 4 || i >= 7 }), nil
}

type MeteoraDlmmSwap2Accounts struct {
	LbPair, ReserveX, ReserveY, UserTokenIn, UserTokenOut, TokenXMint, TokenYMint, Oracle, User, TokenXProgram, TokenYProgram solana.PublicKey
	BitmapExtension                                                                                                           *solana.PublicKey
	BinArrays                                                                                                                 []solana.PublicKey
}

func BuildMeteoraDlmmSwap2(a MeteoraDlmmSwap2Accounts, amountIn, minOut uint64) (solana.Instruction, error) {
	if len(a.BinArrays) == 0 {
		return nil, errors.New("DLMM requires at least one bin array")
	}
	bitmap := hopDLMM
	if a.BitmapExtension != nil {
		bitmap = *a.BitmapExtension
	}
	keys := []solana.PublicKey{a.LbPair, bitmap, a.ReserveX, a.ReserveY, a.UserTokenIn, a.UserTokenOut, a.TokenXMint, a.TokenYMint, a.Oracle, hopDLMM, a.User, a.TokenXProgram, a.TokenYProgram, hopMemo, hopDlmmEvent, hopDLMM}
	keys = append(keys, a.BinArrays...)
	d := make([]byte, 28)
	copy(d, []byte{65, 75, 63, 76, 235, 91, 91, 136})
	binary.LittleEndian.PutUint64(d[8:], amountIn)
	binary.LittleEndian.PutUint64(d[16:], minOut)
	return hopInstruction(hopDLMM, d, keys, 10, func(i int) bool {
		return i == 0 || (i == 1 && a.BitmapExtension != nil) || (i >= 2 && i <= 5) || i == 8 || i >= 16
	}), nil
}
