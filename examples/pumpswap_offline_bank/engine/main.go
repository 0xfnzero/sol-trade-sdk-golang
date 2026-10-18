// Isolated Go VM process avoids duplicate Yellowstone protobuf registrations.
// Every transaction is built/signed by the native SDK parent; no RPC is used.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	litesvm "github.com/LiteSVM/litesvm-go"
	sol "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/sysvar"
	"io"
	"os"
	"reflect"
	"sort"
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
type result struct {
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
func key(s string) sol.PublicKey { return sol.MustPublicKeyFromBase58(s) }
func state(bank *litesvm.LiteSVM, tracked []string) map[string]*sol.Account {
	out := map[string]*sol.Account{}
	for _, k := range tracked {
		out[k] = bank.GetAccount(key(k))
	}
	return out
}
func run() {
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	var request initRequest
	must(decoder.Decode(&request))
	raw, err := os.ReadFile(request.ProgramsPath)
	must(err)
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != request.ProgramsSHA256 {
		panic("program bundle checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	must(err)
	data, err := io.ReadAll(gz)
	must(err)
	must(gz.Close())
	var programs struct {
		Schema   int
		Programs map[string]string
	}
	must(json.Unmarshal(data, &programs))
	if programs.Schema != 1 {
		panic("program schema")
	}
	bank, err := litesvm.New()
	must(err)
	defer bank.Close()
	must(bank.SetSigverify(true))
	must(bank.SetBlockhashCheck(true))
	must(bank.SetLogBytesLimit(1000000))
	c := request.Clock
	must(bank.SetClock(&sysvar.Clock{Slot: uint64(c[0]), EpochStartTimestamp: c[1], Epoch: uint64(c[2]), LeaderScheduleEpoch: uint64(c[3]), UnixTimestamp: c[4]}))
	keys := []string{}
	for k := range programs.Programs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		code, e := base64.StdEncoding.DecodeString(programs.Programs[k])
		must(e)
		must(bank.AddProgram(key(k), code))
	}
	keys = nil
	for k := range request.Accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == sol.SysVarClockPubkey.String() {
			continue
		}
		a := request.Accounts[k]
		lamports := a.Lamports
		if lamports == 0 {
			lamports, err = bank.MinimumBalanceForRentExemption(len(a.Data))
			must(err)
		}
		value, e := litesvm.NewAccount(lamports, a.Data, key(a.Owner), false, 0)
		must(e)
		must(bank.SetAccount(key(k), value))
	}
	must(bank.Airdrop(key(request.Payer), 500000000000))
	hash, err := bank.LatestBlockhash()
	must(err)
	must(encoder.Encode(struct{ Blockhash string }{hash.String()}))
	var transaction struct{ Wire, InvalidALTWire []byte }
	must(decoder.Decode(&transaction))
	before := state(bank, request.Tracked)
	sim, err := bank.SimulateVersionedTransaction(transaction.Wire)
	must(err)
	unchanged := reflect.DeepEqual(before, state(bank, request.Tracked))
	committed, err := bank.SendVersionedTransaction(transaction.Wire)
	must(err)
	after := state(bank, request.Tracked)
	res := result{OK: sim.IsOk(), CommitOK: committed.IsOk(), Error: sim.Error(), Logs: sim.Logs(), Units: sim.ComputeUnits(), Fee: sim.Fee(), Before: before, After: after, SimulationUnchanged: unchanged, RollbackClean: true, CommitMatchesSimulation: true}
	if sim.IsOk() {
		post, e := sim.PostAccounts()
		must(e)
		for _, a := range post {
			if before[a.Address.String()] != nil || after[a.Address.String()] != nil {
				if !reflect.DeepEqual(after[a.Address.String()], a.Account) {
					res.CommitMatchesSimulation = false
				}
			}
		}
	} else {
		for k, a := range before {
			if k != request.Payer && !reflect.DeepEqual(a, after[k]) {
				res.RollbackClean = false
			}
		}
	}
	bad := append([]byte(nil), transaction.Wire...)
	bad[1] ^= 1
	failed, err := bank.SimulateVersionedTransaction(bad)
	must(err)
	res.SignatureRejected = failed.Error() == `"SignatureFailure"` && len(failed.Logs()) == 0
	if len(transaction.InvalidALTWire) > 0 {
		invalid, e := bank.SimulateVersionedTransaction(transaction.InvalidALTWire)
		must(e)
		res.InvalidALTRejected = invalid.Error() == `"AddressLookupTableNotFound"` && len(invalid.Logs()) == 0
	}
	if !reflect.DeepEqual(after, state(bank, request.Tracked)) {
		panic("bad signature changed bank")
	}
	must(encoder.Encode(res))
}
func main() { run() }
