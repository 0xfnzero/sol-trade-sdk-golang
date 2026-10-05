package subscription

import (
	"fmt"
	"github.com/gagliardetto/solana-go"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Measures pool-index construction and selection, not quoting or network time.
func BenchmarkCandidateSparseRoute(b *testing.B) {
	for _, size := range []int{8, 64} {
		for _, reference := range []bool{false, true} {
			name := "indexed"
			search := VisitCandidateRoutes
			if reference {
				name = "scan_reference"
				search = visitCandidateRoutesScanReference
			}
			b.Run(fmt.Sprintf("pools_%d/%s", size, name), func(b *testing.B) {
				pools := []PoolTradeHint{}
				for i := 0; i < 4; i++ {
					pools = append(pools, PoolTradeHint{candidateTestKey(byte(130 + i)), candidateTestKey(byte(i + 1)), candidateTestKey(byte(i + 2))})
				}
				for i := 4; i < size; i++ {
					pools = append(pools, PoolTradeHint{candidateTestKey(byte(130 + i)), candidateTestKey(byte(10 + 2*(i-4))), candidateTestKey(byte(11 + 2*(i-4)))})
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err := search(pools, candidateTestKey(1), candidateTestKey(5), 5, func(path []PoolTradeHint) (bool, error) {
						if len(path) != 4 {
							b.Fatal("wrong path")
						}
						return true, nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func candidateTestKey(n byte) solana.PublicKey {
	var k solana.PublicKey
	for i := range k {
		k[i] = n
	}
	return k
}

func TestCandidateTerminalFrontier(t *testing.T) {
	pools := []PoolTradeHint{}
	n := byte(100)
	for a := byte(1); a <= 11; a++ {
		for b := a + 1; b <= 11; b++ {
			pools = append(pools, PoolTradeHint{candidateTestKey(n), candidateTestKey(a), candidateTestKey(b)})
			n++
		}
	}
	if _, err := CandidateRoutes(pools, candidateTestKey(1), candidateTestKey(200), 4); err == nil || !strings.Contains(err.Error(), "no connected") {
		t.Fatal(err)
	}
	if _, err := CandidateRoutes(pools, candidateTestKey(1), candidateTestKey(200), 5); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatal(err)
	}
}

func TestAllSmallCandidateGraphs(t *testing.T) {
	pairs := [][2]byte{{1, 2}, {1, 3}, {1, 4}, {2, 3}, {2, 4}, {3, 4}}
	for mask := 0; mask < 64; mask++ {
		pools, edges := []PoolTradeHint{}, []PoolTradeHint{}
		for i, p := range pairs {
			if mask&(1<<i) != 0 {
				h := PoolTradeHint{candidateTestKey(byte(100 + i)), candidateTestKey(p[0]), candidateTestKey(p[1])}
				pools = append(pools, h)
				edges = append(edges, h, PoolTradeHint{h.Pool, h.OutputMint, h.InputMint})
			}
		}
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].Pool != edges[j].Pool {
				return edges[i].Pool.String() < edges[j].Pool.String()
			}
			return edges[i].OutputMint.String() < edges[j].OutputMint.String()
		})
		type path struct {
			hints []PoolTradeHint
			mint  solana.PublicKey
			seen  map[solana.PublicKey]bool
		}
		frontier := []path{{nil, candidateTestKey(1), map[solana.PublicKey]bool{candidateTestKey(1): true}}}
		expected := [][]PoolTradeHint{}
		for depth := 0; depth < 3; depth++ {
			next := []path{}
			for _, p := range frontier {
				for _, h := range edges {
					if h.InputMint != p.mint || p.seen[h.OutputMint] {
						continue
					}
					hints := append(append([]PoolTradeHint{}, p.hints...), h)
					if h.OutputMint == candidateTestKey(4) {
						expected = append(expected, hints)
					} else {
						seen := map[solana.PublicKey]bool{}
						for k, v := range p.seen {
							seen[k] = v
						}
						seen[h.OutputMint] = true
						next = append(next, path{hints, h.OutputMint, seen})
					}
				}
			}
			frontier = next
		}
		for i, j := 0, len(pools)-1; i < j; i, j = i+1, j-1 {
			pools[i], pools[j] = pools[j], pools[i]
		}
		actual, err := CandidateRoutes(pools, candidateTestKey(1), candidateTestKey(4), 3)
		if len(expected) == 0 {
			if err == nil {
				t.Fatal("expected disconnected graph", mask)
			}
		} else if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatal("route ordering changed", mask, err)
		}
	}
}

func TestLazyShortestCandidates(t *testing.T) {
	key := func(n byte) solana.PublicKey {
		var k solana.PublicKey
		for i := range k {
			k[i] = n
		}
		return k
	}
	pools := []PoolTradeHint{}
	n := byte(10)
	for a := byte(1); a <= 9; a++ {
		for b := a + 1; b <= 9; b++ {
			pools = append(pools, PoolTradeHint{key(n), key(a), key(b)})
			n++
		}
	}
	calls := 0
	err := VisitCandidateRoutes(pools, key(1), key(2), 5, func(h []PoolTradeHint) (bool, error) {
		calls++
		if len(h) != 1 {
			t.Fatal("not shortest")
		}
		return true, nil
	})
	if err != nil || calls != 1 {
		t.Fatal(calls, err)
	}
	if _, err = CandidateRoutes(pools, key(1), key(2), 5); err == nil {
		t.Fatal("unbounded collection")
	}
}
