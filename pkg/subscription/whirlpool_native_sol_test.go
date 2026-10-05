package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/calc"
	"github.com/gagliardetto/solana-go"
	"math/big"
	"os"
	"strconv"
	"testing"
)

func whirlDynamic(d []byte) []byte {
	b := make([]byte, 60)
	copy(b, []byte{17, 216, 246, 142, 225, 199, 218, 56})
	copy(b[8:12], d[8:12])
	copy(b[12:44], d[9956:9988])
	payload := []byte{}
	for i := 0; i < 88; i++ {
		o := 12 + i*113
		tag := d[o]
		payload = append(payload, tag)
		if tag != 0 {
			b[44+i/8] |= 1 << uint(i%8)
			payload = append(payload, d[o+1:o+113]...)
		}
	}
	return append(b, payload...)
}
func whirlPrepare(t *testing.T, v clmmFixture, reverse bool, age uint64, budget int) (uint64, error) {
	im, om := v.InputMint, v.OutputMint
	if reverse {
		im, om = om, im
	}
	a, q, _, e := clmmSnapshot(t, v).PrepareWhirlpool(PoolTradeHint{solana.MustPublicKeyFromBase58(v.Pool), solana.MustPublicKeyFromBase58(im), solana.MustPublicKeyFromBase58(om)}, CacheReadContext{clmmNumber(t, v.ReadSlot) + age, clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), solana.MustPublicKeyFromBase58(v.Payer), clmmNumber(t, v.Amount), 100, budget)
	if e == nil && (len(a.TickArrays) != 3 || a.TickArrays[0] != a.TickArrays[2] || q.MinimumAmountOut != q.EstimatedNetAmountOut*99/100) {
		t.Fatal("incorrect Whirlpool array padding/minimum")
	}
	return q.EstimatedNetAmountOut, e
}
func TestCachedWhirlpoolLayouts(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, dynamic := range []bool{false, true} {
			v := clmmLoad(t, "whirlpool")
			key := "usdc_to_wsol"
			v.Amount = "10000"
			if reverse {
				v.Amount = "1000000"
				key = "wsol_to_usdc"
			}
			if dynamic {
				for i, a := range v.Accounts {
					d, _ := base64.StdEncoding.DecodeString(a.Data)
					if len(d) >= 8 && bytes.Equal(d[:8], []byte{69, 97, 189, 190, 110, 7, 66, 187}) {
						v.Accounts[i].Data = base64.StdEncoding.EncodeToString(whirlDynamic(d))
					}
				}
			}
			q, e := whirlPrepare(t, v, reverse, 0, 6)
			if e != nil {
				t.Fatal(e)
			}
			var want string
			json.Unmarshal(v.Expected[key], &want)
			if strconv.FormatUint(q, 10) != want {
				t.Fatal(q, want)
			}
		}
	}
}
func TestCachedWhirlpoolExecutionReplay(t *testing.T) {
	d, e := os.ReadFile("../../examples/fixtures/whirlpool_execution_replays_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Name   string
		Pre    string `json:"pre_sqrt_price"`
		Amount string `json:"amount_in"`
		Down   bool
		Output string `json:"actual_output"`
	}
	if e = json.Unmarshal(d, &cases); e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			v := clmmLoad(t, "whirlpool")
			v.Amount = c.Amount
			p, _ := new(big.Int).SetString(c.Pre, 10)
			tick, e := calc.WhirlpoolTickAtSqrtPrice(p)
			if e != nil {
				t.Fatal(e)
			}
			for i, a := range v.Accounts {
				if a.Pubkey == v.Pool {
					b, _ := base64.StdEncoding.DecodeString(a.Data)
					wide := p.Bytes()
					for j := 0; j < 16; j++ {
						b[65+j] = 0
					}
					for j, x := range wide {
						b[65+len(wide)-1-j] = x
					}
					binary.LittleEndian.PutUint32(b[81:], uint32(tick))
					v.Accounts[i].Data = base64.StdEncoding.EncodeToString(b)
				}
			}
			q, e := whirlPrepare(t, v, c.Down, 0, 6)
			if e != nil || strconv.FormatUint(q, 10) != c.Output {
				t.Fatal(q, c.Output, e)
			}
		})
	}
}
func TestCachedWhirlpoolInvalid(t *testing.T) {
	for _, kind := range []string{"stale", "owner", "missing", "pool_identity", "tag", "bitmap", "closed", "budget"} {
		t.Run(kind, func(t *testing.T) {
			v := clmmLoad(t, "whirlpool")
			age := uint64(0)
			budget := 6
			if kind == "stale" {
				age = 1
			}
			if kind == "budget" {
				budget = 0
			}
			kept := []clmmFixtureAccount{}
			for _, a := range v.Accounts {
				d, _ := base64.StdEncoding.DecodeString(a.Data)
				if a.Pubkey == v.Pool {
					if kind == "owner" {
						a.Owner = solana.PublicKey{}.String()
					}
					if kind == "closed" {
						d = nil
					}
				}
				if len(d) >= 8 && bytes.Equal(d[:8], []byte{69, 97, 189, 190, 110, 7, 66, 187}) {
					if kind == "missing" {
						continue
					}
					if kind == "pool_identity" {
						copy(d[9956:9988], make([]byte, 32))
					}
					if kind == "tag" {
						d[12] = 2
					}
					if kind == "bitmap" {
						d = whirlDynamic(d)
						d[44] ^= 1
					}
				}
				a.Data = base64.StdEncoding.EncodeToString(d)
				kept = append(kept, a)
			}
			v.Accounts = kept
			_, e := whirlPrepare(t, v, false, age, budget)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func solRoute(t *testing.T, direction string) (clmmFixture, PreparedCachedRoute, solana.PublicKey, string, uint64) {
	v := clmmLoad(t, "sol_route_"+direction)
	data, e := os.ReadFile("../../examples/fixtures/sol_route_" + direction + "_mainnet_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var extra struct {
		Seed string `json:"temporary_wsol_seed"`
		Rent string `json:"rent_lamports"`
	}
	json.Unmarshal(data, &extra)
	payer := solana.MustPublicKeyFromBase58(v.Payer)
	hints := []PoolTradeHint{}
	for _, h := range v.Legs {
		hints = append(hints, PoolTradeHint{solana.MustPublicKeyFromBase58(h.Pool), solana.MustPublicKeyFromBase58(h.InputMint), solana.MustPublicKeyFromBase58(h.OutputMint)})
	}
	r, e := clmmSnapshot(t, v).PrepareRoute(hints, CacheReadContext{clmmNumber(t, v.ReadSlot), clmmNumber(t, v.Epoch), 0}, clmmNumber(t, v.UnixTimestamp), payer, clmmNumber(t, v.Amount), 100, 8)
	if e != nil {
		t.Fatal(e)
	}
	return v, r, payer, extra.Seed, clmmNumber(t, extra.Rent)
}
func TestNativeSolRoutes(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		t.Run(direction, func(t *testing.T) {
			v, r, payer, seed, rent := solRoute(t, direction)
			old, _, _ := solana.FindProgramAddress([][]byte{payer[:], solana.TokenProgramID[:], routeWsolMint[:]}, solana.SPLAssociatedTokenAccountProgramID)
			before := []solana.PublicKey{}
			for _, ix := range r.SwapInstructions {
				for _, a := range ix.Accounts() {
					before = append(before, a.PublicKey)
				}
			}
			n, e := SettleCachedRouteWithNativeSol(r, payer, seed, rent, direction == "buy", direction == "sell")
			if e != nil {
				t.Fatal(e)
			}
			want := rent
			if direction == "buy" {
				want += r.Legs[0].AmountIn
			}
			if n.RequiredLamports != want || n.TemporaryWsolAccount == old || len(r.Legs) != 3 {
				t.Fatal(n)
			}
			for _, ix := range n.Instructions {
				for _, a := range ix.Accounts() {
					if a.PublicKey == old {
						t.Fatal("existing WSOL ATA changed")
					}
				}
			}
			last := n.Instructions[len(n.Instructions)-1]
			d, _ := last.Data()
			if !bytes.Equal(d, []byte{9}) || last.Accounts()[0].PublicKey != n.TemporaryWsolAccount || !n.Instructions[0].Accounts()[0].IsSigner {
				t.Fatal("temporary close/signature")
			}
			d, _ = n.Instructions[1].Data()
			if !bytes.Equal(d, append([]byte{18}, payer[:]...)) {
				t.Fatal("initialize")
			}
			d, _ = n.Instructions[2].Data()
			if !bytes.Equal(d, []byte{17}) {
				t.Fatal("sync")
			}
			var minimum string
			json.Unmarshal(v.Expected["minimum_amount_out"], &minimum)
			if strconv.FormatUint(r.MinimumNetAmountOut, 10) != minimum {
				t.Fatal("minimum")
			}
			for i := 1; i < 3; i++ {
				if r.Legs[i].AmountIn > r.Legs[i-1].MinimumNetAmountOut {
					t.Fatal("overfunded leg")
				}
			}
			j := 0
			found := false
			for _, ix := range r.SwapInstructions {
				d, _ := ix.Data()
				if ix.ProgramID() == solana.TokenProgramID && bytes.Equal(d, []byte{9}) {
					t.Fatal("plain WSOL closes account")
				}
				for _, a := range ix.Accounts() {
					if a.PublicKey != before[j] {
						t.Fatal("mutated input route")
					}
					found = found || a.PublicKey == old
					j++
				}
			}
			if !found {
				t.Fatal("plain WSOL missing ATA")
			}
		})
	}
}
func TestNativeSolInvalid(t *testing.T) {
	for _, kind := range []string{"empty_seed", "long_utf8", "utf8", "rent", "overflow", "wrong_wallet", "both", "neither", "wrong_asset"} {
		t.Run(kind, func(t *testing.T) {
			_, r, payer, seed, rent := solRoute(t, "buy")
			input, output := true, false
			switch kind {
			case "empty_seed":
				seed = ""
			case "long_utf8":
				seed = "界界界界界界界界界界界"
			case "utf8":
				seed = string([]byte{255})
			case "rent":
				rent = 0
			case "overflow":
				rent = ^uint64(0)
			case "wrong_wallet":
				payer = solana.PublicKey{}
			case "both":
				output = true
			case "neither":
				input = false
			case "wrong_asset":
				input = false
				output = true
			}
			_, e := SettleCachedRouteWithNativeSol(r, payer, seed, rent, input, output)
			if e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestNativeSolRejectsModifiedBundle(t *testing.T) {
	for _, kind := range []string{"count", "swap", "protection", "credit", "zero", "nil_swap", "nil_setup", "nil_account", "typed_nil_swap", "typed_nil_setup"} {
		t.Run(kind, func(t *testing.T) {
			_, r, payer, seed, rent := solRoute(t, "buy")
			switch kind {
			case "count":
				r.SwapInstructions = r.SwapInstructions[:len(r.SwapInstructions)-1]
			case "swap":
				r.SwapInstructions[0], r.SwapInstructions[1] = r.SwapInstructions[1], r.SwapInstructions[0]
			case "protection":
				r.MinimumNetAmountOut++
			case "credit":
				r.Legs[1].AmountIn = r.Legs[0].MinimumNetAmountOut + 1
			case "zero":
				r.Legs[0].AmountIn = 0
			case "nil_swap":
				r.SwapInstructions[0] = nil
			case "nil_setup":
				r.SetupInstructions[0] = nil
			case "typed_nil_swap":
				var ix *solana.GenericInstruction
				r.SwapInstructions[0] = ix
			case "typed_nil_setup":
				var ix *solana.GenericInstruction
				r.SetupInstructions[0] = ix
			case "nil_account":
				ix := r.SwapInstructions[0]
				data, _ := ix.Data()
				keys := append(solana.AccountMetaSlice{}, ix.Accounts()...)
				keys[0] = nil
				r.SwapInstructions[0] = solana.NewInstruction(ix.ProgramID(), keys, data)
			}
			if _, err := SettleCachedRouteWithNativeSol(r, payer, seed, rent, true, false); err == nil {
				t.Fatal("accepted modified bundle")
			}
		})
	}
}
