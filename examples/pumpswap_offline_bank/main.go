// Native SDK signing with an isolated native Go LiteSVM process. No RPC.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/examples/internal/pumpswapreplay"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/subscription"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/trading"
	sol "github.com/gagliardetto/solana-go"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type account struct {
	Owner    string
	Data     []byte
	Lamports uint64
}
type initRequest struct {
	ProgramsPath, ProgramsSHA256, Payer string
	Clock                               [5]int64
	Accounts                            map[string]account
	Tracked                             []string
}
type bankResult struct {
	OK, CommitOK, SimulationUnchanged, CommitMatchesSimulation, RollbackClean, SignatureRejected, InvalidALTRejected bool
	Error                                                                                                            string
	Logs                                                                                                             []string
	Units, Fee                                                                                                       uint64
	Before, After                                                                                                    map[string]*sol.Account
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func wallet() sol.PrivateKey {
	return sol.PrivateKey(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{117}, 32)))
}
func loadAccounts(root string) (map[string]account, error) {
	var snapshot struct {
		Accounts []struct {
			Address, Owner string
			Data           string `json:"data_base64"`
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "examples/fixtures/pumpswap_ttwo_route_accounts.json"))
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "92e8c240e6a109e1a0617cf407b42d3f8153bd571a0529b61e68b06af070ee47" {
		return nil, fmt.Errorf("route fixture checksum mismatch")
	}
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	accounts := map[string]account{}
	for _, a := range snapshot.Accounts {
		data, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			return nil, e
		}
		accounts[a.Address] = account{Owner: a.Owner, Data: data}
	}
	// Load the matching original RPC lamports. Native WSOL vaults must be
	// backed by their captured lamports, not merely an account's rent reserve.
	raw, err = os.ReadFile(filepath.Join(root, "examples/pumpswap_bank/snapshot.json.gz"))
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "112c16c70b391de882554703de55f907fb7a4168e9e427bfc1aa0bb229b4993d" {
		return nil, fmt.Errorf("snapshot checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var frozen struct {
		RouteAccounts map[string]struct {
			Owner    string
			Data     string `json:"data_base64"`
			Lamports string
		} `json:"route_accounts"`
	}
	err = json.NewDecoder(gz).Decode(&frozen)
	gz.Close()
	if err != nil {
		return nil, err
	}
	for k, a := range accounts {
		captured, ok := frozen.RouteAccounts[k]
		if !ok || captured.Owner != a.Owner || captured.Data != base64.StdEncoding.EncodeToString(a.Data) {
			return nil, fmt.Errorf("mismatched frozen route account %s", k)
		}
		a.Lamports, err = strconv.ParseUint(captured.Lamports, 10, 64)
		if err != nil {
			return nil, err
		}
		accounts[k] = a
	}
	var supplement struct {
		Schema   int
		Accounts map[string]struct {
			Owner    string
			Data     []string
			Lamports uint64
		}
	}
	raw, err = os.ReadFile(filepath.Join(root, "examples/pumpswap_bank/supplemental_accounts.json"))
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "99c9be5f37f317bd21e284db51b8d943c292e2609cffeed377f4add7cc93da6d" {
		return nil, fmt.Errorf("supplement checksum mismatch")
	}
	if err = json.Unmarshal(raw, &supplement); err != nil {
		return nil, err
	}
	if supplement.Schema != 1 {
		return nil, fmt.Errorf("supplement schema")
	}
	for k, a := range supplement.Accounts {
		if len(a.Data) != 2 || a.Data[1] != "base64" {
			return nil, fmt.Errorf("invalid supplemental encoding")
		}
		data, e := base64.StdEncoding.DecodeString(a.Data[0])
		if e != nil {
			return nil, e
		}
		accounts[k] = account{a.Owner, data, a.Lamports}
	}
	return accounts, nil
}
func prepare(root string, buy, direct bool, hash sol.Hash) (trading.PumpSwapRouteSelection, trading.CachedTradeRequest, error) {
	search, r, e := pumpswapreplay.LoadSnapshot(filepath.Join(root, "examples/fixtures/pumpswap_ttwo_route_accounts.json"), wallet().PublicKey(), buy)
	if e != nil {
		return trading.PumpSwapRouteSelection{}, r, e
	}
	if direct {
		search.AssetMint = sol.MustPublicKeyFromBase58("TTWofwAge91oFhZs7kpQdyrVRkmevgM88xijGvQFbKo")
		search.ConversionPools = nil
		search.MaximumConversionHops = 0
		if buy {
			r.Amount = 10000
		}
	}
	r.RecentBlockhash = hash
	selected, e := trading.PrepareBestPumpSwapTrade(search, r)
	return selected, r, e
}
func routeFor(selected trading.PumpSwapRouteSelection, r trading.CachedTradeRequest) (subscription.PreparedCachedRoute, error) {
	hints := []subscription.PoolTradeHint{}
	for _, s := range selected.Trade.Steps() {
		hints = append(hints, s.PoolTradeHint)
	}
	return r.Snapshot.PrepareRoute(hints, r.Context, r.UnixTimestamp, r.Payer, r.Amount, *r.SlippageBps, r.MaximumArrays)
}
func tokenAccount(accounts map[string]account, address, mint, owner, program sol.PublicKey, amount uint64) error {
	// Preserve captured Token2022 extensions. Empty extension buffers are invalid.
	var template []byte
	keys := make([]string, 0, len(accounts))
	for k := range accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := accounts[k]
		if a.Owner == program.String() && len(a.Data) >= 165 && bytes.Equal(a.Data[:32], mint[:]) {
			template = a.Data
			break
		}
	}
	if template == nil {
		return fmt.Errorf("missing token template %s", mint)
	}
	data := append([]byte(nil), template...)
	copy(data[:32], mint[:])
	copy(data[32:64], owner[:])
	binary.LittleEndian.PutUint64(data[64:72], amount)
	clear(data[72:108])
	data[108] = 1
	clear(data[109:165])
	accounts[address.String()] = account{Owner: program.String(), Data: data}
	return nil
}
func populate(accounts map[string]account, route subscription.PreparedCachedRoute, payer sol.PublicKey) error {
	for _, ix := range route.SetupInstructions {
		a := ix.Accounts()
		if len(a) >= 6 {
			if e := tokenAccount(accounts, a[1].PublicKey, a[3].PublicKey, payer, a[5].PublicKey, 0); e != nil {
				return e
			}
		}
	}
	for _, ix := range route.SwapInstructions {
		if ix.ProgramID() != instruction.PUMPSWAP_PROGRAM {
			continue
		}
		a := ix.Accounts()
		for _, pair := range [][2]int{{5, 1}, {6, 1}, {10, 9}, {17, 18}, {len(a) - 1, len(a) - 2}} {
			idx, own := pair[0], pair[1]
			mint, program := a[4].PublicKey, a[12].PublicKey
			if idx == 5 {
				mint, program = a[3].PublicKey, a[11].PublicKey
			}
			amount := uint64(0)
			if (idx == 5 || idx == 6) && mint == route.Legs[0].Hint.InputMint {
				amount = 1000000000000
			}
			if e := tokenAccount(accounts, a[idx].PublicKey, mint, a[own].PublicKey, program, amount); e != nil {
				return e
			}
		}
	}
	return nil
}
func runCase(root, engine string, buy, direct, alt, slippage bool) (bankResult, error) {
	var result bankResult
	selected, r, e := prepare(root, buy, direct, sol.Hash{})
	if e != nil {
		return result, e
	}
	route, e := routeFor(selected, r)
	if e != nil {
		return result, e
	}
	accounts, e := loadAccounts(root)
	if e != nil {
		return result, e
	}
	if e = populate(accounts, route, r.Payer); e != nil {
		return result, e
	}
	addresses := sol.PublicKeySlice{}
	seen := map[sol.PublicKey]bool{}
	for _, ix := range append(route.SetupInstructions, route.SwapInstructions...) {
		for _, a := range ix.Accounts() {
			if !seen[a.PublicKey] {
				addresses = append(addresses, a.PublicKey)
				seen[a.PublicKey] = true
			}
		}
	}
	table := sol.PublicKey{191, 11}
	opts := trading.PumpSwapV0Options{GasConfig: &trading.GasFeeConfig{ComputeUnitLimit: 1000000}}
	if alt {
		data := make([]byte, 56)
		binary.LittleEndian.PutUint32(data, 1)
		binary.LittleEndian.PutUint64(data[4:12], ^uint64(0))
		for _, k := range addresses {
			data = append(data, k[:]...)
		}
		accounts[table.String()] = account{Owner: "AddressLookupTab1e1111111111111111111111111", Data: data}
		opts.AddressLookupTables = map[sol.PublicKey]sol.PublicKeySlice{table: addresses}
	}
	tracked := []string{r.Payer.String()}
	temporary, e := sol.CreateWithSeed(r.Payer, r.TemporaryWsolSeed, sol.TokenProgramID)
	if e != nil {
		return result, e
	}
	tracked = append(tracked, temporary.String())
	for k := range accounts {
		tracked = append(tracked, k)
	}
	for _, ix := range route.SwapInstructions {
		for _, a := range ix.Accounts() {
			tracked = append(tracked, a.PublicKey.String())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, engine)
	cmd.Stderr = os.Stderr
	input, e := cmd.StdinPipe()
	if e != nil {
		return result, e
	}
	output, e := cmd.StdoutPipe()
	if e != nil {
		return result, e
	}
	if e = cmd.Start(); e != nil {
		return result, e
	}
	defer func() { input.Close(); cmd.Process.Kill(); cmd.Wait() }()
	encoder, decoder := json.NewEncoder(input), json.NewDecoder(output)
	req := initRequest{ProgramsPath: filepath.Join(root, "examples/pumpswap_bank/programs.json.gz"), ProgramsSHA256: "24e5a1085d333648394a70dfc39bb1cee3a6b17fd7ec841d7713af5b5f5135fe", Payer: r.Payer.String(), Clock: [5]int64{int64(r.Context.Slot), 0, int64(r.Context.Epoch), int64(r.Context.Epoch), int64(r.UnixTimestamp)}, Accounts: accounts, Tracked: tracked}
	if e = encoder.Encode(req); e != nil {
		return result, e
	}
	var hello struct{ Blockhash string }
	if e = decoder.Decode(&hello); e != nil {
		return result, e
	}
	hash, e := sol.HashFromBase58(hello.Blockhash)
	if e != nil {
		return result, e
	}
	selected, r, e = prepare(root, buy, direct, hash)
	if e != nil {
		return result, e
	}
	wire, e := selected.Trade.BuildV0Transaction(r.Context, []sol.PrivateKey{wallet()}, opts)
	if e != nil {
		return result, e
	}
	var badALT []byte
	if alt {
		bad := opts
		bad.AddressLookupTables = map[sol.PublicKey]sol.PublicKeySlice{{192, 12}: addresses}
		badALT, e = selected.Trade.BuildV0Transaction(r.Context, []sol.PrivateKey{wallet()}, bad)
		if e != nil {
			return result, e
		}
	}
	if slippage {
		tx, err := sol.TransactionFromBytes(wire)
		if err != nil {
			return result, err
		}
		// Fail the final swap after earlier route legs have executed, then
		// verify bank rollback. Settlement close instructions follow the swap.
		lastSwap := route.SwapInstructions[len(route.SwapInstructions)-1]
		original, err := lastSwap.Data()
		if err != nil {
			return result, err
		}
		found := false
		for i := range tx.Message.Instructions {
			compiled := &tx.Message.Instructions[i]
			if bytes.Equal(compiled.Data, original) {
				offset := 16
				if lastSwap.ProgramID() == subscription.AmmV4Program {
					offset = 9
				}
				binary.LittleEndian.PutUint64(compiled.Data[offset:offset+8], ^uint64(0))
				found = true
				break
			}
		}
		if !found {
			return result, fmt.Errorf("final swap not found")
		}
		_, e = tx.Sign(func(k sol.PublicKey) *sol.PrivateKey {
			v := wallet()
			if k == v.PublicKey() {
				return &v
			}
			return nil
		})
		if e != nil {
			return result, e
		}
		wire, e = tx.MarshalBinary()
		if e != nil {
			return result, e
		}
	}
	tx, e := sol.TransactionFromBytes(wire)
	if e != nil {
		return result, e
	}
	if e = tx.VerifySignatures(); e != nil {
		return result, e
	}
	if alt && len(tx.Message.AddressTableLookups) == 0 {
		return result, fmt.Errorf("ALT not used")
	}
	if e = encoder.Encode(struct{ Wire, InvalidALTWire []byte }{wire, badALT}); e != nil {
		return result, e
	}
	if e = decoder.Decode(&result); e != nil {
		remaining, _ := io.ReadAll(output)
		return result, fmt.Errorf("bank response: %w %s", e, remaining)
	}
	if result.OK == slippage || result.CommitOK != result.OK || !result.SimulationUnchanged || !result.CommitMatchesSimulation || !result.RollbackClean || !result.SignatureRejected || (alt && !result.InvalidALTRejected) {
		return result, fmt.Errorf("bank invariant failed: success=%t commit=%t readonly=%t poststate=%t rollback=%t signature=%t ALT=%t error=%s logs=%v", result.OK, result.CommitOK, result.SimulationUnchanged, result.CommitMatchesSimulation, result.RollbackClean, result.SignatureRejected, result.InvalidALTRejected, result.Error, result.Logs)
	}
	if result.Fee != 5000 {
		return result, fmt.Errorf("unexpected transaction fee %d", result.Fee)
	}
	if slippage {
		code := 6040
		if !buy {
			code = 6004
			if !direct {
				code = 30
			}
		}
		if !strings.Contains(result.Error, fmt.Sprintf(`"Custom":%d`, code)) {
			return result, fmt.Errorf("unexpected rejection cause: %s", result.Error)
		}
		// A final-leg failure must have actually executed the preceding swaps.
		for _, ix := range route.SwapInstructions[:len(route.SwapInstructions)-1] {
			if !strings.Contains(strings.Join(result.Logs, "\n"), "Program "+ix.ProgramID().String()+" success") {
				return result, fmt.Errorf("earlier swap did not execute before rollback")
			}
		}
		if result.Before[r.Payer.String()].Lamports-result.After[r.Payer.String()].Lamports != result.Fee {
			return result, fmt.Errorf("failed transaction changed payer beyond fee")
		}
	} else if direct {
		a := route.SwapInstructions[0].Accounts()
		input, output := 6, 5
		if !buy {
			input, output = 5, 6
		}
		amount := func(state map[string]*sol.Account, index int) uint64 {
			return binary.LittleEndian.Uint64(state[a[index].PublicKey.String()].Data[64:72])
		}
		debit, credit := amount(result.Before, input)-amount(result.After, input), amount(result.After, output)-amount(result.Before, output)
		if debit != r.Amount || credit < selected.Trade.MinimumOutput() {
			return result, fmt.Errorf("execution balance mismatch: debit=%d input=%d credit=%d minimum=%d", debit, r.Amount, credit, selected.Trade.MinimumOutput())
		}
	} else {
		var rent int64
		for k, after := range result.After {
			before := result.Before[k]
			if k == r.Payer.String() {
				continue
			}
			if before == nil && after != nil {
				rent += int64(after.Lamports)
			}
			if before != nil && after == nil {
				rent -= int64(before.Lamports)
			}
		}
		lamports := int64(result.After[r.Payer.String()].Lamports) - int64(result.Before[r.Payer.String()].Lamports) + int64(result.Fee) + rent
		pumpIndex := len(route.SwapInstructions) - 1
		if !buy {
			pumpIndex = 0
		}
		token := route.SwapInstructions[pumpIndex].Accounts()[5].PublicKey.String()
		before := binary.LittleEndian.Uint64(result.Before[token].Data[64:72])
		after := binary.LittleEndian.Uint64(result.After[token].Data[64:72])
		if buy {
			if lamports != -int64(r.Amount) || after < before || after-before < selected.Trade.MinimumOutput() {
				return result, fmt.Errorf("native buy settlement mismatch: SOL=%d token=%d", lamports, after-before)
			}
		} else if before < after || before-after != r.Amount || lamports < int64(selected.Trade.MinimumOutput()) {
			return result, fmt.Errorf("native sell settlement mismatch: SOL=%d token=%d", lamports, before-after)
		}
		if result.After[temporary.String()] != nil {
			return result, fmt.Errorf("temporary WSOL not closed")
		}
	}
	return result, nil
}
func buildEngine(root string) (string, func(), error) {
	dir, e := os.MkdirTemp("", "pumpswap-go-bank-")
	if e != nil {
		return "", nil, e
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "engine")
	cmd := exec.Command("go", "build", "-o", path, ".")
	cmd.Dir = filepath.Join(root, "examples/pumpswap_offline_bank/engine")
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		cleanup()
		return "", nil, e
	}
	return path, cleanup, nil
}
func main() {
	root, e := os.Getwd()
	must(e)
	engine, cleanup, e := buildEngine(root)
	must(e)
	defer cleanup()
	for _, buy := range []bool{true, false} {
		for _, direct := range []bool{true, false} {
			for _, alt := range []bool{false, true} {
				if !direct && !alt {
					continue
				}
				for _, fail := range []bool{false, true} {
					res, e := runCase(root, engine, buy, direct, alt, fail)
					must(e)
					fmt.Printf("direct=%t buy=%t ALT=%t slippage=%t CU=%d fee=%d OK=%t\n", direct, buy, alt, fail, res.Units, res.Fee, res.OK)
				}
			}
		}
	}
}
