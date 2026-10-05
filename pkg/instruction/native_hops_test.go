package instruction

import (
	"encoding/json"
	"github.com/gagliardetto/solana-go"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeHopsRustGolden(t *testing.T) {
	data, e := os.ReadFile("testdata/hops_rust_5_0_6.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []struct {
			Name, Program string
			Data          []int
			Accounts      []struct {
				Pubkey     string `json:"pubkey"`
				IsSigner   bool   `json:"is_signer"`
				IsWritable bool   `json:"is_writable"`
			}
		}
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	pk := func(n byte) solana.PublicKey {
		var k solana.PublicKey
		for i := range k {
			k[i] = n
		}
		return k
	}
	args := SwapV2Args{Amount: 1000, OtherAmountThreshold: 500, AmountSpecifiedIsInput: true}
	for _, c := range corpus.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var ix solana.Instruction
			var err error
			if strings.HasPrefix(c.Name, "clmm") {
				pda, _, _ := solana.FindProgramAddress([][]byte{[]byte("pool_tick_array_bitmap_extension"), pk(3).Bytes()}, hopCLMM)
				ticks := []solana.PublicKey{pk(13), pk(14)}
				if strings.Contains(c.Name, "bitmap") {
					ticks = []solana.PublicKey{pk(13), pda, pk(14)}
				}
				a := RaydiumClmmSwapV2Accounts{Payer: pk(1), AmmConfig: pk(2), PoolState: pk(3), InputTokenAccount: pk(4), OutputTokenAccount: pk(5), InputVault: pk(6), OutputVault: pk(7), ObservationState: pk(8), TokenProgram: pk(9), TokenProgram2022: pk(10), InputVaultMint: pk(11), OutputVaultMint: pk(12), TickArrays: ticks}
				clArgs := args
				clArgs.SqrtPriceLimit = big.NewInt(4295048017)
				ix, err = BuildRaydiumClmmSwapV2(a, clArgs)
			} else if strings.HasPrefix(c.Name, "whirlpool") {
				parts := strings.Split(c.Name, "_")
				ticks := []solana.PublicKey{pk(11), pk(12), pk(13)}
				if parts[1] == "true" {
					ticks = append(ticks, pk(14))
				}
				a := WhirlpoolSwapV2Accounts{pk(1), pk(2), pk(3), pk(4), pk(5), pk(6), pk(7), pk(8), pk(9), pk(10), ticks}
				ix, err = BuildWhirlpoolSwapV2(a, args, parts[2] == "true")
			} else {
				a := MeteoraDlmmSwap2Accounts{LbPair: pk(1), ReserveX: pk(2), ReserveY: pk(3), UserTokenIn: pk(4), UserTokenOut: pk(5), TokenXMint: pk(6), TokenYMint: pk(7), Oracle: pk(8), User: pk(9), TokenXProgram: pk(10), TokenYProgram: pk(11), BinArrays: []solana.PublicKey{pk(13), pk(14)}}
				if strings.Contains(c.Name, "bitmap") {
					bitmap := pk(12)
					a.BitmapExtension = &bitmap
				}
				ix, err = BuildMeteoraDlmmSwap2(a, 1000, 500)
			}
			if err != nil {
				t.Fatal(err)
			}
			if ix.ProgramID().String() != c.Program {
				t.Fatal("program mismatch")
			}
			payload, err := ix.Data()
			if err != nil {
				t.Fatal(err)
			}
			got := []int{}
			for _, b := range payload {
				got = append(got, int(b))
			}
			if !reflect.DeepEqual(got, c.Data) {
				t.Fatal("data differs from Rust")
			}
			accounts := ix.Accounts()
			if len(accounts) != len(c.Accounts) {
				t.Fatal("account count differs")
			}
			for i, a := range accounts {
				want := c.Accounts[i]
				if a.PublicKey.String() != want.Pubkey || a.IsSigner != want.IsSigner || a.IsWritable != want.IsWritable {
					t.Fatalf("account %d differs from Rust", i)
				}
			}
		})
	}
}
