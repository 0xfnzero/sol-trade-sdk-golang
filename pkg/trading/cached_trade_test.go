package trading

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/gagliardetto/solana-go"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func cachedFixture(t *testing.T, direction, venue string, payer solana.PublicKey) CachedTradeRequest {
	t.Helper()
	prefix := ""
	if venue == "dlmm" {
		prefix = "dlmm_"
	}
	filename := prefix + "sol_route_" + direction + "_mainnet_20261002.json"
	if venue == "pumpswap" {
		filename = "pumpswap_" + direction + "_mainnet_20261004.json"
	}
	if venue == "pumpfun_whirlpool" || venue == "pumpfun_dlmm" || venue == "pumpfun_clmm" {
		filename = venue + "_usdc_" + direction + "_20261005.json"
	}
	b, e := os.ReadFile("../../examples/fixtures/" + filename)
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Payer, Amount, Epoch string
		MaximumSlotAge       json.RawMessage `json:"maximum_slot_age"`
		SlippageBps          uint16          `json:"slippage_bps"`
		ReadSlot             string          `json:"read_slot"`
		Time                 string          `json:"unix_timestamp"`
		Hash                 string          `json:"recent_blockhash"`
		Input                bool            `json:"native_input"`
		Output               bool            `json:"native_output"`
		Seed                 string          `json:"temporary_wsol_seed"`
		Rent                 string          `json:"rent_lamports"`
		Legs                 []struct {
			Pool   string
			Input  string `json:"input_mint"`
			Output string `json:"output_mint"`
		}
		Accounts []struct {
			Pubkey, Owner, Data, Slot string
			Version                   string `json:"write_version"`
		}
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	num := func(s string) uint64 {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	c := &subscription.SubscriptionAccountCache{}
	for _, a := range v.Accounts {
		d, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Update(solana.MustPublicKeyFromBase58(a.Pubkey), subscription.CachedAccount{Owner: solana.MustPublicKeyFromBase58(a.Owner), Data: d, Slot: num(a.Slot), WriteVersion: num(a.Version)}); e != nil {
			t.Fatal(e)
		}
	}
	hints := []subscription.PoolTradeHint{}
	for _, h := range v.Legs {
		hints = append(hints, subscription.PoolTradeHint{Pool: solana.MustPublicKeyFromBase58(h.Pool), InputMint: solana.MustPublicKeyFromBase58(h.Input), OutputMint: solana.MustPublicKeyFromBase58(h.Output)})
	}
	trade := soltradesdk.TradeTypeBuy
	if direction == "sell" {
		trade = soltradesdk.TradeTypeSell
	}
	if payer == (solana.PublicKey{}) {
		payer = solana.MustPublicKeyFromBase58(v.Payer)
	}
	dex := soltradesdk.DexTypeStonkFun
	maxAge := uint64(0)
	var slippage *uint16
	if venue == "pumpswap" || venue == "pumpfun_whirlpool" || venue == "pumpfun_dlmm" || venue == "pumpfun_clmm" {
		dex = soltradesdk.DexTypePumpSwap
		if venue != "pumpswap" {
			dex = soltradesdk.DexTypePumpFun
		}
		maxAge = num(string(bytes.Trim(v.MaximumSlotAge, `"`)))
		slippage = &v.SlippageBps
	}
	return CachedTradeRequest{DexType: dex, SlippageBps: slippage, TradeType: trade, Snapshot: c.Snapshot(), Hints: hints, Context: subscription.CacheReadContext{Slot: num(v.ReadSlot), Epoch: num(v.Epoch), MaximumSlotAge: maxAge}, UnixTimestamp: num(v.Time), Payer: payer, Amount: num(v.Amount), RecentBlockhash: solana.MustHashFromBase58(v.Hash), NativeInput: v.Input, NativeOutput: v.Output, TemporaryWsolSeed: v.Seed, RentLamports: num(v.Rent)}
}
func TestCurveAnchorRejectsMislabeledDirection(t *testing.T) {
	for _, dex := range []soltradesdk.DexType{soltradesdk.DexTypeStonkFun, soltradesdk.DexTypeLaunchLab, soltradesdk.DexTypeBonk} {
		for _, side := range []string{"buy", "sell"} {
			r := cachedFixture(t, side, "dlmm", solana.PublicKey{})
			anchor := r.Hints[0]
			if side == "buy" {
				anchor = r.Hints[len(r.Hints)-1]
				r.TradeType = soltradesdk.TradeTypeSell
			} else {
				r.TradeType = soltradesdk.TradeTypeBuy
			}
			r.DexType = dex
			r.Hints = []subscription.PoolTradeHint{anchor}
			r.NativeInput = false
			r.NativeOutput = false
			if _, err := PrepareCachedTrade(r); err == nil || !strings.Contains(err.Error(), "anchor direction") {
				t.Fatal(dex, side, err)
			}
		}
	}
}

func TestPumpFunConcentratedMultihopEvidence(t *testing.T) {
	for _, protocol := range []string{"whirlpool", "dlmm", "clmm"} {
		for _, direction := range []string{"buy", "sell"} {
			t.Run(protocol+"_"+direction, func(t *testing.T) {
				r := cachedFixture(t, direction, "pumpfun_"+protocol, solana.PublicKey{})
				data, err := os.ReadFile("../../examples/fixtures/pumpfun_" + protocol + "_usdc_" + direction + "_20261005.json")
				if err != nil {
					t.Fatal(err)
				}
				var v struct {
					Expected struct {
						Hash     string `json:"wire_sha256"`
						Minimum  string `json:"minimum_out"`
						Residual string `json:"estimated_native_residual_lamports"`
					}
				}
				if err = json.Unmarshal(data, &v); err != nil {
					t.Fatal(err)
				}
				executor, err := NewTradeExecutorFactory(nil).CreateCachedExecutor(r.DexType)
				if err != nil {
					t.Fatal(err)
				}
				p, err := executor.Prepare(r)
				if err != nil {
					t.Fatal(err)
				}
				wire := append(append([]byte{}, p.Compiled.Message...), make([]byte, p.Compiled.RequiredSignatures*64)...)
				if fmt.Sprintf("%x", sha256.Sum256(wire)) != v.Expected.Hash || strconv.FormatUint(p.Route.MinimumNetAmountOut, 10) != v.Expected.Minimum || strconv.FormatUint(p.EstimatedNativeResidualLamports, 10) != v.Expected.Residual {
					t.Fatal("cross-language simulation evidence mismatch")
				}
			})
		}
	}
}
func TestCachedFactoryPreparation(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		for _, venue := range []string{"dlmm", "whirlpool"} {
			r := cachedFixture(t, direction, venue, solana.PublicKey{})
			f := NewTradeExecutorFactory(nil)
			e, err := f.CreateCachedExecutor(soltradesdk.DexTypeStonkFun)
			if err != nil {
				t.Fatal(err)
			}
			p, err := e.Prepare(r)
			if err != nil {
				t.Fatal(err)
			}
			required := r.RentLamports
			if r.NativeInput {
				required += r.Amount
			}
			if len(p.Route.Legs) != 3 || len(p.Compiled.Message) <= 1232 || p.RequiredNativeLamports != required {
				t.Fatal(p)
			}
			compute, loaded := uint32(300000), uint32(64*1024*1024)
			gold, err := serialization.CompileV1Message(r.Payer, p.Instructions, r.RecentBlockhash, serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
			if err != nil || !bytes.Equal(p.Compiled.Message, gold.Message) {
				t.Fatal("V1 default config")
			}
			r.V1Config = &serialization.V1Config{}
			p, err = e.Prepare(r)
			if err != nil || !bytes.Equal(p.Compiled.Message, gold.Message) {
				t.Fatal("partial V1 config")
			}
		}
	}
}
func TestCachedTransportSignatures(t *testing.T) {
	seed := bytes.Repeat([]byte{7}, 32)
	signer := solana.PrivateKey(ed25519.NewKeyFromSeed(seed))
	r := cachedFixture(t, "buy", "dlmm", signer.PublicKey())
	executor := &CachedTradeExecutor{soltradesdk.DexTypeStonkFun}
	p, err := executor.Prepare(r)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	submit := func(ctx context.Context, wire []byte, d soltradesdk.TradeType) (solana.Signature, error) {
		calls++
		if d != soltradesdk.TradeTypeBuy || !bytes.Equal(wire[:len(p.Compiled.Message)], p.Compiled.Message) {
			t.Fatal("wire")
		}
		sig := wire[len(p.Compiled.Message) : len(p.Compiled.Message)+64]
		if !ed25519.Verify(ed25519.PublicKey(r.Payer[:]), p.Compiled.Message, sig) {
			t.Fatal("signature")
		}
		var s solana.Signature
		copy(s[:], sig)
		return s, nil
	}
	receipt, err := executor.Execute(context.Background(), r, []solana.PrivateKey{signer}, submit)
	if err != nil || !receipt.Submitted || receipt.Confirmed || calls != 1 {
		t.Fatal(receipt, err)
	}
	if _, err = executor.Execute(context.Background(), r, nil, submit); err == nil || calls != 1 {
		t.Fatal("missing signer submitted")
	}
	if _, err = executor.Execute(context.Background(), r, []solana.PrivateKey{signer}, func(context.Context, []byte, soltradesdk.TradeType) (solana.Signature, error) {
		return solana.Signature{}, nil
	}); err == nil {
		t.Fatal("wrong signature accepted")
	}
}
func TestCachedTradeInvalid(t *testing.T) {
	for _, kind := range []string{"protocol", "direction", "disconnected", "stale", "native_flags", "wrong_factory"} {
		t.Run(kind, func(t *testing.T) {
			r := cachedFixture(t, "buy", "dlmm", solana.PublicKey{})
			dex := soltradesdk.DexTypeStonkFun
			switch kind {
			case "protocol":
				r.DexType = soltradesdk.DexTypeRaydiumClmm
			case "direction":
				r.TradeType = soltradesdk.TradeType(99)
			case "disconnected":
				r.Hints[0], r.Hints[2] = r.Hints[2], r.Hints[0]
			case "stale":
				r.Context.Slot++
			case "native_flags":
				r.NativeOutput = true
			case "wrong_factory":
				dex = soltradesdk.DexTypeRaydiumClmm
			}
			e := &CachedTradeExecutor{dex}
			if _, err := e.Prepare(r); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestCachedTipBeforeBusiness(t *testing.T) {
	tipAccount := solana.MustPublicKeyFromBase58("96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5")
	for _, direction := range []string{"buy", "sell"} {
		r := cachedFixture(t, direction, "dlmm", solana.PublicKey{})
		base, e := PrepareCachedTrade(r)
		if e != nil {
			t.Fatal(e)
		}
		r.TipAccount, r.TipLamports = tipAccount, 5000
		p, e := PrepareCachedTrade(r)
		if e != nil {
			t.Fatal(e)
		}
		d, e := p.Instructions[0].Data()
		if e != nil {
			t.Fatal(e)
		}
		if p.Instructions[0].ProgramID() != solana.SystemProgramID || binary.LittleEndian.Uint32(d) != 2 || binary.LittleEndian.Uint64(d[4:]) != 5000 {
			t.Fatal("tip transfer mismatch")
		}
		m := p.Instructions[0].Accounts()
		if m[0].PublicKey != r.Payer || !m[0].IsSigner || !m[0].IsWritable || m[1].PublicKey != tipAccount || m[1].IsSigner || !m[1].IsWritable {
			t.Fatal("tip metas mismatch")
		}
		if p.RequiredNativeLamports != base.RequiredNativeLamports+5000 || len(p.Instructions) != len(base.Instructions)+1 {
			t.Fatal("funding mismatch")
		}
		for i, ix := range base.Instructions {
			b, _ := ix.Data()
			n, _ := p.Instructions[i+1].Data()
			if !bytes.Equal(b, n) || ix.ProgramID() != p.Instructions[i+1].ProgramID() {
				t.Fatal("changed business instruction")
			}
		}
		r.NativeInput, r.NativeOutput = false, false
		p, e = PrepareCachedTrade(r)
		if e != nil || p.RequiredNativeLamports != 5000 {
			t.Fatal("WSOL tip funding", e)
		}
	}
	r := cachedFixture(t, "buy", "dlmm", solana.PublicKey{})
	for _, bad := range []struct {
		key    solana.PublicKey
		amount uint64
	}{{tipAccount, 0}, {solana.PublicKey{}, 5000}, {r.Payer, 1}, {tipAccount, math.MaxUint64}} {
		r.TipAccount, r.TipLamports = bad.key, bad.amount
		if _, e := PrepareCachedTrade(r); e == nil {
			t.Fatal("accepted invalid tip")
		}
	}
}

func TestCandidateRouteMatchesExplicit(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		for _, venue := range []string{"dlmm", "whirlpool"} {
			r := cachedFixture(t, direction, venue, solana.PublicKey{})
			explicit, err := PrepareCachedTrade(r)
			if err != nil {
				t.Fatal(err)
			}
			r.InputMint = r.Hints[0].InputMint
			r.OutputMint = r.Hints[len(r.Hints)-1].OutputMint
			r.Candidates = append([]subscription.PoolTradeHint{}, r.Hints...)
			r.Hints = nil
			for i, j := 0, len(r.Candidates)-1; i < j; i, j = i+1, j-1 {
				r.Candidates[i], r.Candidates[j] = r.Candidates[j], r.Candidates[i]
			}
			automatic, err := PrepareCachedTrade(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(automatic.Compiled.Message, explicit.Compiled.Message) {
				t.Fatal("candidate wire differs from explicit route")
			}
		}
	}
}

func TestUnifiedFactoryRealSignedWire(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		signer := solana.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32)))
		r := cachedFixture(t, direction, "dlmm", signer.PublicKey())
		prepared, err := PrepareCachedTrade(r)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		submit := func(ctx context.Context, wire []byte, trade soltradesdk.TradeType) (solana.Signature, error) {
			calls++
			if trade != r.TradeType || !bytes.Equal(wire[:len(prepared.Compiled.Message)], prepared.Compiled.Message) {
				t.Fatal("wrong signed wire")
			}
			var sig solana.Signature
			copy(sig[:], wire[len(prepared.Compiled.Message):len(prepared.Compiled.Message)+64])
			if !ed25519.Verify(ed25519.PublicKey(r.Payer[:]), prepared.Compiled.Message, sig[:]) {
				t.Fatal("invalid signature")
			}
			return sig, nil
		}
		executor, err := NewTradeExecutorFactory(nil).GetExecutor(r.DexType)
		if err != nil {
			t.Fatal(err)
		}
		envelope := &TradeExecutionRequest{Request: r, Signers: []solana.PrivateKey{signer}, Submit: submit}
		var receipt *ExecuteResult
		if direction == "buy" {
			receipt, err = executor.ExecuteBuy(context.Background(), envelope)
		} else {
			receipt, err = executor.ExecuteSell(context.Background(), envelope)
		}
		if err != nil {
			t.Fatal(err)
		}
		if !receipt.Success || !receipt.Submitted || receipt.Confirmed || calls != 1 {
			t.Fatal(receipt)
		}
		if direction == "buy" {
			_, err = executor.ExecuteSell(context.Background(), envelope)
		} else {
			_, err = executor.ExecuteBuy(context.Background(), envelope)
		}
		if err == nil || calls != 1 {
			t.Fatal("wrong direction submitted")
		}
	}
}

func TestPumpSwapMainnetFactoryWire(t *testing.T) {
	for _, direction := range []string{"buy", "sell"} {
		r := cachedFixture(t, direction, "pumpswap", solana.PublicKey{})
		executor, err := NewTradeExecutorFactory(nil).CreateCachedExecutor(soltradesdk.DexTypePumpSwap)
		if err != nil {
			t.Fatal(err)
		}
		p, err := executor.Prepare(r)
		if err != nil {
			t.Fatal(err)
		}
		d, err := os.ReadFile("../../examples/fixtures/pumpswap_" + direction + "_mainnet_20261004.json")
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Expected struct {
				Hash    string `json:"wire_sha256"`
				Minimum string `json:"minimum_amount_out"`
			}
		}
		if err := json.Unmarshal(d, &v); err != nil {
			t.Fatal(err)
		}
		wire := append(append([]byte{}, p.Compiled.Message...), make([]byte, 64*int(p.Compiled.RequiredSignatures))...)
		if fmt.Sprintf("%x", sha256.Sum256(wire)) != v.Expected.Hash || strconv.FormatUint(p.Route.MinimumNetAmountOut, 10) != v.Expected.Minimum {
			t.Fatal("mainnet replay differs")
		}
	}
}
