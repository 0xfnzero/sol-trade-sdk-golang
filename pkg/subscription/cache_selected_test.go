package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"os"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func selectedKey(n uint64) solana.PublicKey {
	var k solana.PublicKey
	binary.LittleEndian.PutUint64(k[:8], n)
	return k
}
func TestSelectedSnapshotIsolationGuardsAndTombstones(t *testing.T) {
	c := &SubscriptionAccountCache{}
	k1, k2, k3 := selectedKey(1), selectedKey(2), selectedKey(3)
	context := CacheReadContext{10, 0, 0}
	source := []byte("one")
	c.UpdateMany([]AccountUpdate{{k1, CachedAccount{token, source, 10, 1}}, {k2, account("one", 10, 1)}, {k3, account("", 10, 1)}})
	s, e := c.SnapshotSelected([]solana.PublicKey{k1, k1, k3})
	if e != nil {
		t.Fatal(e)
	}
	source[0] = 0
	c.Update(k1, account("two", 10, 2))
	a, e := s.Get(k1, context, nil)
	if e != nil {
		t.Fatal(e)
	}
	a.Data[0] = 0
	a, e = s.Get(k1, context, nil)
	if e != nil || string(a.Data) != "one" {
		t.Fatal(a, e)
	}
	if a, e := s.GetOptional(k3, context, nil); a != nil || e != nil {
		t.Fatal(a, e)
	}
	if _, e = s.GetOptional(k2, context, nil); e == nil {
		t.Fatal("omitted identity accepted")
	}
	if _, e = c.SnapshotSelected([]solana.PublicKey{selectedKey(4)}); e == nil {
		t.Fatal("unknown selected identity accepted")
	}
	for _, context := range []CacheReadContext{{9, 0, 0}, {11, 0, 0}} {
		if _, e = s.Get(k1, context, nil); e == nil {
			t.Fatal("freshness bypass")
		}
	}
	wrong := selectedKey(9)
	if _, e = s.Get(k1, CacheReadContext{10, 0, 0}, &wrong); e == nil {
		t.Fatal("owner bypass")
	}
	r, e := NewSubscriptionReadiness("fork", []string{k1.String()})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.ReadySnapshotSelected(r, []solana.PublicKey{k1}); e == nil {
		t.Fatal("unready allowed")
	}
	r.MarkValidated([]string{k1.String()}, 0, "fork")
	ready, e := c.ReadySnapshotSelected(r, []solana.PublicKey{k1})
	if e != nil {
		t.Fatal(e)
	}
	r.Interrupt("disconnect")
	if _, e = ready.Get(k1, CacheReadContext{10, 0, 0}, nil); e == nil {
		t.Fatal("generation bypass")
	}
	c.Update(k1, account("conflict", 10, 2))
	if _, e = s.Get(k1, CacheReadContext{10, 0, 0}, nil); e == nil {
		t.Fatal("fork bypass")
	}
	if _, e = c.SnapshotSelected([]solana.PublicKey{k1}); e == nil {
		t.Fatal("conflicted selected creation allowed")
	}
}
func TestSelectedSnapshotConcurrentBatchCoherence(t *testing.T) {
	c := &SubscriptionAccountCache{}
	keys := []solana.PublicKey{selectedKey(1), selectedKey(2)}
	c.UpdateMany([]AccountUpdate{{keys[0], account("one", 10, 1)}, {keys[1], account("one", 10, 1)}})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := uint64(2); n < 200; n++ {
			if _, e := c.UpdateMany([]AccountUpdate{{keys[0], account("next", 10, n)}, {keys[1], account("next", 10, n)}}); e != nil {
				t.Error(e)
				return
			}
		}
	}()
	for n := 0; n < 200; n++ {
		s, e := c.SnapshotSelected(keys)
		if e != nil {
			t.Fatal(e)
		}
		a, e := s.Get(keys[0], CacheReadContext{10, 0, 0}, nil)
		if e != nil {
			t.Fatal(e)
		}
		b, e := s.Get(keys[1], CacheReadContext{10, 0, 0}, nil)
		if e != nil || a.WriteVersion != b.WriteVersion {
			t.Fatal("mixed batch", a, b, e)
		}
	}
	wg.Wait()
}
func TestSelectedSnapshotRealCpmmDependencies(t *testing.T) {
	raw, e := os.ReadFile("../../examples/fixtures/cpmm_mainnet_20261002.json")
	if e != nil {
		t.Fatal(e)
	}
	var f map[string]json.RawMessage
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}
	c := &SubscriptionAccountCache{}
	keys := []solana.PublicKey{}
	for _, name := range []string{"pool", "config", "base_mint", "quote_mint", "base_vault", "quote_vault"} {
		var a struct {
			Pubkey, Owner, Data, Slot string
			WriteVersion              string `json:"write_version"`
		}
		if e = json.Unmarshal(f[name], &a); e != nil {
			t.Fatal(e)
		}
		data, e := base64.StdEncoding.DecodeString(a.Data)
		if e != nil {
			t.Fatal(e)
		}
		slot, _ := strconv.ParseUint(a.Slot, 10, 64)
		v, _ := strconv.ParseUint(a.WriteVersion, 10, 64)
		k := solana.MustPublicKeyFromBase58(a.Pubkey)
		keys = append(keys, k)
		c.Update(k, CachedAccount{solana.MustPublicKeyFromBase58(a.Owner), data, slot, v})
	}
	number := func(name string) uint64 {
		var v string
		if e := json.Unmarshal(f[name], &v); e != nil {
			t.Fatal(e)
		}
		n, e := strconv.ParseUint(v, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	var age uint64
	json.Unmarshal(f["maximum_slot_age"], &age)
	context := CacheReadContext{number("read_slot"), number("epoch"), age}
	var payer string
	json.Unmarshal(f["payer"], &payer)
	var baseIn bool
	json.Unmarshal(f["base_in"], &baseIn)
	h := PoolTradeHint{keys[0], keys[2], keys[3]}
	if !baseIn {
		h.InputMint, h.OutputMint = h.OutputMint, h.InputMint
	}
	var slip uint16
	json.Unmarshal(f["slippage_bps"], &slip)
	full, e := c.Snapshot().PrepareCpmm(h, context, number("unix_timestamp"), solana.MustPublicKeyFromBase58(payer), number("amount"), slip)
	if e != nil {
		t.Fatal(e)
	}
	s, e := c.SnapshotSelected(keys)
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := s.PrepareCpmm(h, context, number("unix_timestamp"), solana.MustPublicKeyFromBase58(payer), number("amount"), slip)
	if e != nil || !reflect.DeepEqual(full, prepared) {
		t.Fatal("full/selected mismatch", e)
	}
	for omitted := range keys {
		selected := append([]solana.PublicKey{}, keys[:omitted]...)
		selected = append(selected, keys[omitted+1:]...)
		s, e = c.SnapshotSelected(selected)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.PrepareCpmm(h, context, number("unix_timestamp"), solana.MustPublicKeyFromBase58(payer), number("amount"), slip); e == nil {
			t.Fatal("missing dependency accepted", omitted)
		}
	}
}

var selectedBenchmarkSink *AccountCacheSnapshot

func BenchmarkSnapshotScope(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		c := &SubscriptionAccountCache{}
		keys := make([]solana.PublicKey, 6)
		for n := 0; n < size; n++ {
			k := selectedKey(uint64(n + 1))
			if n < 6 {
				keys[n] = k
			}
			c.Update(k, CachedAccount{token, make([]byte, 512), 10, 1})
		}
		b.Run(fmt.Sprintf("full_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				selectedBenchmarkSink = c.Snapshot()
			}
		})
		b.Run(fmt.Sprintf("selected6_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				s, e := c.SnapshotSelected(keys)
				if e != nil {
					b.Fatal(e)
				}
				selectedBenchmarkSink = s
			}
		})
	}
}
