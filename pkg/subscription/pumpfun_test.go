package subscription

import (
	"encoding/binary"
	"encoding/json"
	"github.com/0xfnzero/sol-trade-sdk-golang/pkg/instruction"
	"github.com/gagliardetto/solana-go"
	"os"
	"strconv"
	"testing"
)

type currentPumpVector struct {
	Quote      string `json:"quote"`
	VQ         string `json:"vq"`
	Supply     string `json:"supply"`
	HasCreator bool   `json:"has_creator"`
	Override   string `json:"creator_override"`
	Mayhem     bool   `json:"mayhem"`
	Buy        bool   `json:"buy"`
	Amount     string `json:"amount"`
	Out        string `json:"expected_out"`
	Protocol   string `json:"expected_protocol"`
	Creator    string `json:"expected_creator"`
}

func currentNumber(v string) uint64 {
	n, e := strconv.ParseUint(v, 10, 64)
	if e != nil {
		panic(e)
	}
	return n
}
func currentFeeFixture() []byte {
	_, bump, e := solana.FindProgramAddress([][]byte{[]byte("fee_config"), instruction.PUMPFUN_PROGRAM[:]}, instruction.PUMPFUN_FEE_PROGRAM)
	if e != nil {
		panic(e)
	}
	d := append([]byte{143, 52, 146, 187, 219, 123, 76, 155, bump}, make([]byte, 32)...)
	put := func(n uint64) { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, n); d = append(d, b...) }
	fees := func(p, c uint64) { put(0); put(p); put(c) }
	count := func(n uint32) { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, n); d = append(d, b...) }
	tier := func(t, p, c uint64) { put(t); put(0); fees(p, c) }
	fees(19, 11)
	count(2)
	tier(1_000_000_000_000, 91, 31)
	tier(2_000_000_000_000, 51, 21)
	count(1)
	tier(1_000_000_000_000, 15, 3)
	fees(23, 7)
	return d
}
func currentStateFixture(v currentPumpVector) (*AccountCacheSnapshot, PoolTradeHint) {
	mint := solana.MustPublicKeyFromBase58("mtCXje1XCpF8Z3BptaJ4AngDERanJtC9grXicrHpump")
	q := solana.MustPublicKeyFromBase58(v.Quote)
	quote := q
	if quote.IsZero() {
		quote = pumpWSOL
	}
	pool := instruction.GetBondingCurvePDA(mint)
	c := make([]byte, 125)
	copy(c, []byte{23, 183, 248, 55, 96, 216, 172, 96})
	for o, n := range map[int]uint64{8: 1_000_000_000_000, 16: currentNumber(v.VQ), 24: 800_000_000_000, 32: 1_000_000_000_000, 40: currentNumber(v.Supply), 115: currentNumber(v.Override)} {
		binary.LittleEndian.PutUint64(c[o:], n)
	}
	if v.HasCreator {
		copy(c[49:], pumpWSOL[:])
	}
	if v.Mayhem {
		c[81] = 1
	}
	copy(c[83:], q[:])
	g := make([]byte, 1054)
	copy(g, []byte{167, 232, 232, 177, 200, 108, 114, 127})
	g[8] = 1
	g[1045] = 1
	binary.LittleEndian.PutUint64(g[1046:], 100)
	m := make([]byte, 82)
	m[45] = 1
	binary.LittleEndian.PutUint64(m[36:], currentNumber(v.Supply))
	qm := make([]byte, 82)
	qm[45] = 1
	token := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	s := &AccountCacheSnapshot{accounts: map[solana.PublicKey]CachedAccount{
		pool:                               {Owner: instruction.PUMPFUN_PROGRAM, Data: c, Slot: 100},
		instruction.PUMPFUN_GLOBAL_ACCOUNT: {Owner: instruction.PUMPFUN_PROGRAM, Data: g, Slot: 100},
		instruction.PUMPFUN_FEE_CONFIG:     {Owner: instruction.PUMPFUN_FEE_PROGRAM, Data: currentFeeFixture(), Slot: 100},
		mint:                               {Owner: token, Data: m, Slot: 100}, quote: {Owner: token, Data: qm, Slot: 100},
	}}
	h := PoolTradeHint{pool, mint, quote}
	if v.Buy {
		h.InputMint, h.OutputMint = quote, mint
	}
	return s, h
}
func currentVectors(t *testing.T) []currentPumpVector {
	t.Helper()
	b, e := os.ReadFile("testdata/pumpfun_current_fee_oracle_2_0_0.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Vectors []currentPumpVector `json:"vectors"`
	}
	if e = json.Unmarshal(b, &corpus); e != nil {
		t.Fatal(e)
	}
	return corpus.Vectors
}
func TestCurrentPumpFunOfficialOracle(t *testing.T) {
	for i, v := range currentVectors(t) {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s, h := currentStateFixture(v)
			state, e := s.PumpFun(h, configContext)
			if e != nil {
				t.Fatal(e)
			}
			r, e := QuoteCachedPumpFunExactIn(state, currentNumber(v.Amount), v.Buy, 100)
			if e != nil {
				t.Fatal(e)
			}
			expected := currentNumber(v.Out)
			if r.EstimatedNetAmountOut != expected || r.MinimumNetAmountOut != expected*9900/10000 || r.Fees.ProtocolFeeBps != currentNumber(v.Protocol) || r.Fees.CreatorFeeBps != currentNumber(v.Creator) {
				t.Fatal("official quote mismatch", r, v)
			}
		})
	}
}
func TestCurrentPumpFunInvalidState(t *testing.T) {
	v := currentVectors(t)[0]
	for _, failure := range []string{"owner", "stale", "complete", "missing-fee", "bump", "gate", "override", "partial"} {
		t.Run(failure, func(t *testing.T) {
			s, h := currentStateFixture(v)
			f := instruction.PUMPFUN_FEE_CONFIG
			g := instruction.PUMPFUN_GLOBAL_ACCOUNT
			switch failure {
			case "owner":
				a := s.accounts[f]
				a.Owner = solana.PublicKey{}
				s.accounts[f] = a
			case "stale":
				a := s.accounts[f]
				a.Slot = 94
				s.accounts[f] = a
			case "complete":
				s.accounts[h.Pool].Data[48] = 1
			case "missing-fee":
				delete(s.accounts, f)
			case "bump":
				s.accounts[f].Data[8] ^= 1
			case "gate":
				s.accounts[g].Data[1045] = 2
			case "override":
				binary.LittleEndian.PutUint64(s.accounts[h.Pool].Data[115:], 101)
			case "partial":
				a := s.accounts[h.Pool]
				a.Data = a.Data[:120]
				s.accounts[h.Pool] = a
			}
			if _, e := s.PumpFun(h, configContext); e == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func TestCurrentPumpFunRejectsInactiveMalformedCreatorFees(t *testing.T) {
	for _, v := range currentVectors(t) {
		if v.HasCreator {
			continue
		}
		s, h := currentStateFixture(v)
		state, err := s.PumpFun(h, configContext)
		if err != nil {
			t.Fatal(err)
		}
		for _, fees := range []PumpFunCurrentFees{{0, 10001}, {9999, 2}, {10001, 0}} {
			if v.Buy {
				state.BuyFees = fees
			} else {
				state.SellFees = fees
			}
			if _, err := QuoteCachedPumpFunExactIn(state, currentNumber(v.Amount), v.Buy, 0); err == nil {
				t.Fatal("invalid inactive creator fee accepted", fees)
			}
		}
	}
}

func pumpRouteFixture(buy bool) (*AccountCacheSnapshot, PoolTradeHint) {
	v := currentPumpVector{Quote: pumpWSOL.String(), VQ: "1000000000000", Supply: "1000000000000000", HasCreator: true, Override: "0", Buy: buy, Amount: "1000000"}
	s, h := currentStateFixture(v)
	mint := h.InputMint
	if buy {
		mint = h.OutputMint
	}
	a := s.accounts[mint]
	a.Owner = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	s.accounts[mint] = a
	g := s.accounts[instruction.PUMPFUN_GLOBAL_ACCOUNT].Data
	copy(g[41:73], pumpWSOL[:])
	copy(g[741:773], pumpWSOL[:])
	s.accounts[instruction.GetPumpFunFeeSharingConfigPDA(mint)] = CachedAccount{Slot: 100}
	return s, h
}

func TestPumpRouteDoesNotRepeatStateValidation(t *testing.T) {
	for _, buy := range []bool{true, false} {
		s, h := pumpRouteFixture(buy)
		reads := 0
		s.continuityGuard = func() error { reads++; return nil }
		prepared, err := s.PreparePumpFun(h, configContext, pumpWSOL, 1000000, 100)
		if err != nil {
			t.Fatal(err)
		}
		directChecks := reads
		reads = 0
		route, err := s.PrepareRoute([]PoolTradeHint{h}, configContext, 0, pumpWSOL, 1000000, 100, 8, true)
		if err != nil {
			t.Fatal(err)
		}
		// Route additionally reads pool owner and the two setup mints. It must
		// not decode the five-account quote state again before preparation.
		if reads != directChecks+3 || route.MinimumNetAmountOut != prepared.Quote.MinimumNetAmountOut {
			t.Fatal("duplicate validation or changed protection", reads, directChecks)
		}
	}
}

func BenchmarkPumpFunRouteDecode(b *testing.B) {
	for _, reference := range []bool{false, true} {
		name := "single_decode"
		if reference {
			name = "duplicate_decode_reference"
		}
		b.Run(name, func(b *testing.B) {
			s, h := pumpRouteFixture(true)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if reference {
					if _, err := s.PumpFun(h, configContext); err != nil {
						b.Fatal(err)
					}
				}
				if _, err := s.PrepareRoute([]PoolTradeHint{h}, configContext, 0, pumpWSOL, 1000000, 100, 8, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
