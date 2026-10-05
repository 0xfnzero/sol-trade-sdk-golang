package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	f "github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"math"
	"os"
	"strconv"
	"testing"
)

type cpmmCaptureAccount struct {
	Pubkey, Owner string
	Data          string `json:"data_base64"`
	Lamports      uint64
}
type cpmmCapture struct {
	Name, Pool, Payer string
	Permissionless    bool
	ShareRate         uint64 `json:"share_rate"`
	ConfigShareRate   uint64 `json:"config_share_rate"`
	SharePDA          string `json:"share_pda"`
	Accounts          []*cpmmCaptureAccount
	Instruction       struct {
		Accounts []string
		Data     string `json:"data_base64"`
		Program  string `json:"program_id"`
	}
	Checks []struct {
		Gross    string
		Payout   string `json:"creator_before_transfer_fee"`
		Protocol string `json:"protocol_share"`
	}
}

func cpmmCases(t *testing.T) []cpmmCapture {
	t.Helper()
	b, e := os.ReadFile("../instruction/testdata/cpmm_creator_fee_rust_5_0_7.json")
	if e != nil {
		t.Fatal(e)
	}
	var c []cpmmCapture
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func cpmmRaw(t *testing.T, a *cpmmCaptureAccount) []byte {
	t.Helper()
	if a == nil {
		return nil
	}
	b, e := base64.StdEncoding.DecodeString(a.Data)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func cpmmPK(s string) solana.PublicKey { return solana.MustPublicKeyFromBase58(s) }
func TestCpmmCreatorFeeRustMainnetParity(t *testing.T) {
	for _, c := range cpmmCases(t) {
		t.Run(c.Name+strconv.FormatBool(c.Permissionless), func(t *testing.T) {
			pool, e := f.DecodeCpmmCollectionPool(cpmmRaw(t, c.Accounts[0]))
			if e != nil {
				t.Fatal(e)
			}
			config, e := f.DecodeCpmmAmmConfig(cpmmRaw(t, c.Accounts[1]))
			if e != nil {
				t.Fatal(e)
			}
			if config.CreatorFeeShareRate != c.ConfigShareRate {
				t.Fatal("config rate")
			}
			pda, e := f.GetCreatorFeeSharePDA(pool.PoolCreator, pool.AmmConfig)
			if e != nil || pda.String() != c.SharePDA {
				t.Fatal("PDA")
			}
			var share *rpc.Account
			if a := c.Accounts[2]; a != nil {
				share = &rpc.Account{Owner: cpmmPK(a.Owner), Lamports: a.Lamports, Data: rpc.DataBytesOrJSONFromBytes(cpmmRaw(t, a))}
			}
			rate, e := f.ResolveCreatorFeeShareRate(config, pool.PoolCreator, pool.AmmConfig, share)
			if e != nil || rate != c.ShareRate {
				t.Fatal("rate", e)
			}
			var payer *solana.PublicKey
			var ix solana.Instruction
			if c.Permissionless {
				p := cpmmPK(c.Payer)
				payer = &p
				ix, e = f.CollectCreatorFeePermissionless(p, cpmmPK(c.Pool), pool)
			} else {
				ix, e = f.CollectCreatorFee(cpmmPK(c.Pool), pool)
			}
			if e != nil {
				t.Fatal(e)
			}
			if len(ix.Accounts()) != len(c.Instruction.Accounts) {
				t.Fatal("length")
			}
			for i, m := range ix.Accounts() {
				w := i == 0 || i == 4 || i == 5 || i == 8 || i == 9 || (!c.Permissionless && i == 2) || (c.Permissionless && i == 3)
				if m.PublicKey.String() != c.Instruction.Accounts[i] || m.IsSigner != (i == 0) || m.IsWritable != w {
					t.Fatalf("meta %d", i)
				}
			}
			data, _ := ix.Data()
			wire, _ := base64.StdEncoding.DecodeString(c.Instruction.Data)
			if !bytes.Equal(data, wire) || ix.ProgramID().String() != c.Instruction.Program {
				t.Fatal("instruction bytes")
			}
			for _, check := range c.Checks {
				gross, _ := strconv.ParseUint(check.Gross, 10, 64)
				p, _ := strconv.ParseUint(check.Payout, 10, 64)
				q, _ := strconv.ParseUint(check.Protocol, 10, 64)
				a, b, e := f.SplitCreatorFee(gross, rate)
				if e != nil || a != p || b != q {
					t.Fatal("payout")
				}
			}
			cache := &SubscriptionAccountCache{}
			ctx := CacheReadContext{100, 0, 5}
			for _, a := range c.Accounts {
				k, owner := pda, solana.PublicKey{}
				if a != nil {
					k = cpmmPK(a.Pubkey)
					owner = cpmmPK(a.Owner)
				}
				if _, e = cache.Update(k, CachedAccount{owner, cpmmRaw(t, a), 100, 1}); e != nil {
					t.Fatal(e)
				}
			}
			snapshot := cache.Snapshot()
			prepared, e := snapshot.PrepareCpmmCreatorFeeCollection(cpmmPK(c.Pool), payer, ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = snapshot.ValidateCpmmCreatorFeeCollection(prepared, payer, CacheReadContext{101, 0, 5}); e != nil {
				t.Fatal(e)
			}
			bad := *prepared
			bad.CreatorPayoutToken0++
			if snapshot.ValidateCpmmCreatorFeeCollection(&bad, payer, ctx) == nil {
				t.Fatal("tampered payout accepted")
			}
			if snapshot.ValidateCpmmCreatorFeeCollection(prepared, payer, CacheReadContext{99, 0, 5}) == nil {
				t.Fatal("backwards accepted")
			}
			observation, _ := snapshot.GetObservation(pda, ctx)
			observation.WriteVersion++
			cache.Update(pda, observation)
			if cache.Snapshot().ValidateCpmmCreatorFeeCollection(prepared, payer, ctx) == nil {
				t.Fatal("version change accepted")
			}
			if _, e = snapshot.PrepareCpmmCreatorFeeCollection(cpmmPK(c.Pool), payer, CacheReadContext{106, 0, 5}); e == nil {
				t.Fatal("stale accepted")
			}
		})
	}
}
func TestCpmmCreatorFeeFallbackAndBounds(t *testing.T) {
	a, b, e := f.SplitCreatorFee(math.MaxUint64, 1000000)
	if e != nil || a != 0 || b != math.MaxUint64 {
		t.Fatal("u64 rounding")
	}
	a, b, e = f.SplitCreatorFee(1, 50000)
	if e != nil || a != 1 || b != 0 {
		t.Fatal("dust")
	}
	if _, _, e = f.SplitCreatorFee(1, 1000001); e == nil {
		t.Fatal("invalid rate")
	}
	var c cpmmCapture
	for _, candidate := range cpmmCases(t) {
		if candidate.Name == "override" {
			c = candidate
			break
		}
	}
	config, _ := f.DecodeCpmmAmmConfig(cpmmRaw(t, c.Accounts[1]))
	pool, _ := f.DecodeCpmmCollectionPool(cpmmRaw(t, c.Accounts[0]))
	data := cpmmRaw(t, c.Accounts[2])
	resolve := func(owner solana.PublicKey, b []byte, lamports uint64) (uint64, error) {
		return f.ResolveCreatorFeeShareRate(config, pool.PoolCreator, pool.AmmConfig, &rpc.Account{Owner: owner, Data: rpc.DataBytesOrJSONFromBytes(b), Lamports: lamports})
	}
	for _, share := range []*rpc.Account{nil, {Owner: f.RAYDIUM_CPMM_PROGRAM, Lamports: 0, Data: rpc.DataBytesOrJSONFromBytes(data)}, {Owner: solana.PublicKey{}, Lamports: 1, Data: rpc.DataBytesOrJSONFromBytes([]byte{1})}} {
		r, e := f.ResolveCreatorFeeShareRate(config, pool.PoolCreator, pool.AmmConfig, share)
		if e != nil || r != config.CreatorFeeShareRate {
			t.Fatal("fallback")
		}
	}
	if _, e = resolve(f.RAYDIUM_CPMM_PROGRAM, data[:144], 1); e == nil {
		t.Fatal("truncated share")
	}
	binary.LittleEndian.PutUint64(data[73:81], 0)
	if r, e := resolve(f.RAYDIUM_CPMM_PROGRAM, data, 1); e != nil || r != 0 {
		t.Fatal("zero override")
	}
	binary.LittleEndian.PutUint64(data[73:81], 1000001)
	if _, e = resolve(f.RAYDIUM_CPMM_PROGRAM, data, 1); e == nil {
		t.Fatal("invalid override")
	}
	if _, e = f.DecodeCpmmAmmConfig(cpmmRaw(t, c.Accounts[1])[:235]); e == nil {
		t.Fatal("truncated config")
	}
	if _, e = f.DecodeCpmmCollectionPool(cpmmRaw(t, c.Accounts[0])[:636]); e == nil {
		t.Fatal("truncated pool")
	}
	cache := &SubscriptionAccountCache{}
	for _, a := range c.Accounts[:2] {
		cache.Update(cpmmPK(a.Pubkey), CachedAccount{cpmmPK(a.Owner), cpmmRaw(t, a), 100, 1})
	}
	if _, e = cache.Snapshot().PrepareCpmmCreatorFeeCollection(cpmmPK(c.Pool), nil, CacheReadContext{100, 0, 0}); e == nil {
		t.Fatal("unknown PDA treated as absence")
	}
}

func TestCpmmCreatorFeeInvalidConfigAndProtocolOverflow(t *testing.T) {
	c := cpmmCases(t)[0]
	cache := &SubscriptionAccountCache{}
	for _, a := range c.Accounts {
		key, owner := cpmmPK(c.SharePDA), solana.PublicKey{}
		data := cpmmRaw(t, a)
		if a != nil {
			key = cpmmPK(a.Pubkey)
			owner = cpmmPK(a.Owner)
		}
		if key == cpmmPK(c.Pool) {
			binary.LittleEndian.PutUint64(data[341:349], math.MaxUint64)
		}
		cache.Update(key, CachedAccount{owner, data, 100, 1})
	}
	if _, err := cache.Snapshot().PrepareCpmmCreatorFeeCollection(cpmmPK(c.Pool), nil, CacheReadContext{100, 0, 0}); err == nil {
		t.Fatal("protocol counter overflow accepted")
	}
	b := cpmmRaw(t, c.Accounts[1])
	b[0] ^= 1
	if _, err := f.DecodeCpmmAmmConfig(b); err == nil {
		t.Fatal("wrong discriminator accepted")
	}
}
