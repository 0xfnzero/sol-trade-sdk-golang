// Official PumpSwap multi_hop_swap for Pump.fun curves and migrated pools.
// go run ./examples/pump_multi_hop --help
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/examples/internal/exampleutil"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/common"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/instruction"
	"github.com/0xfnzero/sol-trade-sdk-golang/v10/pkg/serialization"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"os"
	"strconv"
	"strings"
	"time"
)

var pump = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
var amm = instruction.PUMPSWAP_PROGRAM
var wsol = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
var token = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
var token2022 = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")

func pda(program solana.PublicKey, seeds ...[]byte) solana.PublicKey {
	key, _, err := solana.FindProgramAddress(seeds, program)
	if err != nil {
		panic(err)
	}
	return key
}
func curveAddress(m solana.PublicKey) solana.PublicKey {
	return pda(pump, []byte("bonding-curve"), m[:])
}
func poolAddress(base, quote solana.PublicKey) solana.PublicKey {
	authority := pda(pump, []byte("pool-authority"), base[:])
	return pda(amm, []byte("pool"), []byte{0, 0}, authority[:], base[:], quote[:])
}
func run() error {
	venue := flag.String("venue", "", "Target venue: pumpfun or pumpswap")
	pathText := flag.String("path", "", "SOL,INTERMEDIATE_MINT,TARGET_MINT (2 or 3 hops, always buy order)")
	side := flag.String("side", "", "buy or sell; sell reverses hops automatically")
	userText := flag.String("user", "", "Funded wallet public key; no private key")
	amountText := flag.String("amount", "", "Positive raw input u64")
	minText := flag.String("min-out", "", "Positive raw output u64; provide a fresh minimum quote")
	simulate := flag.Bool("simulate", false, "Unsigned simulation only; no signing or broadcast")
	reportPath := flag.String("report", "", "Write a new report file; refuse overwrite")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *venue != "pumpfun" && *venue != "pumpswap" || *side != "buy" && *side != "sell" {
		return errors.New("provide --venue pumpfun|pumpswap and --side buy|sell")
	}
	user, err := solana.PublicKeyFromBase58(*userText)
	if err != nil {
		return err
	}
	amount, err := strconv.ParseUint(*amountText, 10, 64)
	if err != nil || amount == 0 {
		return errors.New("amount must be positive raw u64")
	}
	minOut, err := strconv.ParseUint(*minText, 10, 64)
	if err != nil || minOut == 0 {
		return errors.New("min-out must be positive raw u64")
	}
	var path []solana.PublicKey
	seen := map[solana.PublicKey]bool{}
	for _, s := range strings.Split(*pathText, ",") {
		var key solana.PublicKey
		if s == "SOL" || s == "WSOL" {
			key = wsol
		} else {
			key, err = solana.PublicKeyFromBase58(s)
			if err != nil {
				return err
			}
		}
		if seen[key] {
			return errors.New("repeated mint")
		}
		seen[key] = true
		path = append(path, key)
	}
	if len(path) < 3 || len(path) > 4 {
		return errors.New("provide a currency and 2 or 3 hop mints")
	}
	endpoint := os.Getenv("SOLANA_RPC_URL")
	if endpoint == "" {
		endpoint = "https://api.mainnet-beta.solana.com"
	}
	client := rpc.New(endpoint)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	global := pda(amm, []byte("global_config"))
	keys := append([]solana.PublicKey{}, path...)
	for i := 1; i < len(path); i++ {
		keys = append(keys, curveAddress(path[i]), poolAddress(path[i], path[i-1]))
	}
	keys = append(keys, global)
	batch, err := client.GetMultipleAccountsWithOpts(ctx, keys, &rpc.GetMultipleAccountsOpts{Commitment: rpc.CommitmentConfirmed})
	if err != nil {
		return err
	}
	if batch == nil || len(batch.Value) != len(keys) {
		return errors.New("incomplete account batch")
	}
	accounts := map[solana.PublicKey]*rpc.Account{}
	for i, key := range keys {
		accounts[key] = batch.Value[i]
	}
	get := func(key solana.PublicKey) (*rpc.Account, error) {
		a := accounts[key]
		if a == nil {
			return nil, fmt.Errorf("missing account %s", key)
		}
		return a, nil
	}
	programs := make([]solana.PublicKey, len(path))
	for i, mint := range path {
		a, e := get(mint)
		if e != nil {
			return e
		}
		d := a.Data.GetBinary()
		if a.Owner != token && a.Owner != token2022 || len(d) < 82 || d[45] != 1 || d[0] > 1 || !bytes.Equal(d[1:4], []byte{0, 0, 0}) || d[46] > 1 || !bytes.Equal(d[47:50], []byte{0, 0, 0}) {
			return fmt.Errorf("invalid mint %s", mint)
		}
		programs[i] = a.Owner
	}
	g, err := get(global)
	if err != nil {
		return err
	}
	gd := g.Data.GetBinary()
	if g.Owner != amm || len(gd) < 899 || !bytes.Equal(gd[:8], []byte{149, 8, 156, 202, 160, 252, 176, 217}) {
		return errors.New("invalid GlobalConfig")
	}
	var recipient solana.PublicKey
	for i := 0; i < 8; i++ {
		copy(recipient[:], gd[643+i*32:675+i*32])
		if !recipient.IsZero() {
			break
		}
	}
	if recipient.IsZero() {
		return errors.New("no current buyback recipient")
	}
	var hops []instruction.PumpMultiHop
	for i := 1; i < len(path); i++ {
		base, quote := path[i], path[i-1]
		address := curveAddress(base)
		a := accounts[address]
		var curve *common.BondingCurveAccount
		if a != nil {
			curve = common.DecodeBondingCurveAccount(a.Data.GetBinary(), address)
			if a.Owner != pump || curve == nil {
				return errors.New("invalid curve")
			}
		}
		hop := instruction.PumpMultiHop{BaseMint: base, QuoteMint: quote, BaseTokenProgram: programs[i], QuoteTokenProgram: programs[i-1]}
		if curve != nil && !curve.Complete {
			if solana.PublicKey(curve.EffectiveQuoteMint()) != quote {
				return errors.New("curve quote mismatch")
			}
			hop.Venue = "curve"
			hop.Address = address
			hop.BaseVault = common.GetAssociatedTokenAddressFast(address, base, programs[i])
			hop.QuoteVault = common.GetAssociatedTokenAddressFast(address, quote, programs[i-1])
			hop.Mayhem = curve.IsMayhemMode
			hop.Cashback = curve.IsCashbackCoin
		} else {
			address = poolAddress(base, quote)
			a, err = get(address)
			if err != nil {
				return err
			}
			p := instruction.DecodePoolAccount(a.Data.GetBinary())
			if a.Owner != amm || p == nil || p.BaseMint != base || p.QuoteMint != quote {
				return errors.New("invalid canonical pool")
			}
			hop.Venue = "pool"
			hop.Address = address
			hop.BaseVault = p.PoolBaseTokenAccount
			hop.QuoteVault = p.PoolQuoteTokenAccount
			hop.Mayhem = p.IsMayhemMode
			hop.Cashback = p.IsCashbackCoin
			hop.Complete = true
			hop.Index = p.Index
			hop.Creator = p.Creator
		}
		hops = append(hops, hop)
	}
	currency, target := hops[0], hops[len(hops)-1]
	want := "curve"
	if *venue == "pumpswap" {
		want = "pool"
	}
	if target.Venue != want {
		return errors.New("target venue changed; refresh path/venue")
	}
	input, output, inputProgram, outputProgram := currency.QuoteMint, target.BaseMint, currency.QuoteTokenProgram, target.BaseTokenProgram
	route := append([]instruction.PumpMultiHop{}, hops...)
	if *side == "sell" {
		input, output, inputProgram, outputProgram = target.BaseMint, currency.QuoteMint, target.BaseTokenProgram, currency.QuoteTokenProgram
		for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
			route[i], route[j] = route[j], route[i]
		}
	}
	inputATA := instruction.GetAssociatedTokenAddress(user, input, inputProgram)
	var instructions []solana.Instruction
	for _, v := range []struct{ Owner, Mint, Program solana.PublicKey }{{user, input, inputProgram}, {user, output, outputProgram}, {recipient, currency.QuoteMint, currency.QuoteTokenProgram}} {
		ix, _, e := common.BuildCreateIdempotentATA(user, v.Owner, v.Mint, v.Program)
		if e != nil {
			return e
		}
		instructions = append(instructions, ix)
	}
	if *side == "buy" && input == wsol && currency.Venue == "pool" {
		instructions = append(instructions, system.NewTransferInstruction(amount, user, inputATA).Build(), solana.NewInstruction(token, solana.AccountMetaSlice{solana.Meta(inputATA).WRITE()}, []byte{17}))
	}
	swap, err := instruction.SelectPumpRoute(instruction.PumpRouteOptions{Execution: instruction.PumpRouteOfficial, MinimumOutputAmount: &minOut, BuybackRecipient: &recipient}, user, input, output, amount, route)
	if err != nil {
		return err
	}
	instructions = append(instructions, swap)
	block, err := client.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return err
	}
	compute, loaded := uint32(400000), uint32(64*1024*1024)
	compiled, err := serialization.CompileV1Message(user, instructions, block.Value.Blockhash, serialization.V1Config{ComputeUnitLimit: &compute, LoadedAccountsDataSizeLimit: &loaded})
	if err != nil {
		return err
	}
	wire := append(append([]byte{}, compiled.Message...), make([]byte, compiled.RequiredSignatures*64)...)
	if len(wire) > 4096 {
		return errors.New("V1 transaction too large")
	}
	var descriptions []map[string]string
	for _, h := range hops {
		descriptions = append(descriptions, map[string]string{"venue": h.Venue, "address": h.Address.String(), "base_mint": h.BaseMint.String(), "quote_mint": h.QuoteMint.String()})
	}
	report := map[string]any{"signed": false, "broadcast": false, "snapshot_slot": batch.Context.Slot, "side": *side, "amount_in": *amountText, "min_amount_out": *minText, "message_version": 1, "route_in_buy_order": descriptions, "wire": base64.StdEncoding.EncodeToString(wire), "wire_bytes": len(wire)}
	var simulationErr error
	if *simulate {
		response, e := exampleutil.SimulateRaw(ctx, endpoint, wire, batch.Context.Slot)
		if e != nil {
			return e
		}
		report["response"] = json.RawMessage(response)
		simulationErr = exampleutil.ValidateSimulationResponse(response)
	}

	b, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if *reportPath != "" {
		f, e := os.OpenFile(*reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = f.Write(append(b, '\n'))
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	fmt.Println(string(b))

	return simulationErr
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
