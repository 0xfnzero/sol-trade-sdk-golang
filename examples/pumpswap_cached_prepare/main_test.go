package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"fmt"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/constants"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	"github.com/gagliardetto/solana-go"
	"os"
	"sync"
	"testing"
)

// Independently decode the signed V1 wire; verify signatures, account indices,
// instruction discriminator, direction and exact input/protected output fields.
func verifyWire(t *testing.T, wire []byte, steps []trading.PumpSwapQuotedStep) {
	t.Helper()
	if len(wire) < 42 || wire[0] != 129 {
		t.Fatal("invalid V1 header")
	}
	n, keys, count := int(wire[1]), int(wire[41]), int(wire[40])
	end := len(wire) - 64*n
	if end < 42+32*keys || len(wire) > 4096 {
		t.Fatal("invalid wire size")
	}
	message := wire[:end]
	for i := 0; i < n; i++ {
		if !ed25519.Verify(ed25519.PublicKey(message[42+32*i:74+32*i]), message, wire[end+64*i:end+64*(i+1)]) {
			t.Fatal("signature mismatch")
		}
	}
	offset := 42 + 32*keys
	mask := binary.LittleEndian.Uint32(message[4:8])
	if mask&3 != 0 {
		offset += 8
	}
	for _, bit := range []uint32{4, 8, 16} {
		if mask&bit != 0 {
			offset += 4
		}
	}
	headers := message[offset : offset+4*count]
	offset += 4 * count
	pumpCount := 0
	for i := 0; i < count; i++ {
		h := headers[4*i : 4*i+4]
		program, accounts, size := int(h[0]), int(h[1]), int(binary.LittleEndian.Uint16(h[2:]))
		if program >= keys || offset+accounts+size > end {
			t.Fatal("bad compiled index or size")
		}
		for _, a := range message[offset : offset+accounts] {
			if int(a) >= keys {
				t.Fatal("bad account index")
			}
		}
		key := solana.PublicKeyFromBytes(message[42+32*program : 74+32*program])
		if key == instruction.PUMPSWAP_PROGRAM {
			data := message[offset+accounts : offset+accounts+size]
			step := steps[0]
			for _, s := range steps {
				if s.Pool == solana.PublicKeyFromBytes(message[42+32*int(message[offset]):74+32*int(message[offset])]) {
					step = s
				}
			}
			if binary.LittleEndian.Uint64(data[8:16]) != step.AmountIn || binary.LittleEndian.Uint64(data[16:24]) != step.MinimumNetAmountOut {
				t.Fatal("signed PumpSwap amount drift")
			}
			if size == 25 {
				if !bytes.Equal(data[:8], instruction.PUMPSWAP_BUY_EXACT_QUOTE_IN_DISCRIMINATOR) || data[24] != 1 {
					t.Fatal("bad buy data")
				}
			} else if size != 24 || !bytes.Equal(data[:8], instruction.PUMPSWAP_SELL_DISCRIMINATOR) {
				t.Fatal("bad sell data")
			}
			pumpCount++
		}
		offset += accounts + size
	}
	if offset != end || pumpCount != 1 {
		t.Fatal("unexpected signed payload")
	}
}

func TestPreparedPumpSwapConcurrentV0V1Builds(t *testing.T) {
	wallet := solana.NewWallet()
	search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", wallet.PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := trading.PrepareBestPumpSwapTrade(search, r)
	if err != nil {
		t.Fatal(err)
	}
	signers := []solana.PrivateKey{wallet.PrivateKey}
	original, err := selected.Trade.BuildTransaction(r.Context, signers)
	if err != nil {
		t.Fatal(err)
	}
	addresses := solana.PublicKeySlice{}
	for i := 0; i < int(original[41]); i++ {
		addresses = append(addresses, solana.PublicKeyFromBytes(original[42+32*i:74+32*i]))
	}
	opts := trading.PumpSwapV0Options{AddressLookupTables: map[solana.PublicKey]solana.PublicKeySlice{solana.NewWallet().PublicKey(): addresses}}
	errors := make(chan error, 8)
	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(v0 bool) {
			defer group.Done()
			for i := 0; i < 4; i++ {
				var e error
				if v0 {
					_, e = selected.Trade.BuildV0Transaction(r.Context, signers, opts)
				} else {
					var wire []byte
					wire, e = selected.Trade.BuildTransaction(r.Context, signers)
					if e == nil && !bytes.Equal(wire, original) {
						e = fmt.Errorf("prepared V1 mutated during concurrent build")
					}
				}
				if e != nil {
					errors <- e
					return
				}
			}
		}(worker%2 == 0)
	}
	group.Wait()
	close(errors)
	for e := range errors {
		t.Fatal(e)
	}
}

func TestCapturedPumpSwapNativeReplay(t *testing.T) {
	wallet := solana.NewWallet()
	for iteration := 0; iteration < 2; iteration++ {
		for _, buy := range []bool{true, false} {
			for _, direct := range []bool{false, true} {
				search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", wallet.PublicKey(), buy)
				if err != nil {
					t.Fatal(err)
				}
				if direct {
					search.AssetMint = solana.MustPublicKeyFromBase58("TTWofwAge91oFhZs7kpQdyrVRkmevgM88xijGvQFbKo")
					search.ConversionPools = nil
					search.MaximumConversionHops = 0
					if buy {
						r.Amount = 10000
					}
				}
				selection, err := trading.PrepareBestPumpSwapTrade(search, r)
				if err != nil {
					t.Fatal(err)
				}
				steps := selection.Trade.Steps()
				if !direct {
					expected := []uint64{102110, 491, 47932572}
					if !buy {
						expected = []uint64{9924, 2017142, 19264527}
					}
					for i, want := range expected {
						if steps[i].MinimumNetAmountOut != want {
							t.Fatalf("cross-language quote drift at hop %d: got %d want %d", i, steps[i].MinimumNetAmountOut, want)
						}
					}
				}
				expected := 3
				if direct {
					expected = 1
				}
				if len(steps) != expected || selection.RoutesEvaluated != 1 {
					t.Fatal("unexpected selected path")
				}
				for i := 1; i < len(steps); i++ {
					if steps[i].AmountIn != steps[i-1].MinimumNetAmountOut {
						t.Fatal("spent unprotected intermediate credit")
					}
				}
				original := selection.Trade.Steps()[0]
				steps[0].AmountIn = 1
				if selection.Trade.Steps()[0] != original {
					t.Fatal("mutable prepared steps")
				}
				wire, err := selection.Trade.BuildTransaction(r.Context, []solana.PrivateKey{wallet.PrivateKey})
				if err != nil {
					t.Fatal(err)
				}
				verifyWire(t, wire, selection.Trade.Steps())
				// Compile the same real route as V0 with caller-cached lookup
				// addresses, nonce authority and a tip; parse and verify both signatures.
				authority := solana.NewWallet()
				var addresses solana.PublicKeySlice
				for i := 0; i < int(wire[41]); i++ {
					addresses = append(addresses, solana.PublicKeyFromBytes(wire[42+32*i:74+32*i]))
				}
				table := solana.NewWallet().PublicKey()
				options := trading.PumpSwapV0Options{AddressLookupTables: map[solana.PublicKey]solana.PublicKeySlice{table: addresses}, NonceAccount: solana.NewWallet().PublicKey(), NonceAuthority: authority.PublicKey(), NonceHash: solana.Hash{7}, GasConfig: &trading.GasFeeConfig{ComputeUnitLimit: 450000, ComputeUnitPrice: 1000}}
				r.TipAccount = solana.NewWallet().PublicKey()
				r.TipLamports = 100
				withTip, err := trading.PrepareBestPumpSwapTrade(search, r)
				if err != nil {
					t.Fatal(err)
				}
				v1Nonce, err := withTip.Trade.BuildV1Transaction(r.Context, []solana.PrivateKey{wallet.PrivateKey, authority.PrivateKey}, trading.PumpSwapNonce{Account: options.NonceAccount, Authority: options.NonceAuthority, Hash: options.NonceHash})
				if err != nil {
					t.Fatal(err)
				}
				verifyWire(t, v1Nonce, withTip.Trade.Steps())
				if !bytes.Equal(v1Nonce[8:40], options.NonceHash[:]) {
					t.Fatal("V1 cached nonce hash ignored")
				}
				v0, err := withTip.Trade.BuildV0Transaction(r.Context, []solana.PrivateKey{wallet.PrivateKey, authority.PrivateKey}, options)
				if err != nil {
					t.Fatal(err)
				}
				tx, err := solana.TransactionFromBytes(v0)
				if err != nil {
					t.Fatal(err)
				}
				if err = tx.VerifySignatures(); err != nil || len(tx.Signatures) != 2 || len(v0) > 1232 || tx.Message.RecentBlockhash != options.NonceHash {
					t.Fatal("invalid signed V0", err)
				}
				first := tx.Message.Instructions[0]
				if tx.Message.AccountKeys[first.ProgramIDIndex] != solana.SystemProgramID || binary.LittleEndian.Uint32(first.Data) != 4 {
					t.Fatal("nonce advance is not first")
				}
				if len(tx.Message.AddressTableLookups) == 0 {
					t.Fatal("cached ALT ignored")
				}
				if !direct {
					if _, err = selection.Trade.BuildV0Transaction(r.Context, []solana.PrivateKey{wallet.PrivateKey}, trading.PumpSwapV0Options{}); err == nil {
						t.Fatal("oversized V0 accepted without ALT")
					}
				}
				stale := r.Context
				stale.Slot++
				if _, err = selection.Trade.BuildTransaction(stale, []solana.PrivateKey{wallet.PrivateKey}); err == nil {
					t.Fatal("accepted expired quote")
				}
				epoch := r.Context
				epoch.Epoch++
				if err = selection.Trade.ValidateFreshness(epoch); err == nil {
					t.Fatal("accepted epoch change")
				}
				if _, err = selection.Trade.BuildTransaction(r.Context, []solana.PrivateKey{wallet.PrivateKey, solana.NewWallet().PrivateKey}); err == nil {
					t.Fatal("accepted extraneous signer")
				}
				if _, err = selection.Trade.BuildTransaction(r.Context, nil); err == nil {
					t.Fatal("accepted missing payer signature")
				}
				// Same frozen request deterministically replays independently; sell routes
				// are freshly quoted from target back to the receipt asset.
				again, err := trading.PrepareBestPumpSwapTrade(search, r)
				if err != nil || again.Trade.MinimumOutput() != selection.Trade.MinimumOutput() {
					t.Fatal("non-repeatable frozen quote", err)
				}
			}
		}
	}
}

func TestPreparedPumpSwapFreezesCallerQuotePolicy(t *testing.T) {
	wallet := solana.NewWallet()
	search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", wallet.PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}
	originalSlip := *r.SlippageBps
	units := uint32(450000)
	price := uint64(123)
	r.V1Config = &serialization.V1Config{ComputeUnitLimit: &units, PriorityFee: &price}
	selected, err := trading.PrepareBestPumpSwapTrade(search, r)
	if err != nil {
		t.Fatal(err)
	}
	*r.SlippageBps = 9999
	units = 1
	price = 99999
	wire, err := selected.Trade.BuildTransaction(r.Context, []solana.PrivateKey{wallet.PrivateKey})
	if err != nil {
		t.Fatal(err)
	}
	verifyWire(t, wire, selected.Trade.Steps())
	offset := 42 + 32*int(wire[41])
	if binary.LittleEndian.Uint64(wire[offset:offset+8]) != 123 || binary.LittleEndian.Uint32(wire[offset+8:offset+12]) != 450000 {
		t.Fatal("caller mutated frozen V1 gas policy")
	}
	if selected.Trade.MinimumOutput() != 47932572 || originalSlip != 100 {
		t.Fatal("caller mutated frozen slippage")
	}
	if _, err = selected.Trade.BuildV0Transaction(r.Context, []solana.PrivateKey{wallet.PrivateKey}, trading.PumpSwapV0Options{}); err == nil {
		t.Fatal("silently dropped V1 settings in V0")
	}
}

// Synthetic competing AMM fee schedules over captured vault balances exercise
// ranking and rejection; they are local scenarios, not real alternate markets.
func TestCapturedPumpSwapSelectsHighestProtectedRoute(t *testing.T) {
	search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", solana.NewWallet().PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := trading.PrepareBestPumpSwapTrade(search, r)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../fixtures/pumpswap_ttwo_route_accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	var exported struct{ Accounts []struct{ Address string } }
	if err = json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}
	oldHint := search.ConversionPools[1]
	for _, mode := range []string{"worse", "tie", "stale"} {
		cache := &subscription.SubscriptionAccountCache{}
		for _, a := range exported.Accounts {
			k := solana.MustPublicKeyFromBase58(a.Address)
			value, e := r.Snapshot.Get(k, r.Context, nil)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = cache.Update(k, value); e != nil {
				t.Fatal(e)
			}
		}
		alternative := solana.PublicKey{0, 0, 1}
		account, err := r.Snapshot.Get(oldHint.Pool, r.Context, nil)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "worse" {
			binary.LittleEndian.PutUint64(account.Data[176:184], 1000)
			binary.LittleEndian.PutUint64(account.Data[184:192], 10000)
		}
		if mode == "stale" {
			account.Slot--
		}
		if _, err = cache.Update(alternative, account); err != nil {
			t.Fatal(err)
		}
		local := search
		local.ConversionPools = append(append([]subscription.PoolTradeHint(nil), search.ConversionPools...), subscription.PoolTradeHint{Pool: alternative, InputMint: oldHint.InputMint, OutputMint: oldHint.OutputMint})
		req := r
		req.Snapshot = cache.Snapshot()
		selected, err := trading.PrepareBestPumpSwapTrade(local, req)
		if err != nil {
			t.Fatal(err)
		}
		wantPool := oldHint.Pool
		if mode == "tie" {
			wantPool = alternative
		}
		if selected.RoutesEvaluated != 2 || selected.Trade.Steps()[0].Pool != wantPool || selected.Trade.MinimumOutput() != baseline.Trade.MinimumOutput() {
			t.Fatal("incorrect best protected path", mode)
		}
		if mode == "stale" && (len(selected.RejectedRoutes) != 1 || selected.RejectedRoutes[0].Reason == "") {
			t.Fatal("missing rejection diagnostic")
		}
		local.MaximumSearchNodes = 3
		if _, err = trading.PrepareBestPumpSwapTrade(local, req); err == nil {
			t.Fatal("accepted partial best result")
		}
	}
}

func TestCapturedPumpSwapRejectsInvalidRequests(t *testing.T) {
	search, r, err := loadSnapshot("../fixtures/pumpswap_ttwo_route_accounts.json", solana.NewWallet().PublicKey(), true)
	if err != nil {
		t.Fatal(err)
	}
	r.Amount = 1
	if _, err = trading.PrepareBestPumpSwapTrade(search, r); err == nil {
		t.Fatal("accepted zero-output route")
	}
	r.Amount = 1000000
	r.TradeType = soltradesdk.TradeTypeSell
	search.AssetMint = search.TokenMint
	if _, err = trading.PrepareBestPumpSwapTrade(search, r); err == nil {
		t.Fatal("accepted identical endpoints")
	}
	search.AssetMint = constants.SOL_TOKEN_ACCOUNT
	search.MaximumSearchNodes = 1
	if _, err = trading.PrepareBestPumpSwapTrade(search, r); err == nil {
		t.Fatal("accepted incomplete search")
	}
}
