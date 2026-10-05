package subscription

import (
	"encoding/binary"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"testing"
)

// The reference measures the former full-map-copy algorithm on accepted writes.
// It is a local microbenchmark, not end-to-end trading or network latency.
func BenchmarkSubscriptionUpdateScaling(b *testing.B) {
	for _, size := range []int{100, 10000} {
		for _, reference := range []bool{false, true} {
			name := "staged"
			if reference {
				name = "full_map_reference"
			}
			b.Run(fmt.Sprintf("accounts_%d/%s", size, name), func(b *testing.B) {
				c := &SubscriptionAccountCache{accounts: make(map[solana.PublicKey]CachedAccount, size)}
				keys := make([]solana.PublicKey, size)
				for i := range keys {
					binary.LittleEndian.PutUint64(keys[i][:], uint64(i+1))
					c.accounts[keys[i]] = CachedAccount{Data: make([]byte, 165), Slot: 1}
				}
				key := keys[0]
				data := make([]byte, 165)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					a := CachedAccount{Data: data, Slot: 1, WriteVersion: uint64(i + 1)}
					if !reference {
						if _, err := c.Update(key, a); err != nil {
							b.Fatal(err)
						}
						continue
					}
					c.mu.Lock()
					next := make(map[solana.PublicKey]CachedAccount, len(c.accounts))
					for k, v := range c.accounts {
						next[k] = v
					}
					next[key] = owned(a)
					c.accounts = next
					c.mu.Unlock()
				}
			})
		}
	}
}

func BenchmarkReadinessGuardScaling(b *testing.B) {
	for _, size := range []int{100, 10000} {
		for _, reference := range []bool{false, true} {
			name := "fast_guard"
			if reference {
				name = "status_reference"
			}
			b.Run(fmt.Sprintf("accounts_%d/%s", size, name), func(b *testing.B) {
				keys := make([]string, size)
				for i := range keys {
					keys[i] = fmt.Sprint(i)
				}
				r, _ := NewSubscriptionReadiness("confirmed/fork-a", keys)
				r.MarkValidated(keys, 0, "confirmed/fork-a")
				guard, _ := r.Guard()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if reference {
						s := r.Status()
						if s.State != "ready" || s.Generation != 0 {
							b.Fatal(s)
						}
					} else if err := guard(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
