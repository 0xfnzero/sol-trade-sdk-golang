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
	soltradesdk "github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/serialization"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/subscription"
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
		PumpFunVersion       soltradesdk.PumpFunVersion `json:"pumpfun_version"`
		MaximumSlotAge       json.RawMessage            `json:"maximum_slot_age"`
		SlippageBps          uint16                     `json:"slippage_bps"`
		ReadSlot             string                     `json:"read_slot"`
		Time                 string                     `json:"unix_timestamp"`
		Hash                 string                     `json:"recent_blockhash"`
		Input                bool                       `json:"native_input"`
		Output               bool                       `json:"native_output"`
		Seed                 string                     `json:"temporary_wsol_seed"`
		Rent                 string                     `json:"rent_lamports"`
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
	return CachedTradeRequest{PumpFunVersion: v.PumpFunVersion, DexType: dex, SlippageBps: slippage, TradeType: trade, Snapshot: c.Snapshot(), Hints: hints, Context: subscription.CacheReadContext{Slot: num(v.ReadSlot), Epoch: num(v.Epoch), MaximumSlotAge: maxAge}, UnixTimestamp: num(v.Time), Payer: payer, Amount: num(v.Amount), RecentBlockhash: solana.MustHashFromBase58(v.Hash), NativeInput: v.Input, NativeOutput: v.Output, TemporaryWsolSeed: v.Seed, RentLamports: num(v.Rent)}
}

func TestPumpFunInstructionVersionsPreserveProtectedHops(t *testing.T) {
	if soltradesdk.NewTradeConfigBuilder("http://localhost").Build().PumpFunVersion != soltradesdk.PumpFunV3 {
		t.Fatal("default must be V3")
	}
	for _, direction := range []string{"buy", "sell"} {
		r := cachedFixture(t, direction, "pumpfun_whirlpool", solana.PublicKey{})
		base, e := PrepareCachedTrade(r)
		if e != nil {
			t.Fatal(e)
		}
		floor := uint64(1)
		auto := r
		auto.PumpRoute = instruction.PumpRouteOptions{MinimumOutputAmount: &floor, BuybackRecipient: &r.Payer}
		fallback, e := PrepareCachedTrade(auto)
		if e != nil || !bytes.Equal(base.Compiled.Message, fallback.Compiled.Message) {
			t.Fatal("cross-DEX automatic fallback changed wire", e)
		}
		required := auto
		required.PumpRoute.Execution = instruction.PumpRouteOfficial
		if _, e := PrepareCachedTrade(required); e == nil {
			t.Fatal("official mode silently selected a cross-DEX route")
		}
		tooHigh := uint64(math.MaxUint64)
		auto.PumpRoute.MinimumOutputAmount = &tooHigh
		if _, e := PrepareCachedTrade(auto); e == nil {
			t.Fatal("fallback weakened final minimum")
		}
		var minimum uint64
		for _, version := range []soltradesdk.PumpFunVersion{soltradesdk.PumpFunLegacy, soltradesdk.PumpFunV2, soltradesdk.PumpFunV3} {
			r.PumpFunVersion = version
			r.TradeConfig = soltradesdk.NewTradeConfigBuilder("http://localhost").PumpFunVersion(soltradesdk.PumpFunV2).Build()
			prepared, err := PrepareCachedTrade(r)
			if err != nil {
				t.Fatal(err)
			}
			index := 0
			if direction == "buy" {
				index = len(prepared.Route.Legs) - 1
			}
			anchor := prepared.Route.Legs[index]
			data, err := anchor.Instruction.Data()
			if err != nil {
				t.Fatal(err)
			}
			buy := map[soltradesdk.PumpFunVersion][]byte{soltradesdk.PumpFunLegacy: {56, 252, 116, 8, 158, 223, 205, 95}, soltradesdk.PumpFunV2: {194, 171, 28, 70, 104, 77, 91, 47}, soltradesdk.PumpFunV3: {225, 247, 80, 30, 213, 179, 132, 136}}
			sell := map[soltradesdk.PumpFunVersion][]byte{soltradesdk.PumpFunLegacy: {51, 230, 133, 164, 1, 127, 131, 173}, soltradesdk.PumpFunV2: {93, 246, 130, 60, 231, 233, 64, 178}, soltradesdk.PumpFunV3: {28, 146, 222, 119, 38, 196, 105, 213}}
			disc := sell[version]
			if direction == "buy" {
				disc = buy[version]
			}
			if !bytes.Equal(data[:8], disc) || binary.LittleEndian.Uint64(data[8:]) != anchor.AmountIn || binary.LittleEndian.Uint64(data[16:]) != anchor.MinimumNetAmountOut {
				t.Fatal("version or protected amount mismatch")
			}
			if version == soltradesdk.PumpFunV3 && len(anchor.Instruction.Accounts()) != 17 {
				t.Fatal("V3 accounts")
			}
			if minimum != 0 && minimum != prepared.Route.MinimumNetAmountOut {
				t.Fatal("version changed route protection")
			}
			minimum = prepared.Route.MinimumNetAmountOut
		}
	}
}

func TestCachedClientInheritsPumpFunVersion(t *testing.T) {
	config := func(version soltradesdk.PumpFunVersion) *soltradesdk.TradeConfig {
		if version == "" {
			return nil
		}
		return soltradesdk.NewTradeConfigBuilder("http://unused.invalid").PumpFunVersion(version).Build()
	}
	signer := solana.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32)))
	for _, direction := range []string{"buy", "sell"} {
		base := cachedFixture(t, direction, "pumpfun_whirlpool", signer.PublicKey())
		for _, tc := range []struct {
			name                                           string
			global, request, requestConfig, snapshot, want soltradesdk.PumpFunVersion
		}{
			{"global_v1", soltradesdk.PumpFunLegacy, "", "", soltradesdk.PumpFunV3, soltradesdk.PumpFunLegacy},
			{"global_v2", soltradesdk.PumpFunV2, "", "", soltradesdk.PumpFunV3, soltradesdk.PumpFunV2},
			{"global_v3", soltradesdk.PumpFunV3, "", "", soltradesdk.PumpFunV2, soltradesdk.PumpFunV3},
			{"request_override", soltradesdk.PumpFunV2, soltradesdk.PumpFunV3, soltradesdk.PumpFunLegacy, soltradesdk.PumpFunV2, soltradesdk.PumpFunV3},
			{"request_config_override", soltradesdk.PumpFunV2, "", soltradesdk.PumpFunV3, soltradesdk.PumpFunLegacy, soltradesdk.PumpFunV3},
			{"snapshot_fallback", "", "", "", soltradesdk.PumpFunV2, soltradesdk.PumpFunV2},
			{"default_v3", "", "", "", "", soltradesdk.PumpFunV3},
		} {
			t.Run(direction+"/"+tc.name, func(t *testing.T) {
				r := base
				r.PumpFunVersion, r.TradeConfig = tc.request, config(tc.requestConfig)
				r.Snapshot = base.Snapshot.WithPumpFunVersion(tc.snapshot)
				factory := NewTradeExecutorFactory(NewTradeExecutor(nil, config(tc.global), nil))
				client := NewTradingClient(factory)
				executor, err := factory.CreateCachedExecutor(r.DexType)
				if err != nil {
					t.Fatal(err)
				}
				// The explicit-version path supplies the expected instruction inputs;
				// TestPumpFunInstructionVersionsPreserveProtectedHops checks its ABI.
				expectedRequest := r
				expectedRequest.PumpFunVersion = tc.want
				expected, err := PrepareCachedTrade(expectedRequest)
				if err != nil {
					t.Fatal(err)
				}
				index := 0
				if direction == "buy" {
					index = len(expected.Route.Legs) - 1
				}
				wantData, err := expected.Route.Legs[index].Instruction.Data()
				if err != nil {
					t.Fatal(err)
				}
				for name, prepare := range map[string]func(CachedTradeRequest) (PreparedCachedTrade, error){
					"client_prepare": client.PrepareCachedTrade, "factory_prepare": executor.Prepare,
				} {
					t.Run(name, func(t *testing.T) {
						p, err := prepare(r)
						if err != nil {
							t.Fatal(err)
						}
						data, err := p.Route.Legs[index].Instruction.Data()
						if err != nil || !bytes.Equal(data, wantData) || p.Route.MinimumNetAmountOut != expected.Route.MinimumNetAmountOut {
							t.Fatalf("version %s or protected inputs not preserved: data=%x err=%v", tc.want, data, err)
						}
					})
				}
				for _, entry := range []string{"client_execute", "factory_execute", "client_buy_sell"} {
					t.Run(entry, func(t *testing.T) {
						calls := 0
						submit := func(_ context.Context, wire []byte, side soltradesdk.TradeType) (solana.Signature, error) {
							calls++
							message := wire[:len(wire)-ed25519.SignatureSize]
							var signature solana.Signature
							copy(signature[:], wire[len(message):])
							if side != r.TradeType || !bytes.Contains(message, wantData) || !ed25519.Verify(ed25519.PublicKey(r.Payer[:]), message, signature[:]) {
								t.Fatalf("signed wire did not preserve version %s and protected inputs", tc.want)
							}
							return signature, nil
						}
						var err error
						signers := []solana.PrivateKey{signer}
						switch entry {
						case "client_execute":
							_, err = client.ExecuteCachedTrade(context.Background(), r, signers, submit)
						case "factory_execute":
							_, err = executor.Execute(context.Background(), r, signers, submit)
						default:
							request := &TradeExecutionRequest{Request: r, Signers: signers, Submit: submit}
							if direction == "buy" {
								_, err = client.Buy(context.Background(), r.DexType, request)
							} else {
								_, err = client.Sell(context.Background(), r.DexType, request)
							}
						}
						if err != nil || calls != 1 {
							t.Fatalf("submission failed: calls=%d err=%v", calls, err)
						}
					})
				}
				// Reusing the source snapshot must still select its original version.
				control := r
				control.PumpFunVersion, control.TradeConfig = "", nil
				original, err := PrepareCachedTrade(control)
				if err != nil {
					t.Fatal(err)
				}
				control.PumpFunVersion = tc.snapshot
				if control.PumpFunVersion == "" {
					control.PumpFunVersion = soltradesdk.PumpFunV3
				}
				unchanged, err := PrepareCachedTrade(control)
				if err != nil {
					t.Fatal(err)
				}
				before, _ := unchanged.Route.Legs[index].Instruction.Data()
				after, _ := original.Route.Legs[index].Instruction.Data()
				if !bytes.Equal(before, after) || r.PumpFunVersion != tc.request || r.TradeConfig != nil && r.TradeConfig.PumpFunVersion != tc.requestConfig {
					t.Fatal("client changed the caller's request or snapshot")
				}
			})
		}
	}
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
					t.Fatal("cross-language unsigned fixture wire/quote mismatch")
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
	executor := &CachedTradeExecutor{DexType: soltradesdk.DexTypeStonkFun}
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
			e := &CachedTradeExecutor{DexType: dex}
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
		// Original captures preserve the old ceil-minimum and track_volume=0.
		// Current wires use official floor/track_volume=1 and readonly global volume.
		// The readonly privilege is asserted separately in subscription tests.
		expectedHash := "7eef1cfe9c2b2f958bb0f290a1e35a90acdf84283cec137bddc43d5b0263c027"
		if direction == "sell" {
			expectedHash = "abb7e8eff2d17f35b8fef75dc7cf585fb07db6427c7941bf992f02ba02512117"
		}
		oldMinimum, err := strconv.ParseUint(v.Expected.Minimum, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(wire)) != expectedHash || p.Route.MinimumNetAmountOut != oldMinimum-1 {
			t.Errorf("mainnet replay differs: direction=%s hash=%x minimum=%d", direction, sha256.Sum256(wire), p.Route.MinimumNetAmountOut)
		}
	}
}

func TestExplicitDammRejectsInvalidSlippage(t *testing.T) {
	for _, slip := range []uint16{10000, 10001, 65535} {
		minimum := uint64(1)
		_, err := PrepareCachedTrade(CachedTradeRequest{DexType: soltradesdk.DexTypeMeteoraDammV2, TradeType: soltradesdk.TradeTypeBuy, Amount: 1, FixedOutputAmount: &minimum, SlippageBps: &slip})
		if err == nil || !strings.Contains(err.Error(), "slippage") {
			t.Fatalf("accepted invalid slippage %d: %v", slip, err)
		}
	}
}
